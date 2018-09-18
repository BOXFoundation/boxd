// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/BOXFoundation/Quicksilver/p2p"
	"github.com/grafana/grafana/pkg/cmd/grafana-cli/logger"
	"github.com/jbenet/goprocess"
)

// BlockChain define chain struct
type BlockChain struct {
	newblockMsgCh chan p2p.Message
	proc          goprocess.Process
}

// NewBlockChain return a blockchain.
func NewBlockChain(parent goprocess.Process) *BlockChain {
	return &BlockChain{
		newblockMsgCh: make(chan p2p.Message, 1024),
		proc:          goprocess.WithParent(parent),
	}
}

// Run launch blockchain.
func (chain *BlockChain) Run() {
	chain.loop()
}

func (chain *BlockChain) subscribeMessageNotifiee(notifiee p2p.Net) {
	notifiee.Subscribe(p2p.NewNotifiee(p2p.NewBlockMsg, chain.newblockMsgCh))
}

func (chain *BlockChain) loop() {
	for {
		select {
		case msg := <-chain.newblockMsgCh:
			chain.processBlock(msg)
		case <-chain.proc.Closing():
			logger.Info("Quit blockchain loop.")
			return
		}
	}
}

func (chain *BlockChain) processBlock(msg p2p.Message) {

}
