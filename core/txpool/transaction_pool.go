// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txpool

import (
	"errors"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/metrics"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
	"github.com/jbenet/goprocess"
)

// const defines constants
const (
	TxMsgBufferChSize          = 65536
	ChainUpdateMsgBufferChSize = 65536

	metricsLoopInterval = 2 * time.Second
)

var logger = log.NewLogger("txpool") // logger

var _ service.TxHandler = (*TransactionPool)(nil)

// TransactionPool define struct.
type TransactionPool struct {
	notifiee            p2p.Net
	newTxMsgCh          chan p2p.Message
	newChainUpdateMsgCh chan *chain.UpdateMsg
	txNotifee           *p2p.Notifiee
	proc                goprocess.Process
	chain               *chain.BlockChain
	hashToTx            map[crypto.HashType]*TxWrap
	bus                 eventbus.Bus
	// outpoint -> tx spending it; outpoints can be arbitrary, valid (exists and unspent) or invalid
	outPointToTx   map[types.OutPoint]*types.Transaction
	txMutex        sync.Mutex
	hashToOrphanTx map[crypto.HashType]*types.Transaction
	// outpoint -> orphans spending it; outpoints can be arbitrary, valid or invalid
	// Use map here since there can be multiple spending txs and we don't know which
	// one will be accepted, unlike in outPointToTx where first seen tx is accepted
	outPointToOrphan map[types.OutPoint]map[crypto.HashType]*types.Transaction
}

// TxWrap wrap transaction
type TxWrap struct {
	Tx             *types.Transaction
	AddedTimestamp int64
	Height         uint32
	FeePerKB       uint64
}

// NewTransactionPool new a transaction pool.
func NewTransactionPool(parent goprocess.Process, notifiee p2p.Net, c *chain.BlockChain, bus eventbus.Bus) *TransactionPool {
	return &TransactionPool{
		newTxMsgCh:          make(chan p2p.Message, TxMsgBufferChSize),
		newChainUpdateMsgCh: make(chan *chain.UpdateMsg, ChainUpdateMsgBufferChSize),
		proc:                goprocess.WithParent(parent),
		notifiee:            notifiee,
		chain:               c,
		bus:                 bus,
		hashToTx:            make(map[crypto.HashType]*TxWrap),
		hashToOrphanTx:      make(map[crypto.HashType]*types.Transaction),
		outPointToOrphan:    make(map[types.OutPoint]map[crypto.HashType]*types.Transaction),
		outPointToTx:        make(map[types.OutPoint]*types.Transaction),
	}
}

// implement interface service.Server
var _ service.Server = (*TransactionPool)(nil)

// Run launch transaction pool.
func (tx_pool *TransactionPool) Run() error {
	// p2p tx msg
	tx_pool.txNotifee = p2p.NewNotifiee(p2p.TransactionMsg, p2p.Unique, tx_pool.newTxMsgCh)
	tx_pool.notifiee.Subscribe(tx_pool.txNotifee)

	// chain update msg
	tx_pool.bus.Subscribe(eventbus.TopicChainUpdate, tx_pool.receiveChainUpdateMsg)

	tx_pool.proc.Go(tx_pool.loop).SetTeardown(tx_pool.teardown)
	return nil
}

// Proc returns the goprocess of the TransactionPool
func (tx_pool *TransactionPool) Proc() goprocess.Process {
	return tx_pool.proc
}

// Stop the server
func (tx_pool *TransactionPool) Stop() {
	tx_pool.proc.Close()
}

// teardown to clean the process
func (tx_pool *TransactionPool) teardown() error {
	close(tx_pool.newChainUpdateMsgCh)
	close(tx_pool.newTxMsgCh)
	return nil
}

func (tx_pool *TransactionPool) receiveChainUpdateMsg(msg *chain.UpdateMsg) {
	tx_pool.newChainUpdateMsgCh <- msg
}

