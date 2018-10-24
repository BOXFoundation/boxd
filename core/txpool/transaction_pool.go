// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txpool

import (
	"errors"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/jbenet/goprocess"
)

// const defines constants
const (
	TxMsgBufferChSize          = 65536
	ChainUpdateMsgBufferChSize = 65536
)

var logger = log.NewLogger("txpool") // logger

var _ service.TxHandler = (*TransactionPool)(nil)

// TransactionPool define struct.
type TransactionPool struct {
	notifiee            p2p.Net
	newTxMsgCh          chan p2p.Message
	newChainUpdateMsgCh chan p2p.Message
	proc                goprocess.Process
	chain               *chain.BlockChain
	hashToTx            map[crypto.HashType]*TxWrap
	txMutex             sync.Mutex
	hashToOrphanTx      map[crypto.HashType]*types.Transaction
	outPointToOrphan    map[types.OutPoint]map[crypto.HashType]*types.Transaction
	stxoSet             map[types.OutPoint]*types.Transaction
}

// TxWrap wrap transaction
type TxWrap struct {
	Tx             *types.Transaction
	AddedTimestamp int64
	Height         int32
	FeePerKB       int64
}

// NewTransactionPool new a transaction pool.
func NewTransactionPool(parent goprocess.Process, notifiee p2p.Net, chain *chain.BlockChain) *TransactionPool {
	return &TransactionPool{
		newTxMsgCh:          make(chan p2p.Message, TxMsgBufferChSize),
		newChainUpdateMsgCh: make(chan p2p.Message, ChainUpdateMsgBufferChSize),
		proc:                goprocess.WithParent(parent),
		notifiee:            notifiee,
		chain:               chain,
		hashToTx:            make(map[crypto.HashType]*TxWrap),
		hashToOrphanTx:      make(map[crypto.HashType]*types.Transaction),
		outPointToOrphan:    make(map[types.OutPoint]map[crypto.HashType]*types.Transaction),
		stxoSet:             make(map[types.OutPoint]*types.Transaction),
	}
}

func (tx_pool *TransactionPool) subscribeMessageNotifiee(notifiee p2p.Net) {
	notifiee.Subscribe(p2p.NewNotifiee(p2p.TransactionMsg, tx_pool.newTxMsgCh))
	notifiee.Subscribe(p2p.NewNotifiee(p2p.ChainUpdateMsg, tx_pool.newChainUpdateMsgCh))
}

// implement interface service.Server
var _ service.Server = (*TransactionPool)(nil)

