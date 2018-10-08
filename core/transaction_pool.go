// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"errors"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
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
	ErrOrphanTransaction      = errors.New("Orphan transaction cannot be admitted into the pool")
)

// TransactionPool define struct.
type TransactionPool struct {
	notifiee   p2p.Net
	newTxMsgCh chan p2p.Message
	proc       goprocess.Process
	chain      *BlockChain

	// transaction pool
	hashToTx map[crypto.HashType]*TxWrap
	txMutex  sync.Mutex
	// orphan transaction pool
	hashToOrphanTx map[crypto.HashType]*types.Transaction
	// Transaction output point to orphan transactions spending it.
	// One outpoint can have multiple spending txs, only one can be legit
	// Note: use map here, not slice, so we don't have to delete from slice
	outPointToOrphan map[types.OutPoint]map[crypto.HashType]*types.Transaction

	// spent tx outputs (STXO) by all txs in the pool
	stxoSet map[types.OutPoint]*types.Transaction
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
		newTxMsgCh:       make(chan p2p.Message, TxMsgBufferChSize),
		proc:             goprocess.WithParent(parent),
		notifiee:         notifiee,
		chain:            chain,
		hashToTx:         make(map[crypto.HashType]*TxWrap),
		hashToOrphanTx:   make(map[crypto.HashType]*types.Transaction),
		outPointToOrphan: make(map[types.OutPoint]map[crypto.HashType]*types.Transaction),
		stxoSet:          make(map[types.OutPoint]*types.Transaction),
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
	tx := new(types.Transaction)
	if err := tx.Unmarshal(msg.Body()); err != nil {
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

	// Accept any orphan transactions that depend on this transaction
	// and repeat for those accepted transactions until there are no more.
	return tx_pool.processOrphans(tx)
}

// Potentially accept the transaction to the memory pool.
func (tx_pool *TransactionPool) maybeAcceptTx(tx *types.Transaction, broadcast bool) error {
	tx_pool.txMutex.Lock()
	defer tx_pool.txMutex.Unlock()
	txHash, _ := tx.TxHash()

	// Don't accept the transaction if it already exists in the pool.
	// This applies to orphan transactions as well
	if tx_pool.isTransactionInPool(txHash) || tx_pool.isOrphanInPool(txHash) {
		logger.Debugf("Tx %v already exists", txHash)
		return ErrDuplicateTxInPool
	}

	// Perform preliminary sanity checks on the transaction.
	if err := SanityCheckTransaction(tx); err != nil {
		logger.Debugf("Tx %v fails sanity check: %v", txHash, err)
		return err
	}

	// A standalone transaction must not be a coinbase transaction.
	if IsCoinBase(tx) {
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
	if err := tx_pool.checkPoolDoubleSpend(tx); err != nil {
		logger.Debugf("Tx %v double spends outputs spent by other pending txs: %v", txHash, err)
		return err
	}

	// TODO: check tx is already exist in the main chain??

	// Add orphan transaction
	if tx_pool.isOrphan(tx) {
		tx_pool.addOrphan(tx)
		return ErrOrphanTransaction
	}

	// TODO: sequence lock

	txFee, err := tx_pool.chain.checkTransactionInputs(tx, nextBlockHeight)
	if err != nil {
		return err
	}

	// TODO: checkInputsStandard

	// TODO: GetSigOpCost check

	// TODO: Whether the minfee limit is needed？
	// how to calc the minfee, or use a fixed value.
	txSize, err := tx.SerializeSize()
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
	if err = tx_pool.chain.ValidateTransactionScripts(tx); err != nil {
		return err
	}

	feePerKB := txFee * 1000 / (int64)(txSize)
	// add transaction to pool.
	tx_pool.addTx(tx, nextBlockHeight, feePerKB)

	logger.Debugf("Accepted transaction %v (pool size: %v)", txHash, len(tx_pool.hashToTx))
	// Broadcast this tx.
	if broadcast {
		tx_pool.notifiee.Broadcast(p2p.TransactionMsg, tx)
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

// A tx is an orphan if any of its spending utxo does not exist
func (tx_pool *TransactionPool) isOrphan(tx *types.Transaction) bool {
	for _, txIn := range tx.Vin {
		// Spend main chain UTXOs
		utxo := tx_pool.chain.utxoSet.FindUtxo(txIn.PrevOutPoint)
		if utxo != nil && !utxo.IsSpent {
			continue
		}

		// Spend mempool tx outputs
		// Note: we do not check double spends within mempool,
		// which will be checked when an orphan is allowed into mempool
		if _, exists := tx_pool.hashToTx[txIn.PrevOutPoint.Hash]; !exists {
			return true
		}
	}

	return false
}

func (tx_pool *TransactionPool) checkTransactionStandard(tx *types.Transaction) error {
	// TODO:
	return nil
}

func (tx_pool *TransactionPool) checkPoolDoubleSpend(tx *types.Transaction) error {
	for _, txIn := range tx.Vin {
		if _, exists := tx_pool.stxoSet[txIn.PrevOutPoint]; exists {
			return ErrOutPutAlreadySpent
		}
	}
	return nil
}

// ProcessOrphans determines if there are any orphans which depend on the passed
// transaction and potentially accepts them to the memory pool.
// It repeats the process for the newly accepted transactions until there are no more.
func (tx_pool *TransactionPool) processOrphans(tx *types.Transaction) error {
	// Start with processing at least the passed tx.
	acceptedTxs := []*types.Transaction{tx}

	// Note: use index here instead of range because acceptedTxs can be extended inside the loop
	for i := 0; i < len(acceptedTxs); i++ {
		acceptedTx := acceptedTxs[i]
		acceptedTxHash, _ := acceptedTx.TxHash()

		// Look up all txs that spend output from the tx we just accepted.
		outPoint := types.OutPoint{Hash: *acceptedTxHash}
		for txOutIdx := range acceptedTx.Vout {
			outPoint.Index = uint32(txOutIdx)

			orphans, exists := tx_pool.outPointToOrphan[outPoint]
			if !exists {
				continue
			}

			for _, orphan := range orphans {
				if err := tx_pool.maybeAcceptTx(orphan, false); err != nil {
					continue
				}
				tx_pool.removeOrphan(orphan)
				// Add this tx to the list of txs to process so any orphan
				// txs that depend on this tx are handled too.
				acceptedTxs = append(acceptedTxs, orphan)

				// Only one transaction for this outpoint can be accepted,
				// so the rest are now double spends and are removed later.
				break
			}
		}
	}

	// Recursively remove any orphans that also redeem any outputs redeemed
	// by the accepted transactions since those are now definitive double spends.
	for _, acceptedTx := range acceptedTxs {
		tx_pool.removeOrphanDoubleSpends(acceptedTx)
	}

	return nil
}

// Add transaction into tx pool
func (tx_pool *TransactionPool) addTx(tx *types.Transaction, height int32, feePerKB int64) {
	txHash, _ := tx.TxHash()

	txWrap := &TxWrap{
		tx:             tx,
		addedTimestamp: time.Now().Unix(),
		height:         height,
		feePerKB:       feePerKB,
	}
	tx_pool.hashToTx[*txHash] = txWrap

	// outputs spent by this new tx
	for _, txIn := range tx.Vin {
		tx_pool.stxoSet[txIn.PrevOutPoint] = tx
	}
}

// Remove transaction from tx pool
func (tx_pool *TransactionPool) removeTx(tx *types.Transaction) {
	txHash, _ := tx.TxHash()

	// Unspend the referenced outpoints.
	for _, txIn := range tx.Vin {
		delete(tx_pool.stxoSet, txIn.PrevOutPoint)
	}
	delete(tx_pool.hashToTx, *txHash)
}

// Add orphan
func (tx_pool *TransactionPool) addOrphan(tx *types.Transaction) {
	// TODO: limit orphan pool size

	txHash, _ := tx.TxHash()
	tx_pool.hashToOrphanTx[*txHash] = tx

	for _, txIn := range tx.Vin {
		// Indexing
		if _, exists := tx_pool.outPointToOrphan[txIn.PrevOutPoint]; !exists {
			tx_pool.outPointToOrphan[txIn.PrevOutPoint] =
				make(map[crypto.HashType]*types.Transaction)
		}
		tx_pool.outPointToOrphan[txIn.PrevOutPoint][*txHash] = tx
	}

	logger.Debugf("Stored orphan transaction %v", txHash)
}

// Remove orphan
func (tx_pool *TransactionPool) removeOrphan(tx *types.Transaction) {
	txHash, _ := tx.TxHash()
	// Orphans whose output this orphan spends
	for _, txIn := range tx.Vin {
		siblingOrphans, exists := tx_pool.outPointToOrphan[txIn.PrevOutPoint]
		if exists {
			delete(siblingOrphans, *txHash)

			// Remove the map entry altogether if there are no
			// longer any orphans which depend on it.
			if len(siblingOrphans) == 0 {
				delete(tx_pool.outPointToOrphan, txIn.PrevOutPoint)
			}
		}
	}

	// TODO: redundant bcoz orphan's outputs are not connected to spending orphans in addOrphan
	// Orphans who spends this orphan's output
	// outPoint := types.OutPoint{Hash: *txHash}
	// for txOutIdx := range tx.Vout {
	// 	outPoint.Index = uint32(txOutIdx)
	// 	for _, orphan := range tx_pool.outPointToOrphan[outPoint] {
	// 		tx_pool.removeOrphan(orphan)
	// 	}
	// }

	// Remove the transaction from the orphan pool.
	delete(tx_pool.hashToOrphanTx, *txHash)
}

// Remove all transactions which spend outputs spent by the
// passed transaction from the memory pool. This is necessary when a new main chain
// tip block may contain transactions which were previously unknown to the memory pool.
func (tx_pool *TransactionPool) removeDoubleSpends(tx *types.Transaction) {
	for _, txIn := range tx.Vin {
		if doubleSpentTx, exists := tx_pool.stxoSet[txIn.PrevOutPoint]; exists {
			tx_pool.removeTx(doubleSpentTx)
		}
	}
}

// removeOrphanDoubleSpends removes all orphans which spend outputs spent by the
// passed transaction from the orphan pool.  Removing those orphans then leads
// to removing all orphans which rely on them, recursively.  This is necessary
// when a transaction is added to the main pool because it may spend outputs
// that orphans also spend.
func (tx_pool *TransactionPool) removeOrphanDoubleSpends(tx *types.Transaction) {
	for _, txIn := range tx.Vin {
		for _, orphan := range tx_pool.outPointToOrphan[txIn.PrevOutPoint] {
			tx_pool.removeOrphan(orphan)
		}
	}
}

// Returns all transactions in mempool
func (tx_pool *TransactionPool) getAllTxs() []*TxWrap {
	txs := make([]*TxWrap, len(tx_pool.hashToTx))
	i := 0
	for _, tx := range tx_pool.hashToTx {
		txs[i] = tx
		i++
	}
	return txs
}

func calcRequiredMinFee(txSize int) int64 {
	return 0
}
