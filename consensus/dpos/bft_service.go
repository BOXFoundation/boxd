// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"bytes"
	"errors"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

type status int

// EternalBlockMsgKeyType is renamed EternalBlockMsgKey type
type EternalBlockMsgKeyType [crypto.HashSize + 8]byte

// Define const.
const (
	EternalBlockMsgChBufferSize        = 65536
	MaxEternalBlockMsgCacheTime        = 10 * 60
	free                        status = iota
	underway
)

// Define error msg.
var (
	ErrNoNeedToUpdateEternalBlock = errors.New("No need to update Eternal block")
	ErrIllegalMsg                 = errors.New("Illegal message from remote peer")
	ErrEternalBlockMsgHashIsExist = errors.New("EternalBlockMsgHash is already exist")
)

// BftService use for quick identification of eternal block.
type BftService struct {
	eternalBlockMsgCh       chan p2p.Message
	notifiee                p2p.Net
	chain                   *chain.BlockChain
	consensus               *Dpos
	cache                   map[EternalBlockMsgKeyType][]*EternalBlockMsg
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
		cache:             make(map[EternalBlockMsgKeyType][]*EternalBlockMsg),
		proc:              goprocess.WithParent(consensus.proc),
	}
	var err error
	bft.existEternalBlockMsgKey, err = lru.New(64)
	if err != nil {
		return nil, err
	}
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
			if err := bft.handleEternalBlock(msg); err != nil {
				logger.Warnf("Failed to handle eternalBlockMsg. Err: %s", err.Error())
			}
		case <-p.Closing():
			logger.Info("Quit bftservice loop.")
			return
		}
	}
}

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
	bft.tryToUpdateEternal()
}

func (bft *BftService) tryToUpdateEternal() {
	now := time.Now().Unix()
	for k, v := range bft.cache {
		if v[0].timestamp > now || now-v[0].timestamp > MaxEternalBlockMsgCacheTime {
			delete(bft.cache, k)
		}
		if len(v) < 2/3*PeriodSize {
			continue
		}
		if bft.updateEternal(v[0]) {
			delete(bft.cache, k)
		}
	}
}

func (bft *BftService) updateEternal(msg *EternalBlockMsg) bool {
	block, err := bft.chain.LoadBlockByHash(msg.hash)
	if err != nil {
		return false
	}

	if block.Height <= bft.chain.EternalBlock().Height {
		return true
	}
	if err := bft.chain.SetEternal(block); err != nil {
		return false
	}
	logger.Infof("Eternal block has changed! Hash: %s Height: %d", block.BlockHash(), block.Height)
	return true
}

func (bft *BftService) generateKey(hash crypto.HashType, timestamp int64) *EternalBlockMsgKeyType {
	buf := make([]byte, crypto.HashSize+8)
	copy(buf, hash[:])
	w := bytes.NewBuffer(buf[crypto.HashSize:])
	util.WriteInt64(w, timestamp)
	result := new(EternalBlockMsgKeyType)
	copy(result[:], buf)
	return result
}

func (bft *BftService) handleEternalBlock(msg p2p.Message) error {

	// quick check
	peerID := msg.From().Pretty()
	if !util.InArray(peerID, bft.consensus.context.periodContext.periodPeers) {
		return ErrIllegalMsg
	}

	eternalBlockMsg := new(EternalBlockMsg)
	if err := eternalBlockMsg.Unmarshal(msg.Body()); err != nil {
		return err
	}

	key := bft.generateKey(eternalBlockMsg.hash, eternalBlockMsg.timestamp)

	if bft.existEternalBlockMsgKey.Contains(*key) {
		return ErrEternalBlockMsgHashIsExist
	}
	now := time.Now().Unix()
	if eternalBlockMsg.timestamp > now || now-eternalBlockMsg.timestamp > MaxEternalBlockMsgCacheTime {
		return ErrIllegalMsg
	}

	pubkey, ok := crypto.RecoverCompact(eternalBlockMsg.hash[:], eternalBlockMsg.signature)
	if ok {
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
			return err
		}

		msg, ok := bft.cache[*key]
		if ok {
			msg := append(msg, eternalBlockMsg)
			bft.cache[*key] = msg
			if len(bft.cache[*key]) > 2/3*PeriodSize {
				bft.existEternalBlockMsgKey.Add(*key, *key)
			}
		}
		bft.cache[*key] = []*EternalBlockMsg{eternalBlockMsg}

		// update eternal block
		// block, err := bft.chain.LoadBlockByHash(eternalBlockMsg.hash)
		// if err != nil {
		// 	if err == core.ErrBlockIsNil {
		// 		// cache the ternalBlockMsg
		// 		msg, ok := bft.cache[*key]
		// 		if ok {
		// 			msg := append(msg, eternalBlockMsg)
		// 			bft.cache[*key] = msg
		// 			if len(bft.cache[*key]) > 2/3*PeriodSize {
		// 				bft.existEternalBlockMsgKey.Add(*key, *key)
		// 			}
		// 		}
		// 		bft.cache[*key] = []*EternalBlockMsg{eternalBlockMsg}
		// 		return nil
		// 	}
		// 	return err
		// }

		// if block.Height <= bft.chain.EternalBlock().Height {
		// 	return ErrNoNeedToUpdateEternalBlock
		// }
		// if err := bft.chain.SetEternal(block); err != nil {
		// 	return err
		// }
		// logger.Info("Eternal block has changed! Hash: %s Height: %d", block.BlockHash(), block.Height)
	}

	return nil
}
