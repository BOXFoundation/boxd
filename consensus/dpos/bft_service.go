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

// Define const.
const (
	EternalBlockMsgChBufferSize        = 65536
	MaxEternalBlockMsgCacheTime        = 5
	MinConfirmMsgNumberForEternalBlock = 2 * PeriodSize / 3
)

// BftService use for quick identification of eternal block.
type BftService struct {
	eternalBlockMsgCh       chan p2p.Message
	notifiee                p2p.Net
	chain                   *chain.BlockChain
	consensus               *Dpos
	msgCache                *sync.Map
	existEternalBlockMsgKey *lru.Cache
	proc                    goprocess.Process
}

// NewBftService new bft service for eternalBlockMsg.
func NewBftService(consensus *Dpos) (*BftService, error) {

	bft := &BftService{
		eternalBlockMsgCh: make(chan p2p.Message, EternalBlockMsgChBufferSize),
		notifiee:          consensus.net,
		chain:             consensus.chain,
		consensus:         consensus,
		msgCache:          new(sync.Map),
		proc:              goprocess.WithParent(consensus.proc),
	}

	bft.existEternalBlockMsgKey, _ = lru.New(64)
	return bft, nil
}

// Start bft service to handle eternalBlockMsg.
func (bft *BftService) Start() {
	bft.subscribeMessageNotifiee()
	bft.proc.Go(bft.loop)
	// bft.proc.Go(bft.checkEternalBlock)
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
func (bft *BftService) FetchIrreversibleInfo() *types.IrreversibleInfo {

	tailHeight := bft.chain.TailBlock().Height
	MinerRefreshIntervalInSecond := MinerRefreshInterval / SecondInMs
	offset := time.Now().Unix() % MinerRefreshIntervalInSecond

	if tailHeight == 0 {
		return nil
	}
	height := tailHeight
	for offset >= 0 && height > 0 {
		block, err := bft.chain.LoadBlockByHeight(height)
		if err != nil {
			continue
		}
		blockHash := *block.BlockHash()
		if value, ok := bft.msgCache.Load(blockHash); ok {
			signatures := value.([][]byte)
			if len(signatures) > MinConfirmMsgNumberForEternalBlock {
				go bft.updateEternal(block)
				irreversibleInfo := new(types.IrreversibleInfo)
				irreversibleInfo.Hash = blockHash
				irreversibleInfo.Signatures = value.([][]byte)
				bft.msgCache.Delete(blockHash)
				return irreversibleInfo
			}
		}
		height--
		offset--
	}
	if offset == MinerRefreshIntervalInSecond-1 {
		bft.msgCache = &sync.Map{}
	}

	return nil
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
	if bft.chain.TailBlock().Height-bft.chain.EternalBlock().Height > MinConfirmMsgNumberForEternalBlock*BlockNumPerPeiod {
		block, err := bft.chain.LoadBlockByHeight(bft.chain.EternalBlock().Height + 1)
		if err != nil {
			logger.Errorf("Failed to update eternal block. LoadBlockByHeight occurs error: %s", err.Error())
		} else {
			bft.updateEternal(block)
		}
	}
}

func (bft *BftService) updateEternal(block *types.Block) {

	if block.Height <= bft.chain.EternalBlock().Height {
		//logger.Warnf("No need to update eternal block because the height is lower than current eternal block height")
		return
	}
	if err := bft.chain.SetEternal(block); err != nil {
		logger.Info("Failed to update eternal block.Hash: %s, Height: %d, Err: %s",
			block.BlockHash().String(), block.Height, err.Error())
		return
	}
	logger.Infof("Eternal block has changed! Hash: %s Height: %d", block.BlockHash(), block.Height)

}

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

	key := eternalBlockMsg.Hash
	if bft.existEternalBlockMsgKey.Contains(key) {
		logger.Debugf("Enough eternalBlockMsgs has been received.")
		return nil
	}

	now := time.Now().Unix()
	if eternalBlockMsg.Timestamp > now || now-eternalBlockMsg.Timestamp > MaxEternalBlockMsgCacheTime {
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

	if pubkey, ok := crypto.RecoverCompact(eternalBlockMsg.Hash[:], eternalBlockMsg.Signature); ok {
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

		if msg, ok := bft.msgCache.Load(key); ok {
			value := msg.([][]byte)
			if util.InArray(eternalBlockMsg.Signature, value) {
				return nil
			}
			value = append(value, eternalBlockMsg.Signature)
			bft.msgCache.Store(key, value)
			if len(value) > MinConfirmMsgNumberForEternalBlock {
				bft.existEternalBlockMsgKey.Add(key, key)
			}
		} else {
			bft.msgCache.Store(key, [][]byte{eternalBlockMsg.Signature})
		}
	}

	return nil
}
