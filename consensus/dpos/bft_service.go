// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"errors"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
	"github.com/jbenet/goprocess"
)

// Define const.
const (
	EternalBlockMsgChBufferSize = 65536
)

// Define error msg.
var (
	ErrNoNeedToUpdateEternalBlock = errors.New("No need to update Eternal block")
	ErrIllegalMsg                 = errors.New("Illegal message from remote peer")
)

// BftService use for quick identification of eternal block.
type BftService struct {
	eternalBlockMsgCh chan p2p.Message
	notifiee          p2p.Net
	chain             *chain.BlockChain
	consensus         *Dpos
	proc              goprocess.Process
}

// NewBftService new bft service for eternalBlockMsg.
func NewBftService(consensus *Dpos) *BftService {
	return &BftService{
		eternalBlockMsgCh: make(chan p2p.Message, EternalBlockMsgChBufferSize),
		notifiee:          consensus.net,
		chain:             consensus.chain,
		consensus:         consensus,
		proc:              goprocess.WithParent(consensus.proc),
	}
}

// Start bft service to handle eternalBlockMsg.
func (bft *BftService) Start() {
	bft.subscribeMessageNotifiee()
	bft.proc.Go(bft.loop)
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
		// update eternal block

		block, err := bft.chain.LoadBlockByHash(eternalBlockMsg.hash)
		if err != nil {
			return err
		}
		if block.Height <= bft.chain.EternalBlock().Height {
			return ErrNoNeedToUpdateEternalBlock
		}
		if err := bft.chain.SetEternal(block); err != nil {
			return err
		}
		logger.Info("Eternal block has changed! Hash: %s Height: %d", block.BlockHash(), block.Height)
	}

	return nil
}
