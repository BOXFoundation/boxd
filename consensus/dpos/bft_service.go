// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"sync"
	"time"

	chain "github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

// bft check eternal status
type status int

// EternalBlockMsgKeyType is renamed EternalBlockMsgKey type
// type EternalBlockMsgKeyType [EternalBlockMsgKeySize]byte

// Define const.
const (
	EternalBlockMsgChBufferSize        = 65536
	MaxEternalBlockMsgCacheTime        = 5
	MinConfirmMsgNumberForEternalBlock = 2 * PeriodSize / 3
	// EternalBlockMsgKeySize             = crypto.HashSize + 8

	free status = iota
	underway
)

// BftService use for quick identification of eternal block.
type BftService struct {
	eternalBlockMsgCh       chan p2p.Message
	notifiee                p2p.Net
	chain                   *chain.BlockChain
	consensus               *Dpos
	cache                   *sync.Map
	existEternalBlockMsgKey *lru.Cache
	checkStatus             status
	proc                    goprocess.Process
}

// NewBftService new bft service for eternalBlockMsg.
func NewBftService(consensus *Dpos) (*BftService, error) {

	bft := &BftService{
		eternalBlockMsgCh: make(chan p2p.Message, EternalBlockMsgChBufferSize),
		notifiee:          consensus.net,
		chain:             consensus.chain,
		consensus:         consensus,
		checkStatus:       free,
		cache:             new(sync.Map),
		proc:              goprocess.WithParent(consensus.proc),
	}

	bft.existEternalBlockMsgKey, _ = lru.New(64)
	return bft, nil
}

// Start bft service to handle eternalBlockMsg.
func (bft *BftService) Start() {
	bft.subscribeMessageNotifiee()
	bft.proc.Go(bft.loop)
	bft.proc.Go(bft.checkEternalBlock)
}

func (bft *BftService) subscribeMessageNotifiee() {
	bft.notifiee.Subscribe(p2p.NewNotifiee(p2p.EternalBlockMsg, bft.eternalBlockMsgCh))
}

func (bft *BftService) loop(p goprocess.Process) {
	logger.Info("Start BftService to quick identification of eternal block...")
	for {
		select {
		case msg := <-bft.eternalBlockMsgCh:
			if err := bft.handleEternalBlockMsg(msg); err != nil {
				logger.Warnf("Failed to handle eternalBlockMsg. Err: %s", err.Error())
			}
		case <-p.Closing():
			logger.Info("Quit bftservice loop.")
			return
		}
	}
}

// FetchIrreversibleInfo fetch Irreversible block info.
func (bft *BftService) FetchIrreversibleInfo() (*types.IrreversibleInfo, error) {

	tail := bft.chain.TailBlock()
	MinerRefreshIntervalInSecond := MinerRefreshInterval / SecondInMs
	offset := uint32(time.Now().Unix() % MinerRefreshIntervalInSecond)
	defer func() {
		if offset == uint32(MinerRefreshIntervalInSecond-1) {
			bft.cache = &sync.Map{}
		}
	}()
	var targetHash *crypto.HashType
	height := tail.Height - 1
	for offset >= 0 && height > 0 {
		height--
		offset--
		block, err := bft.chain.LoadBlockByHeight(height)
		if err != nil {
			return nil, err
		}
		if len(block.Signature) >= MinConfirmMsgNumberForEternalBlock {
			go bft.updateEternal(block)
			targetHash = block.Hash
			break
		}
	}

	if value, ok := bft.cache.Load(targetHash); ok {
		irreversibleInfo := new(types.IrreversibleInfo)
		irreversibleInfo.Hash = targetHash
		irreversibleInfo.Signatures = value.([][]byte)
		bft.cache.Delete(targetHash)
		return irreversibleInfo, nil
	}
	return nil, nil
}

// checkEternalBlock check to update eternal block.
func (bft *BftService) checkEternalBlock(p goprocess.Process) {
	logger.Info("Start to check eternalBlock...")
	timerChan := time.NewTicker(time.Second)
	defer timerChan.Stop()
	for {
		select {
		case <-timerChan.C:
			bft.maybeUpdateEternalBlock()
		case <-p.Closing():
			logger.Info("Quit checkEternalBlock loop.")
			return
		}
	}
}