// handle new tx message from network.
func (tx_pool *TransactionPool) loop(p goprocess.Process) {
	logger.Info("Waitting for new tx message...")
	metricsTicker := time.NewTicker(metricsLoopInterval)
	defer metricsTicker.Stop()
	for {
		select {
		case msg := <-tx_pool.newTxMsgCh:
			tx_pool.processTxMsg(msg)
		case msg := <-tx_pool.newChainUpdateMsgCh:
			tx_pool.processChainUpdateMsg(msg)
		case <-metricsTicker.C:
			metrics.MetricsTxPoolSizeGauge.Update(int64(len(tx_pool.hashToTx)))
			metrics.MetricsOrphanTxPoolSizeGauge.Update(int64(len(tx_pool.hashToOrphanTx)))
		case <-p.Closing():
			logger.Info("Quit transaction pool loop.")
			tx_pool.notifiee.UnSubscribe(tx_pool.txNotifee)
			tx_pool.bus.Unsubscribe(eventbus.TopicChainUpdate, tx_pool.receiveChainUpdateMsg)
			return
		}
	}
}

// chain update message from blockchain: block connection/disconnection
func (tx_pool *TransactionPool) processChainUpdateMsg(msg *chain.UpdateMsg) error {
	block := msg.Block
	if msg.Connected {
		logger.Infof("Block %v connects to main chain", block.BlockHash())
		return tx_pool.removeBlockTxs(block)
	}
	logger.Infof("Block %v disconnects from main chain", block.BlockHash())
	return tx_pool.addBlockTxs(block)
}

// Add all transactions contained in this block into mempool
func (tx_pool *TransactionPool) addBlockTxs(block *types.Block) error {
	for _, tx := range block.Txs[1:] {
		if err := tx_pool.maybeAcceptTx(tx, false /* do not broadcast */, true); err != nil {
			return err
		}
	}
	return nil
}

// Remove all transactions contained in this block and their double spends from main and orphan pool
func (tx_pool *TransactionPool) removeBlockTxs(block *types.Block) error {
	for _, tx := range block.Txs[1:] {
		// Since the passed tx is confirmed in a new block, all its childrent remain valid, thus no recursive removal.
		tx_pool.removeTx(tx, false /* non-recursive */)
		tx_pool.removeDoubleSpendTxs(tx)
		tx_pool.removeOrphan(tx)
		tx_pool.removeDoubleSpendOrphans(tx)
	}
	return nil
}

func (tx_pool *TransactionPool) processTxMsg(msg p2p.Message) error {
	tx := new(types.Transaction)
	if err := tx.Unmarshal(msg.Body()); err != nil {
		return err
	}

	if err := tx_pool.ProcessTx(tx, false); err != nil && util.InArray(err, core.EvilBehavior) {
		tx_pool.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.BadTxEvent)
		return err
	}
	tx_pool.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.NewTxEvent)
	return nil
}

// ProcessTx is used to handle new transactions.
// utxoSet: utxos associated with the tx
func (tx_pool *TransactionPool) ProcessTx(tx *types.Transaction, broadcast bool) error {
	if err := tx_pool.maybeAcceptTx(tx, broadcast, true); err != nil {
		return err
	}

	return tx_pool.processOrphans(tx)
}

