// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/BOXFoundation/Quicksilver/log"
	"github.com/BOXFoundation/Quicksilver/p2p"
	"github.com/jbenet/goprocess"
)

var logger log.Logger // logger

func init() {
	logger = log.NewLogger("core")
}

// BlockChain define chain struct
type BlockChain struct {
	notifiee      p2p.Net
	newblockMsgCh chan p2p.Message
	txpool        *TransactionPool
	proc          goprocess.Process
}

// NewBlockChain return a blockchain.
func NewBlockChain(parent goprocess.Process, notifiee p2p.Net) *BlockChain {

	return &BlockChain{
		notifiee:      notifiee,
		newblockMsgCh: make(chan p2p.Message, 1024),
		proc:          goprocess.WithParent(parent),
		txpool:        NewTransactionPool(parent, notifiee),
	}
}

// Run launch blockchain.
func (chain *BlockChain) Run() {

	chain.subscribeMessageNotifiee(chain.notifiee)
	go chain.loop()
	chain.txpool.Run()
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