func (bft *BftService) maybeUpdateEternalBlock() {
	if bft.checkStatus == underway {
		return
	}
	defer func() {
		bft.checkStatus = free
	}()
	bft.checkStatus = underway
	// bft.tryToUpdateEternal()
	if bft.chain.TailBlock().Height-bft.chain.EternalBlock().Height > MinConfirmMsgNumberForEternalBlock*BlockNumPerPeiod {
		block, err := bft.chain.LoadBlockByHeight(bft.chain.EternalBlock().Height + 1)
		if err != nil {
			logger.Errorf("Failed to update eternal block. LoadBlockByHeight occurs error: %s", err.Error())
			return
		}
		if err := bft.chain.SetEternal(block); err != nil {
			logger.Errorf("Failed to setEternal block. Height: %d, Hash: %v, err: %s", block.Height, block.Hash, err.Error())
			return
		}
		logger.Infof("Eternal block has changed! Hash: %s Height: %d", block.BlockHash(), block.Height)
	}
}

// func (bft *BftService) tryToUpdateEternal() {

// 	now := time.Now().Unix()
// 	bft.cache.Range(func(k, v interface{}) bool {
// 		value := v.([]*EternalBlockMsg)
// 		if value[0].timestamp > now || now-value[0].timestamp > MaxEternalBlockMsgCacheTime {
// 			bft.cache.Delete(k)
// 		}
// 		if len(value) <= MinConfirmMsgNumberForEternalBlock {
// 			return true
// 		}
// 		if bft.updateEternal(value[0]) {
// 			bft.cache.Delete(k)
// 		}
// 		return true
// 	})
// }

func (bft *BftService) updateEternal(block *types.Block) {

	if block.Height <= bft.chain.EternalBlock().Height {
		logger.Warnf("No need to update eternal block because the height is lower than current eternal block height")
		return
	}
	//TODO: check irreversible signature information

	if err := bft.chain.SetEternal(block); err != nil {
		logger.Info("Failed to update eternal block.Hash: %s, Height: %d, Err: %s",
			block.BlockHash().String(), block.Height, err.Error())
		return
	}
	logger.Infof("Eternal block has changed! Hash: %s Height: %d", block.BlockHash(), block.Height)

}

// func (bft *BftService) generateKey(hash crypto.HashType, timestamp int64) *EternalBlockMsgKeyType {
// 	buf := make([]byte, EternalBlockMsgKeySize)
// 	copy(buf, hash[:])
// 	w := bytes.NewBuffer(buf[crypto.HashSize:])
// 	util.WriteInt64(w, timestamp)
// 	result := new(EternalBlockMsgKeyType)
// 	copy(result[:], buf)
// 	return result
// }

func (bft *BftService) handleEternalBlockMsg(msg p2p.Message) error {

	// quick check
	peerID := msg.From().Pretty()
	if !util.InArray(peerID, bft.consensus.context.periodContext.periodPeers) {
		return ErrNotMintPeer
	}

	eternalBlockMsg := new(EternalBlockMsg)
	if err := eternalBlockMsg.Unmarshal(msg.Body()); err != nil {
		return err
	}

	key := eternalBlockMsg.hash
	if bft.existEternalBlockMsgKey.Contains(key) {
		logger.Debugf("Enough eternalBlockMsgs has been received.")
		return nil
	}

	now := time.Now().Unix()
	if eternalBlockMsg.timestamp > now || now-eternalBlockMsg.timestamp > MaxEternalBlockMsgCacheTime {
		return ErrIllegalMsg
	}

	miner, err := bft.consensus.context.periodContext.FindMinerWithTimeStamp(now + 1)
	if err != nil {
		return err
	}
	addr, err := types.NewAddress(bft.consensus.miner.Addr())
	if err != nil {
		return err
	}
	// No need to deal with messages that were not in my production block time period
	if *miner != *addr.Hash160() {
		return nil
	}

	if pubkey, ok := crypto.RecoverCompact(eternalBlockMsg.hash[:], eternalBlockMsg.signature); ok {
		addrPubKeyHash, err := types.NewAddressFromPubKey(pubkey)
		if err != nil {
			return err
		}
		addr := *addrPubKeyHash.Hash160()
		var period *Period
		for _, v := range bft.consensus.context.periodContext.period {
			if v.addr == addr && peerID == v.peerID {
				period = v
			}
		}
		if period == nil {
			return ErrIllegalMsg
		}

		if msg, ok := bft.cache.Load(key); ok {
			value := msg.([][]byte)
			value = append(value, eternalBlockMsg.signature)
			bft.cache.Store(key, value)
			if len(value) > MinConfirmMsgNumberForEternalBlock {
				bft.existEternalBlockMsgKey.Add(key, key)
			}
		} else {
			bft.cache.Store(key, [][]byte{eternalBlockMsg.signature})
		}
	}

	return nil
}