// Run launch transaction pool.
func (tx_pool *TransactionPool) Run() error {
	tx_pool.subscribeMessageNotifiee(tx_pool.notifiee)
	tx_pool.proc.Go(tx_pool.loop)

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

// handle new tx message from network.
func (tx_pool *TransactionPool) loop(p goprocess.Process) {
	logger.Info("Waitting for new tx message...")
	for {
		select {
		case msg := <-tx_pool.newTxMsgCh:
			tx_pool.processTxMsg(msg)
		case msg := <-tx_pool.newChainUpdateMsgCh:
			tx_pool.processChainUpdateMsg(msg)
		case <-p.Closing():
			logger.Info("Quit transaction pool loop.")
			return
		}
	}
}

// chain update message from blockchain: block connection/disconnection
func (tx_pool *TransactionPool) processChainUpdateMsg(msg p2p.Message) error {
	localMessage, ok := msg.(*chain.LocalMessage)
	if !ok {
		logger.Errorf("Received non-local message")
		return core.ErrNonLocalMessage
	}
	chainUpdateMsg, ok := localMessage.Data().(chain.UpdateMsg)
	if !ok {
		logger.Errorf("Received local message is not a chain update")
		return core.ErrLocalMessageNotChainUpdate
	}

	block := chainUpdateMsg.Block
	if chainUpdateMsg.Connected {
		logger.Infof("Block %v connects to main chain", block.BlockHash())
		return tx_pool.removeBlockTxs(block)
	}
	logger.Infof("Block %v disconnects from main chain", block.BlockHash())
	return tx_pool.addBlockTxs(block)
}

// Add all transactions contained in this block into mempool
func (tx_pool *TransactionPool) addBlockTxs(block *types.Block) error {
	for _, tx := range block.Txs[1:] {
		utxoSet := chain.NewUtxoSet()
		if err := utxoSet.LoadTxUtxos(tx, tx_pool.chain.DbTx); err != nil {
			return err
		}
		if err := tx_pool.maybeAcceptTx(tx, tx_pool.chain.LongestChainHeight, utxoSet, false /* do not broadcast */); err != nil {
			return err
		}
	}
	return nil
}

// Remove all transactions contained in this block from mempool
func (tx_pool *TransactionPool) removeBlockTxs(block *types.Block) error {
	for _, tx := range block.Txs[1:] {
		tx_pool.removeTx(tx)
		tx_pool.removeDoubleSpends(tx)
		tx_pool.removeOrphan(tx)
	}
	return nil
}

func (tx_pool *TransactionPool) processTxMsg(msg p2p.Message) error {
	tx := new(types.Transaction)
	if err := tx.Unmarshal(msg.Body()); err != nil {
		return err
	}

	return tx_pool.ProcessTx(tx, false)
}

// ProcessTx is used to handle new transactions.
// utxoSet: utxos associated with the tx
func (tx_pool *TransactionPool) ProcessTx(tx *types.Transaction, broadcast bool) error {

	// TODO: check tx is already exist in the main chain??
	utxoSet := chain.NewUtxoSet()
	if err := utxoSet.LoadTxUtxos(tx, tx_pool.chain.DbTx); err != nil {
		return err
	}
	// Note: put actual implementation in doProcessTx() for unit test purpose
	return tx_pool.doProcessTx(tx, tx_pool.chain.LongestChainHeight, utxoSet, broadcast)
}

func (tx_pool *TransactionPool) doProcessTx(tx *types.Transaction, currChainHeight int32,
	utxoSet *chain.UtxoSet, broadcast bool) error {

	if err := tx_pool.maybeAcceptTx(tx, currChainHeight, utxoSet, broadcast); err != nil {
		return err
	}

	return tx_pool.processOrphans(tx)
}

// Potentially accept the transaction to the memory pool.
func (tx_pool *TransactionPool) maybeAcceptTx(tx *types.Transaction, currChainHeight int32,
	utxoSet *chain.UtxoSet, broadcast bool) error {

	tx_pool.txMutex.Lock()
	defer tx_pool.txMutex.Unlock()
	txHash, _ := tx.TxHash()

	// Don't accept the transaction if it already exists in the pool.
	// This applies to orphan transactions as well
	if tx_pool.isTransactionInPool(txHash) || tx_pool.isOrphanInPool(txHash) {
		logger.Debugf("Tx %v already exists", txHash)
		return core.ErrDuplicateTxInPool
	}

	// Perform preliminary sanity checks on the transaction.
	if err := chain.ValidateTransactionPreliminary(tx); err != nil {
		logger.Debugf("Tx %v fails sanity check: %v", txHash, err)
		return err
	}

	// A standalone transaction must not be a coinbase transaction.
	if chain.IsCoinBase(tx) {
		logger.Debugf("Tx %v is an individual coinbase", txHash)
		return core.ErrCoinbaseTx
	}

	nextBlockHeight := currChainHeight + 1

	// ensure it is a standard transaction
	if err := tx_pool.checkTransactionStandard(tx); err != nil {
		logger.Debugf("Tx %v is not standard: %v", txHash, err)
		return core.ErrNonStandardTransaction
	}

	// The transaction must not use any of the same outputs as other transactions already in the pool.
	// This check only detects double spends within the transaction pool itself.
	// Double spending coins from the main chain will be checked in ValidateTxInputs.
	if err := tx_pool.checkPoolDoubleSpend(tx); err != nil {
		logger.Debugf("Tx %v double spends outputs spent by other pending txs: %v", txHash, err)
		return err
	}

	// Add orphan transaction
	if tx_pool.isOrphan(utxoSet, tx) {
		tx_pool.addOrphan(tx)
		return core.ErrOrphanTransaction
	}

	// TODO: sequence lock

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
func (tx_pool *TransactionPool) isOrphan(utxoSet *chain.UtxoSet, tx *types.Transaction) bool {
	for _, txIn := range tx.Vin {
		// Spend main chain UTXOs
		utxo := utxoSet.FindUtxo(txIn.PrevOutPoint)
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
			return core.ErrOutPutAlreadySpent
		}
	}
	return nil
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
				utxoSet := chain.NewUtxoSet()
				if err := utxoSet.LoadTxUtxos(tx, tx_pool.chain.DbTx); err != nil {
					return err
				}
				if err := tx_pool.maybeAcceptTx(orphan, tx_pool.chain.LongestChainHeight, utxoSet, false); err != nil {
					continue
				}
				tx_pool.removeOrphan(orphan)
				acceptedTxs = append(acceptedTxs, orphan)
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
		Tx:             tx,
		AddedTimestamp: time.Now().Unix(),
		Height:         height,
		FeePerKB:       feePerKB,
	}
	tx_pool.hashToTx[*txHash] = txWrap

	// outputs spent by this new tx
	for _, txIn := range tx.Vin {
		tx_pool.stxoSet[txIn.PrevOutPoint] = tx
	}

	// TODO: build address - tx index.
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

func calcRequiredMinFee(txSize int) int64 {
	return 0
}
