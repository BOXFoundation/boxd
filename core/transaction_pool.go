// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"container/heap"
	"errors"
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
	notifiee      p2p.Net
	newTxMsgCh    chan p2p.Message
	proc          goprocess.Process
	chain         *BlockChain
	priorityQueue *util.PriorityQueue

	// transaction pool
	hashToTx map[crypto.HashType]*TxWrap

	// orphan transaction pool
	hashToOrphanTx map[crypto.HashType]*TxWrap
	// orphan transaction's parent; one parent can have multiple orphan children
	orphanTxHashToChildren map[crypto.HashType][]*TxWrap

	// spent tx outputs (STXO) by all txs in the pool
	stxoSet map[types.OutPoint]*types.Transaction
}

func lessFunc(queue *util.PriorityQueue, i, j int) bool {

	txi := queue.Items(i).(*TxWrap)
	txj := queue.Items(j).(*TxWrap)
	if txi.feePerKB == txj.feePerKB {
		return txi.addedTimestamp < txj.addedTimestamp
	}
	return txi.feePerKB < txj.feePerKB
}

// TxWrap wrap transaction
type TxWrap struct {
	tx             *types.Transaction
	addedTimestamp int64
	height         int32
	feePerKB       int64
}

// NewTransactionPool new a transaction pool.
func NewTransactionPool(parent goprocess.Process, notifiee p2p.Net, chain *BlockChain) *TransactionPool {
	return &TransactionPool{
		newTxMsgCh:             make(chan p2p.Message, TxMsgBufferChSize),
		proc:                   goprocess.WithParent(parent),
		notifiee:               notifiee,
		chain:                  chain,
		priorityQueue:          util.NewPriorityQueue(lessFunc),
		hashToTx:               make(map[crypto.HashType]*TxWrap),
		hashToOrphanTx:         make(map[crypto.HashType]*TxWrap),
		orphanTxHashToChildren: make(map[crypto.HashType][]*TxWrap),
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

// processTx is the main workhorse for handling insertion of new
// transactions into the memory pool.  It includes functionality
// such as rejecting duplicate transactions, ensuring transactions follow all
// rules, orphan transaction handling, and insertion into the memory pool.
func (tx_pool *TransactionPool) processTx(tx *types.Transaction, broadcast bool) error {
	if err := tx_pool.maybeAcceptTx(tx, broadcast); err != nil {
		return err
	}

	// TODO: process orphan
	return nil
}

// Potentially accept the transaction to the memory pool.
func (tx_pool *TransactionPool) maybeAcceptTx(tx *types.Transaction, broadcast bool) error {
	txHash := tx.Hash

	// Don't accept the transaction if it already exists in the pool.
	// This applies to orphan transactions as well
	if tx_pool.isTransactionInPool(txHash) || tx_pool.isOrphanInPool(txHash) {
		logger.Debugf("Tx %v already exists", txHash)
		return ErrDuplicateTxInPool
	}

	// Perform preliminary sanity checks on the transaction.
	if err := SanityCheckTransaction(tx.MsgTx); err != nil {
		logger.Debugf("Tx %v fails sanity check: %v", txHash, err)
		return err
	}

	// A standalone transaction must not be a coinbase transaction.
	if IsCoinBase(tx.MsgTx) {
		logger.Debugf("Tx %v is an individual coinbase", txHash)
		return ErrCoinbaseTx
	}

	// Get the current height of the main chain. A standalone transaction
	// will be mined into the next block at best, so its height is at least
	// one more than the current height.
	nextBlockHeight := tx_pool.chain.longestChainHeight + 1

	// ensure it is a standard transaction
	if err := tx_pool.checkTransactionStandard(tx); err != nil {
		logger.Debugf("Tx %v is not standard: %v", txHash, err)
		return ErrNonStandardTransaction
	}

	// The transaction must not use any of the same outputs as other transactions already in the pool.
	// This check only detects double spends within the transaction pool itself.
	// Double spending coins from the main chain will be checked in checkTransactionInputs.
	if err := tx_pool.checkPoolDoubleSpend(tx.MsgTx); err != nil {
		logger.Debugf("Tx %v double spends outputs spent by other pending txs: %v", txHash, err)
		return err
	}

	// TODO: check msgTx is already exist in the main chain??

	// TODO: sequence lock

	txFee, err := tx_pool.chain.checkTransactionInputs(tx.MsgTx, nextBlockHeight)
	if err != nil {
		return err
	}

	// TODO: checkInputsStandard

	// TODO: GetSigOpCost check

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

	// TODO: priority check

	// TODO: free-to-relay rate limit

	// verify crypto signatures for each input
	if err = tx_pool.chain.ValidateTransactionScripts(tx.MsgTx); err != nil {
		return err
	}

	feePerKB := txFee * 1000 / (int64)(txSize)
	// add transaction to pool.
	tx_pool.addTx(tx, nextBlockHeight, feePerKB)

	logger.Debugf("Accepted transaction %v (pool size: %v)", txHash, len(tx_pool.hashToTx))
	// Broadcast this tx.
	if broadcast {
		tx_pool.notifiee.Broadcast(p2p.TransactionMsg, tx.MsgTx)
	}
	return nil
}

func (tx_pool *TransactionPool) isTransactionInPool(txHash *crypto.HashType) bool {
	_, exists := tx_pool.hashToTx[*txHash]
	return exists
}

func (tx_pool *TransactionPool) isOrphanInPool(txHash *crypto.HashType) bool {
	_, exists := tx_pool.hashToOrphanTx[*txHash]
	return exists
}

func (tx_pool *TransactionPool) checkTransactionStandard(tx *types.Transaction) error {
	// TODO:
	return nil
}

func (tx_pool *TransactionPool) checkPoolDoubleSpend(msgTx *types.MsgTx) error {
	for _, txIn := range msgTx.Vin {
		if _, exists := tx_pool.stxoSet[txIn.PrevOutPoint]; exists {
			return ErrOutPutAlreadySpent
		}
	}
	return nil
}

func (tx_pool *TransactionPool) handleOrphan() error {
	return nil
}

// Add transaction into tx pool
func (tx_pool *TransactionPool) addTx(tx *types.Transaction, height int32, feePerKB int64) {
	txWrap := &TxWrap{
		tx:             tx,
		addedTimestamp: time.Now().Unix(),
		height:         height,
		feePerKB:       feePerKB,
	}
	tx_pool.hashToTx[*tx.Hash] = txWrap
	// place onto heap sorted by feePerKB
	heap.Push(tx_pool.priorityQueue, txWrap)

	// outputs spent by this new tx
	for _, txIn := range tx.MsgTx.Vin {
		tx_pool.stxoSet[txIn.PrevOutPoint] = tx
	}
}

func calcRequiredMinFee(txSize int) int64 {
	return 0
}
