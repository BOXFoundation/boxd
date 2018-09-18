// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/BOXFoundation/Quicksilver/p2p"
	"github.com/jbenet/goprocess"
)

// const define const
const (
	PoolSize = 65536
)

// TransactionPool define struct.
type TransactionPool struct {
	notifiee   p2p.Net
	newTxMsgCh chan p2p.Message
	proc       goprocess.Process
}

// NewTransactionPool new a transaction pool.
func NewTransactionPool(parent goprocess.Process, notifiee p2p.Net) *TransactionPool {
	return &TransactionPool{
		newTxMsgCh: make(chan p2p.Message, PoolSize),
		proc:       goprocess.WithParent(parent),
		notifiee:   notifiee,
	}
}

func (tx_pool *TransactionPool) subscribeMessageNotifiee(notifiee p2p.Net) {
	notifiee.Subscribe(p2p.NewNotifiee(p2p.TransactionMsg, tx_pool.newTxMsgCh))
}

// Run launch transaction pool.
func (tx_pool *TransactionPool) Run() {
	tx_pool.subscribeMessageNotifiee(tx_pool.notifiee)
	go tx_pool.loop()
}

// handle new tx message from network.
func (tx_pool *TransactionPool) loop() {
	for {
		select {
		case msg := <-tx_pool.newTxMsgCh:
			tx_pool.processTx(msg)
		case <-tx_pool.proc.Closing():
			logger.Info("Quit transaction pool loop.")
			return
		}
	}
}

func (tx_pool *TransactionPool) processTx(msg p2p.Message) {

}
