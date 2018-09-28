// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"container/heap"
	"errors"
	"sync/atomic"
	"time"

	corepb "github.com/BOXFoundation/Quicksilver/core/pb"
	"github.com/BOXFoundation/Quicksilver/core/types"
	"github.com/BOXFoundation/Quicksilver/crypto"
	"github.com/BOXFoundation/Quicksilver/p2p"
	"github.com/BOXFoundation/Quicksilver/util"
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
	pool       *util.PriorityQueue
	// transaction pool
	hashToTx map[crypto.HashType]*TxWrap

	// orphan transaction pool
	hashToOrphanTx map[crypto.HashType]*TxWrap
	// orphan transaction's parent; one parent can have multiple orphan children
	parentToOrphanTx map[crypto.HashType]*TxWrap

	outpoints map[types.OutPoint]*TxWrap

	// UTXO set
	outpointToOutput map[types.OutPoint]types.TxOut
}

func lessFunc(queue *util.PriorityQueue, i, j int) bool {

	txi := queue.Items(i).(*TxWrap)
	txj := queue.Items(j).(*TxWrap)
	if txi.feePerKB == txj.feePerKB {
		return txi.added < txj.added
	}
	return txi.feePerKB < txj.feePerKB
}

// TxWrap wrap transaction
type TxWrap struct {
	tx       *types.Transaction
	added    int64
	height   int32
	feePerKB int64
}

// NewTransactionPool new a transaction pool.
func NewTransactionPool(parent goprocess.Process, notifiee p2p.Net, chain *BlockChain) *TransactionPool {
	return &TransactionPool{
		newTxMsgCh: make(chan p2p.Message, TxMsgBufferChSize),
		proc:       goprocess.WithParent(parent),
		notifiee:   notifiee,
		chain:      chain,
		pool:       util.NewPriorityQueue(lessFunc),
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
	logger.Info("Waitting for new tx message...")
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
	tx, err := types.NewTx(msgTx)
	if err != nil {
		return err
	}
	return tx_pool.processTx(tx, false)
}

func (tx_pool *TransactionPool) processTx(tx *types.Transaction, broadcast bool) error {

	if tx_pool.isTransactionInPool(tx.Hash) {
		return ErrDuplicateTxInPool
	}

	if err := SanityCheckTransaction(tx.MsgTx); err != nil {
		return err
	}

	// A standalone transaction must not be a coinbase transaction.
	if IsCoinBase(tx.MsgTx) {
		return ErrCoinbaseTx
	}

	// ensure it is a "standard" transaction
	// TODO: is needed?
	if err := tx_pool.checkTransactionStandard(tx); err != nil {
		return ErrNonStandardTransaction
	}

	if err := tx_pool.checkDoubleSpend(tx.MsgTx); err != nil {
		return err
	}

	unspentUtxoCache, err := tx_pool.chain.LoadUnspentUtxo(tx)
	if err != nil {
		return err
	}

	// check msgTx is already exist in the main chain
	if err := tx_pool.checkExistInChain(tx, unspentUtxoCache); err != nil {
		return err
	}

	// handle orphan transactions
	if err := tx_pool.handleOrphan(); err != nil {
		return err
	}

	txFee, err := tx_pool.chain.CheckTransactionInputs(tx, unspentUtxoCache)
	if err != nil {
		return err
	}

	// TODO: Whether the minfee limit is needed？
	// how to calc the minfee, or use a fixed value.
	txSize, err := tx.MsgTx.SerializeSize()
	if err != nil {
		return err
	}
	minFee := calcRequiredMinFee(txSize)
	if txFee < minFee {
		return errors.New("txFee is less than minFee")
	}

	// verify crypto signatures for each input
	if err = tx_pool.chain.ValidateTransactionScripts(tx.MsgTx, unspentUtxoCache); err != nil {
		return err
	}

	// add transaction to pool.
	tx_pool.push(unspentUtxoCache, tx, txFee, int64(txSize))

	// Accept any orphan transactions that depend on this tx.
	if broadcast {
		tx_pool.notifiee.Broadcast(p2p.TransactionMsg, tx.MsgTx)
	}
	return nil
}

func (tx_pool *TransactionPool) isTransactionInPool(hash *crypto.HashType) bool {

	if _, exists := tx_pool.hashToTx[*hash]; exists {
		return true
	}
	return false
}

func (tx_pool *TransactionPool) checkTransactionStandard(tx *types.Transaction) error {
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

func (tx_pool *TransactionPool) checkExistInChain(tx *types.Transaction, unspentUtxoCache *UtxoUnspentCache) error {

	msgTx := tx.MsgTx
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

	prevOut := types.OutPoint{Hash: *tx.Hash}
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

func (tx_pool *TransactionPool) handleOrphan() error {
	return nil
}

func (tx_pool *TransactionPool) push(unspentUtxoCache *UtxoUnspentCache, tx *types.Transaction, txFee int64, txSize int64) *TxWrap {
	txwrap := &TxWrap{
		tx:       tx,
		height:   tx_pool.chain.tail.Height,
		feePerKB: int64(txFee / txSize),
	}
	atomic.StoreInt64(&txwrap.added, time.Now().Unix())
	heap.Push(tx_pool.pool, txwrap)
	tx_pool.hashToTx[*tx.Hash] = txwrap
	for _, txIn := range tx.MsgTx.Vin {
		tx_pool.outpoints[txIn.PrevOutPoint] = txwrap
	}
	return txwrap
}

func calcRequiredMinFee(txSize int) int64 {
	return 0
}