// Potentially accept the transaction to the memory pool.
func (tx_pool *TransactionPool) maybeAcceptTx(tx *types.Transaction, broadcast, detectDupOrphan bool) error {

	tx_pool.txMutex.Lock()
	defer tx_pool.txMutex.Unlock()
	txHash, _ := tx.TxHash()

	// Don't accept the transaction if it already exists in the pool.
	// This applies to orphan transactions as well
	if tx_pool.isTransactionInPool(txHash) || detectDupOrphan && tx_pool.isOrphanInPool(txHash) {
		logger.Debugf("Tx %v already exists", txHash.String())
		return core.ErrDuplicateTxInPool
	}

	// TODO: check tx is already exist in the main chain??

	// Perform preliminary sanity checks on the transaction.
	if err := chain.ValidateTransactionPreliminary(tx); err != nil {
		logger.Debugf("Tx %v fails sanity check: %v", txHash.String(), err)
		return err
	}

	// A standalone transaction must not be a coinbase transaction.
	if chain.IsCoinBase(tx) {
		logger.Debugf("Tx %v is an individual coinbase", txHash.String())
		return core.ErrCoinbaseTx
	}

	// ensure it is a standard transaction
	if err := tx_pool.checkTransactionStandard(tx); err != nil {
		logger.Debugf("Tx %v is not standard: %v", txHash.String(), err)
		return core.ErrNonStandardTransaction
	}

	// Quickly detects if the tx double spends with any transaction in the pool.
	// Double spending with the main chain txs will be checked in ValidateTxInputs.
	if err := tx_pool.checkPoolDoubleSpend(tx); err != nil {
		logger.Debugf("Tx %v double spends outputs spent by other pending txs: %v", txHash.String(), err)
		return err
	}

	utxoSet, err := tx_pool.getTxUtxoSet(tx)
	if err != nil {
		return err
	}

	// Add orphan transaction
	if tx_pool.isOrphan(utxoSet, tx) {
		tx_pool.addOrphan(tx)
		return core.ErrOrphanTransaction
	}

	// TODO: sequence lock

	nextBlockHeight := tx_pool.chain.LongestChainHeight + 1

	txFee, err := chain.ValidateTxInputs(utxoSet, tx, nextBlockHeight)
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
	if err = chain.ValidateTxScripts(utxoSet, tx); err != nil {
		return err
	}

	feePerKB := txFee * 1000 / (uint64)(txSize)
	// add transaction to pool.
	tx_pool.addTx(tx, nextBlockHeight, feePerKB)

	logger.Debugf("Accepted transaction %v (pool size: %v)", txHash.String(), len(tx_pool.hashToTx))
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
func (tx_pool *TransactionPool) isOrphan(utxoSet *chain.UtxoSet, tx *types.Transaction) bool {
	for _, txIn := range tx.Vin {
		utxo := utxoSet.FindUtxo(txIn.PrevOutPoint)
		if utxo == nil || utxo.IsSpent {
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
		if _, exists := tx_pool.outPointToTx[txIn.PrevOutPoint]; exists {
			return core.ErrOutPutAlreadySpent
		}
	}
	return nil
}

func (tx_pool *TransactionPool) getTxUtxoSet(tx *types.Transaction) (*chain.UtxoSet, error) {
	utxoSet := chain.NewUtxoSet()
	if err := utxoSet.LoadTxUtxos(tx, tx_pool.chain.DB()); err != nil {
		return nil, err
	}

	// Outputs of existing txs in main pool can also be spent
	for _, txIn := range tx.Vin {
		utxo := utxoSet.FindUtxo(txIn.PrevOutPoint)
		if utxo != nil && !utxo.IsSpent {
			continue
		}
		if poolTxWrap, exists := tx_pool.hashToTx[txIn.PrevOutPoint.Hash]; exists {
			utxoSet.AddUtxo(poolTxWrap.Tx, txIn.PrevOutPoint.Index, poolTxWrap.Height)
		}
	}
	return utxoSet, nil
}

// ProcessOrphans used to handle orphan transactions
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
				// Do not reject a tx simply because it's already in orphan pool
				// since it may be acceptable now
				if err := tx_pool.maybeAcceptTx(orphan, false,
					false /* no duplicate orphan check */); err != nil {
					continue
				}
				tx_pool.removeOrphan(orphan)
				acceptedTxs = append(acceptedTxs, orphan)
				// Once one child orphan is accepted, others will definitely be rejected due to double spending.
				// So no need to continue here
				break
			}
		}
	}

	// Remove any orphans that double spends with the accepted transactions.
	for _, acceptedTx := range acceptedTxs {
		tx_pool.removeDoubleSpendOrphans(acceptedTx)
	}

	return nil
}

// Add transaction into tx pool
func (tx_pool *TransactionPool) addTx(tx *types.Transaction, height uint32, feePerKB uint64) {
	txHash, _ := tx.TxHash()

	txWrap := &TxWrap{
		Tx:             tx,
		AddedTimestamp: time.Now().Unix(),
		Height:         height,
		FeePerKB:       feePerKB,
	}
	tx_pool.hashToTx[*txHash] = txWrap

	// outputs spent by this new tx
	for _, txIn := range tx.Vin {
		tx_pool.outPointToTx[txIn.PrevOutPoint] = tx
	}

	// TODO: build address - tx index.
}

