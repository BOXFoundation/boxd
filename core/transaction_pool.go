// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"errors"
	"time"

	corepb "github.com/BOXFoundation/Quicksilver/core/pb"
	"github.com/BOXFoundation/Quicksilver/core/types"
	"github.com/BOXFoundation/Quicksilver/crypto"
	"github.com/BOXFoundation/Quicksilver/p2p"
	proto "github.com/gogo/protobuf/proto"
	"github.com/jbenet/goprocess"
)

// const defines constants
const (
	TxMsgBufferChSize = 65536
)

// define error message
var (
	ErrDuplicateTxInPool      = errors.New("Duplicate transactions in tx pool")
	ErrCoinbaseTx             = errors.New("Transaction must not be a coinbase transaction")
	ErrNonStandardTransaction = errors.New("Transaction is not a standard transaction")
	ErrOutPutAlreadySpent     = errors.New("Output already spent by transaction in the pool")
)

// TransactionPool define struct.
type TransactionPool struct {
	notifiee   p2p.Net
	newTxMsgCh chan p2p.Message
	proc       goprocess.Process
	chain      *BlockChain

	// transaction pool
	hashToTx map[crypto.HashType]*TxWrap

	// orphan transaction pool
	hashToOrphanTx map[crypto.HashType]*TxWrap
	// orphan transaction's parent; one parent can have multiple orphan children
	parentToOrphanTx map[crypto.HashType]*TxWrap

	// UTXO set
	outpointToOutput map[types.OutPoint]types.TxOut
}

// TxWrap wrap transaction
type TxWrap struct {
	tx     *types.Transaction
	added  time.Time
	height int
}

// NewTransactionPool new a transaction pool.
func NewTransactionPool(parent goprocess.Process, notifiee p2p.Net, chain *BlockChain) *TransactionPool {
	return &TransactionPool{
		newTxMsgCh: make(chan p2p.Message, TxMsgBufferChSize),
		proc:       goprocess.WithParent(parent),
		notifiee:   notifiee,
		chain:      chain,
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
			tx_pool.processTxMsg(msg)
		case <-tx_pool.proc.Closing():
			logger.Info("Quit transaction pool loop.")
			return
		}
	}
}

func (tx_pool *TransactionPool) processTxMsg(msg p2p.Message) error {
	body := msg.Body()
	pbtx := new(corepb.MsgTx)
	if err := proto.Unmarshal(body, pbtx); err != nil {
		return err
	}
	msgTx := new(types.MsgTx)
	if err := msgTx.Deserialize(pbtx); err != nil {
		return err
	}
	return tx_pool.processTx(msgTx)
}

func (tx_pool *TransactionPool) processTx(msgTx *types.MsgTx) error {

	hash, err := msgTx.MsgTxHash()
	if err != nil {
		return err
	}

	if tx_pool.isTransactionInPool(hash) {
		return ErrDuplicateTxInPool
	}

	if err = SanityCheckTransaction(msgTx); err != nil {
		return err
	}

	// A standalone transaction must not be a coinbase transaction.
	if IsCoinBase(msgTx) {
		return ErrCoinbaseTx
	}

	// ensure it is a "standard" transaction
	if err = tx_pool.checkTransactionStandard(msgTx); err != nil {
		return ErrNonStandardTransaction
	}

	if err = tx_pool.checkDoubleSpend(msgTx); err != nil {
		return err
	}

	// check msgTx is already exist in the main chain
	if err = tx_pool.checkExistInChain(msgTx, hash); err != nil {
		return err
	}

	// handle orphan transactions
	tx_pool.handleOrphan()

	return nil
}

func (tx_pool *TransactionPool) isTransactionInPool(hash *crypto.HashType) bool {
	if _, exists := tx_pool.hashToTx[*hash]; exists {
		return true
	}

	return false
}

func (tx_pool *TransactionPool) checkTransactionStandard(msgTx *types.MsgTx) error {
	// TODO:
	return nil
}

func (tx_pool *TransactionPool) checkDoubleSpend(msgTx *types.MsgTx) error {
	for _, txIn := range msgTx.Vin {
		if _, exists := tx_pool.outpointToOutput[txIn.PrevOutPoint]; exists {
			return ErrOutPutAlreadySpent
		}
	}
	return nil
}

func (tx_pool *TransactionPool) checkExistInChain(msgTx *types.MsgTx, hash *crypto.HashType) error {

	unspentUtxoCache, err := tx_pool.chain.LoadUnspentUtxo(msgTx)

	if err != nil {
		return err
	}

	for _, txIn := range msgTx.Vin {
		prevOut := &txIn.PrevOutPoint
		utxoWrap := unspentUtxoCache.FindByOutPoint(*prevOut)
		if utxoWrap != nil && !utxoWrap.IsPacked {
			continue
		}

		if _, exists := tx_pool.hashToTx[prevOut.Hash]; exists {
			// AddTxOut ignores out of range index values, so it is
			// safe to call without bounds checking here.
			// utxoView.AddTxOut(poolTxDesc.Tx, prevOut.Index,
			// 	mining.UnminedHeight)
		}
	}

	prevOut := types.OutPoint{Hash: *hash}
	for txOutIdx := range msgTx.Vout {
		prevOut.Index = uint32(txOutIdx)
		utxoWrap := unspentUtxoCache.FindByOutPoint(prevOut)
		// There is no outpoint in theory.
		if utxoWrap != nil && !utxoWrap.IsPacked {
			return errors.New("Transaction already exists")
		}
		unspentUtxoCache.RemoveByOutPoint(prevOut)
	}
	return nil
}

func (tx_pool *TransactionPool) handleOrphan() {

}