// Remove transaction from tx pool. Note we do not recursively remove dependent txs here
func (tx_pool *TransactionPool) removeTx(tx *types.Transaction, recursive bool) {
	txHash, _ := tx.TxHash()

	// Unspend the referenced outpoints.
	for _, txIn := range tx.Vin {
		delete(tx_pool.outPointToTx, txIn.PrevOutPoint)
	}
	delete(tx_pool.hashToTx, *txHash)

	if !recursive {
		return
	}
	// Start with processing at least the passed tx.
	removedTxs := []*types.Transaction{tx}
	// Note: use index here instead of range because removedTxs can be extended inside the loop
	for i := 0; i < len(removedTxs); i++ {
		removedTx := removedTxs[i]
		removedTxHash, _ := removedTx.TxHash()
		// Look up all txs that spend output from the tx we just removed.
		outPoint := types.OutPoint{Hash: *removedTxHash}
		for txOutIdx := range removedTx.Vout {
			outPoint.Index = uint32(txOutIdx)

			childTx, exists := tx_pool.outPointToTx[outPoint]
			if !exists {
				continue
			}

			// Move the child tx from main pool to orphan pool
			// The outer loop is already a recursion, so no more recursion within
			tx_pool.removeTx(childTx, false /* non-recursive */)
			tx_pool.addOrphan(childTx)

			removedTxs = append(removedTxs, childTx)
		}
	}
}

// removeDoubleSpendTxs removes all txs from the main pool, which double spend the passed transaction.
func (tx_pool *TransactionPool) removeDoubleSpendTxs(tx *types.Transaction) {
	for _, txIn := range tx.Vin {
		if doubleSpentTx, exists := tx_pool.outPointToTx[txIn.PrevOutPoint]; exists {
			tx_pool.removeTx(doubleSpentTx, true /* recursive */)
		}
	}
}

// Add orphan
func (tx_pool *TransactionPool) addOrphan(tx *types.Transaction) {
	// TODO: limit orphan pool size

	txHash, _ := tx.TxHash()
	tx_pool.hashToOrphanTx[*txHash] = tx

	for _, txIn := range tx.Vin {
		if _, exists := tx_pool.outPointToOrphan[txIn.PrevOutPoint]; !exists {
			tx_pool.outPointToOrphan[txIn.PrevOutPoint] =
				make(map[crypto.HashType]*types.Transaction)
		}
		tx_pool.outPointToOrphan[txIn.PrevOutPoint][*txHash] = tx
	}

	logger.Debugf("Stored orphan transaction %v", txHash.String())
}

// Remove orphan
func (tx_pool *TransactionPool) removeOrphan(tx *types.Transaction) {
	txHash, _ := tx.TxHash()
	// Outpoints this orphan spends
	for _, txIn := range tx.Vin {
		// Sibling orphans spending the same outpoint as this tx input
		siblingOrphans, exists := tx_pool.outPointToOrphan[txIn.PrevOutPoint]
		if !exists {
			continue
		}
		delete(siblingOrphans, *txHash)

		// Delete the outpoint entry entirely if there are no longer any dependent orphans.
		if len(siblingOrphans) == 0 {
			delete(tx_pool.outPointToOrphan, txIn.PrevOutPoint)
		}
	}

	delete(tx_pool.hashToOrphanTx, *txHash)
	logger.Debugf("Removed orphan transaction %v", txHash.String())
}

// removeDoubleSpendOrphans removes all orphans from the orphan pool, which double spend the passed transaction.
func (tx_pool *TransactionPool) removeDoubleSpendOrphans(tx *types.Transaction) {
	for _, txIn := range tx.Vin {
		for _, orphan := range tx_pool.outPointToOrphan[txIn.PrevOutPoint] {
			tx_pool.removeOrphan(orphan)
		}
	}
}

// GetAllTxs returns all transactions in mempool
func (tx_pool *TransactionPool) GetAllTxs() []*TxWrap {
	txs := make([]*TxWrap, len(tx_pool.hashToTx))
	i := 0
	for _, tx := range tx_pool.hashToTx {
		txs[i] = tx
		i++
	}
	return txs
}

func calcRequiredMinFee(txSize int) uint64 {
	return 0
}
