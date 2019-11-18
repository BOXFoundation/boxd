// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txpool

import (
	"fmt"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/metrics"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/vm"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

// const defines constants
const (
	TxMsgBufferChSize          = 65536
	ChainUpdateMsgBufferChSize = 65536
	TxScriptBufferChSize       = 65536

	metricsLoopInterval = 1 * time.Second

	// Note: reuse metrics ticker to save cost
	txTTL = 3600 * time.Second
)

var logger = log.NewLogger("txpool") // logger

var _ service.TxHandler = (*TransactionPool)(nil)

// Wrapper for info needed to validate a tx's script
type txScriptWrap struct {
	tx      *types.Transaction
	utxoSet *chain.UtxoSet
}

// TransactionPool define struct.
type TransactionPool struct {
	notifiee            p2p.Net
	newTxMsgCh          chan p2p.Message
	newChainUpdateMsgCh chan *chain.UpdateMsg
	newTxScriptCh       chan *txScriptWrap
	txNotifee           *p2p.Notifiee
	proc                goprocess.Process
	chain               *chain.BlockChain
	hashToTx            *sync.Map
	bus                 eventbus.Bus
	// outpoint -> tx spending it; outpoints can be arbitrary, valid (exists and unspent) or invalid
	// types.OutPoint -> *types.Transaction
	outPointToTx *sync.Map
	txMutex      sync.Mutex
	// crypto.HashType -> *types.TxWrap
	hashToOrphanTx *sync.Map
	// outpoint -> orphans spending it; outpoints can be arbitrary, valid or invalid
	// Use map here since there can be multiple spending txs and we don't know which
	// one will be accepted, unlike in outPointToTx where first seen tx is accepted
	// types.OutPoint -> (crypto.HashType -> *types.Transaction)
	outPointToOrphan *sync.Map
	txcache          *lru.Cache
}

// NewTransactionPool new a transaction pool.
func NewTransactionPool(parent goprocess.Process, notifiee p2p.Net, c *chain.BlockChain, bus eventbus.Bus) *TransactionPool {
	txcache, _ := lru.New(65536)
	return &TransactionPool{
		newTxMsgCh:          make(chan p2p.Message, TxMsgBufferChSize),
		newChainUpdateMsgCh: make(chan *chain.UpdateMsg, ChainUpdateMsgBufferChSize),
		newTxScriptCh:       make(chan *txScriptWrap, TxScriptBufferChSize),
		proc:                goprocess.WithParent(parent),
		notifiee:            notifiee,
		chain:               c,
		bus:                 bus,
		hashToTx:            new(sync.Map),
		hashToOrphanTx:      new(sync.Map),
		outPointToOrphan:    new(sync.Map),
		outPointToTx:        new(sync.Map),
		txcache:             txcache,
	}
}

// implement interface service.Server
var _ service.Server = (*TransactionPool)(nil)

// Run launch transaction pool.
func (tx_pool *TransactionPool) Run() error {
	// p2p tx msg
	tx_pool.txNotifee = p2p.NewNotifiee(p2p.TransactionMsg, tx_pool.newTxMsgCh)
	tx_pool.notifiee.Subscribe(tx_pool.txNotifee)

	// chain update msg
	tx_pool.bus.SubscribeAsync(eventbus.TopicChainUpdate, tx_pool.receiveChainUpdateMsg, true)

	// chain update msg
	tx_pool.bus.SubscribeAsync(eventbus.TopicInvalidTx, tx_pool.removeTx, true)

	tx_pool.proc.Go(tx_pool.loop)

	tx_pool.proc.Go(tx_pool.cleanExpiredTxsLoop)
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
			if err := tx_pool.processTxMsg(msg); err != nil {
				logger.Warnf("Failed to processTxMsg from %s. Err: %s", msg.From().Pretty(), err)
			}
		case msg := <-tx_pool.newChainUpdateMsgCh:
			tx_pool.processChainUpdateMsg(msg)
		case <-metricsTicker.C:
			metrics.MetricsTxPoolSizeGauge.Update(int64(lengthOfSyncMap(tx_pool.hashToTx)))
			metrics.MetricsOrphanTxPoolSizeGauge.Update(int64(lengthOfSyncMap(tx_pool.hashToOrphanTx)))
			if time.Now().Unix()%30 == 0 {
				var hashstr string
				tx_pool.hashToTx.Range(func(k, v interface{}) bool {
					txwrap := v.(*types.TxWrap)
					hash, _ := txwrap.Tx.TxHash()
					hashstr += hash.String()
					hashstr += ","
					return true
				})
				if len(hashstr) > 1 {
					logger.Debugf("tx_pool info: %s", string([]rune(hashstr)[:len(hashstr)-1]))
				}
			}
		case txScript := <-tx_pool.newTxScriptCh:
			go tx_pool.checkTxScript(txScript)
		case <-p.Closing():
			logger.Info("Quit transaction pool loop.")
			tx_pool.notifiee.UnSubscribe(tx_pool.txNotifee)
			tx_pool.bus.Unsubscribe(eventbus.TopicChainUpdate, tx_pool.receiveChainUpdateMsg)
			tx_pool.bus.Unsubscribe(eventbus.TopicInvalidTx, tx_pool.removeTx)
			return
		}
	}
}

func (tx_pool *TransactionPool) cleanExpiredTxsLoop(p goprocess.Process) {
	ticker := time.NewTicker(txTTL)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			tx_pool.cleanExpiredTxs()
		case <-p.Closing():
			logger.Info("Quit cleanUpTxPool loop.")
			return
		}
	}
}

// chain update message from blockchain: block connection/disconnection
func (tx_pool *TransactionPool) processChainUpdateMsg(msg *chain.UpdateMsg) {

	for _, v := range msg.DetachBlocks {
		logger.Debugf("Block %s %d disconnects from main chain", v.BlockHash(), v.Header.Height)
		for _, tx := range v.Txs[1:] {
			txHash, _ := tx.TxHash()
			if tx_pool.txcache.Contains(*txHash) {
				tx_pool.txcache.Remove(*txHash)
			}
			if err := tx_pool.maybeAcceptTx(tx, core.DefaultMode /* do not broadcast */, true); err != nil {
				logger.Errorf("Failed to add tx into mem_pool when block disconnect. TxHash: %s, err: %v", txHash, err)
			}
		}
		// remove related child txs.
		for _, tx := range v.Txs {
			removedTxHash, _ := tx.TxHash()
			outPoint := types.OutPoint{Hash: *removedTxHash}
			for txOutIdx := range tx.Vout {
				outPoint.Index = uint32(txOutIdx)
				childTx := tx_pool.FindTransaction(&outPoint)
				if childTx == nil {
					continue
				}
				hash, _ := childTx.TxHash()
				logger.Debugf("Remove related child tx %s when block %v disconnects from main chain", hash, v.BlockHash())
				tx_pool.removeTx(childTx, true)
			}
		}
	}
	for _, v := range msg.AttachBlocks {
		logger.Debugf("Block %s %d connects to main chain", v.BlockHash(), v.Header.Height)
		tx_pool.removeBlockTxs(v)
	}

}

// Remove all transactions contained in this block and their double spends from main and orphan pool
func (tx_pool *TransactionPool) removeBlockTxs(block *types.Block) error {
	for _, tx := range block.Txs[1:] {
		txHash, _ := tx.TxHash()
		tx_pool.txcache.Add(*txHash, true)
		// Since the passed tx is confirmed in a new block, all its childrent remain valid, thus no recursive removal.
		tx_pool.removeTx(tx, false /* non-recursive */)
		// tx_pool.removeDoubleSpendTxs(tx)
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
	hash, _ := tx.TxHash()
	logger.Debugf("Start to process tx from network. Hash: %v", hash)

	if err := tx_pool.ProcessTx(tx, core.RelayMode); err != nil {
		for _, e := range core.EvilBehavior {
			if err.Error() == e.Error() {
				tx_pool.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.BadTxEvent)
				return err
			}
		}
	}
	tx_pool.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.NewTxEvent)
	return nil
}

// ProcessTx is used to handle new transactions.
// utxoSet: utxos associated with the tx
func (tx_pool *TransactionPool) ProcessTx(tx *types.Transaction, transferMode core.TransferMode) error {
	if err := tx_pool.maybeAcceptTx(tx, transferMode, true); err != nil {
		txHash, _ := tx.TxHash()
		logger.Warnf("Failed to accept tx. TxHash: %s, Err: %s", txHash, err)
		return err
	}
	return tx_pool.processOrphans(tx)
}

// Potentially accept the transaction to the memory pool.
func (tx_pool *TransactionPool) maybeAcceptTx(
	tx *types.Transaction, transferMode core.TransferMode, detectDupOrphan bool,
) error {
	txHash, _ := tx.TxHash()
	logger.Debugf("Maybe accept tx. Hash: %v", txHash)
	tx_pool.txMutex.Lock()
	defer tx_pool.txMutex.Unlock()

	// Don't accept the transaction if it already exists in the pool.
	// This applies to orphan transactions as well
	if tx_pool.isTransactionInPool(txHash) ||
		detectDupOrphan && tx_pool.isOrphanInPool(txHash) ||
		tx_pool.txcache.Contains(*txHash) {
		logger.Debugf("Tx %v already exists", txHash.String())
		return core.ErrDuplicateTxInPool
	}

	// Perform preliminary sanity checks on the transaction.
	if err := chain.ValidateTransactionPreliminary(tx); err != nil {
		logger.Errorf("Tx %v fails sanity check: %v", txHash.String(), err)
		return err
	}

	// A standalone transaction must not be a coinbase transaction.
	if chain.IsCoinBase(tx) {
		logger.Errorf("Tx %v is an individual coinbase", txHash.String())
		return core.ErrCoinbaseTx
	}

	// ensure it is a standard transaction
	if !txlogic.IsStandardTx(tx, tx_pool.chain.TailState()) {
		logger.Errorf("Tx %v is not standard", txHash.String())
		return core.ErrNonStandardTransaction
	}
	// if err := tx_pool.chain.Consensus().VerifyTx(tx); err != nil {
	// 	logger.Errorf("Failed to verify tx in consensus. Err: %v", txHash.String(), err)
	// 	return err
	// }
	utxoSet, err := chain.GetExtendedTxUtxoSet(tx, tx_pool.chain.DB(), tx_pool.hashToTx)
	if err != nil {
		logger.Errorf("Could not get extended utxo set for tx %v", txHash)
		return err
	}
	// A tx is an orphan if any of its spending utxo does not exist
	if utxoSet.TxInputAmount(tx) == 0 {
		// Add orphan transaction
		tx_pool.addOrphan(tx)
		return core.ErrOrphanTransaction
	}
	nextBlockHeight := tx_pool.chain.LongestChainHeight + 1

	if err := tx_pool.CheckGasAndNonce(tx, utxoSet); err != nil {
		return err
	}
	// Quickly detects if the tx double spends with any transaction in the pool.
	// Double spending with the main chain txs will be checked in ValidateTxInputs.
	if err := tx_pool.checkDoubleSpend(tx); err != nil {
		logger.Errorf("Tx %v double spends outputs spent by other pending txs: %v", txHash.String(), err)
		return err
	}
	if tx.Type == types.PayToPubkTx {
		// this tx may be a contract tx that transfer box to a contract address
		// so reset tx type and parse tx type when this tx is packed in bpos
		tx.Type = types.UnknownTx
	}
	// To check script later so main thread is not blocked
	tx_pool.newTxScriptCh <- &txScriptWrap{tx, utxoSet}

	// add transaction to pool.
	tx_pool.addTx(tx, nextBlockHeight)

	logger.Debugf("Accepted new tx. Hash: %v", txHash)
	tx_pool.txcache.Add(*txHash, true)
	switch transferMode {
	case core.BroadcastMode:
		return tx_pool.notifiee.Broadcast(p2p.TransactionMsg, tx)
	case core.RelayMode:
		return tx_pool.notifiee.Relay(p2p.TransactionMsg, tx)
	default:
	}
	return nil
}

func (tx_pool *TransactionPool) isTransactionInPool(txHash *crypto.HashType) bool {
	_, exists := tx_pool.hashToTx.Load(*txHash)
	return exists
}

// FindTransaction returns tx with given outpoint
func (tx_pool *TransactionPool) FindTransaction(outpoint *types.OutPoint) *types.Transaction {
	if tx, exists := tx_pool.outPointToTx.Load(*outpoint); exists {
		return tx.(*types.Transaction)
	}
	return nil
}

func (tx_pool *TransactionPool) isOrphanInPool(txHash *crypto.HashType) bool {
	_, exists := tx_pool.hashToOrphanTx.Load(*txHash)
	return exists
}

func (tx_pool *TransactionPool) checkDoubleSpend(tx *types.Transaction) error {
	txHash, _ := tx.TxHash()
	// check double spend txs
	if dsOps, dsTxs := tx_pool.getPoolDoubleSpendTxs(tx); len(dsTxs) > 0 {
		for i := 0; i < len(dsTxs); i++ {
			dsTxHash, _ := dsTxs[i].TxHash()
			logger.Errorf("tx %s has a double spend outpoint: %s, revelant tx hash: %v",
				txHash, dsOps[i], dsTxHash)
		}
		return core.ErrOutPutAlreadySpent
	}
	// check double spend inside tx
	switch len(tx.Vin) {
	case 1:
	case 2:
		if tx.Vin[0].PrevOutPoint == tx.Vin[1].PrevOutPoint {
			logger.Warnf("tx %s has the same vin, %+v", txHash, tx)
			return core.ErrDoubleSpendTx
		}
	default:
		opSet := make(map[types.OutPoint]struct{})
		for _, txIn := range tx.Vin {
			if _, exists := opSet[txIn.PrevOutPoint]; exists {
				logger.Errorf("tx %s have the same vin: %+v", txHash, tx)
				return core.ErrDoubleSpendTx
			}
			opSet[txIn.PrevOutPoint] = struct{}{}
		}
	}
	return nil
}

func (tx_pool *TransactionPool) getPoolDoubleSpendTxs(
	tx *types.Transaction,
) ([]*types.OutPoint, []*types.Transaction) {
	var (
		ops []*types.OutPoint
		txs []*types.Transaction
	)
	for _, txIn := range tx.Vin {
		if tx := tx_pool.FindTransaction(&txIn.PrevOutPoint); tx != nil {
			ops = append(ops, &txIn.PrevOutPoint)
			txs = append(txs, tx)
		}
	}
	return ops, txs
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
			v, exists := tx_pool.outPointToOrphan.Load(outPoint)
			if !exists {
				continue
			}
			orphans := v.(*sync.Map)
			orphans.Range(func(k, v interface{}) bool {
				orphan := v.(*types.Transaction)
				if err := tx_pool.maybeAcceptTx(orphan, core.DefaultMode, false); err != nil {
					txHash, _ := orphan.TxHash()
					logger.Warnf("Failed to accept orphan tx. TxHash: %s, Err: %s", txHash, err)
					return true
				}
				tx_pool.removeOrphan(orphan)
				acceptedTxs = append(acceptedTxs, orphan)
				return false
			})
		}
	}

	// Remove any orphans that double spends with the accepted transactions.
	for _, acceptedTx := range acceptedTxs {
		tx_pool.removeDoubleSpendOrphans(acceptedTx)
	}
	return nil
}

// Add transaction into tx pool
func (tx_pool *TransactionPool) addTx(tx *types.Transaction, height uint32) {
	txHash, _ := tx.TxHash()
	txWrap := &types.TxWrap{
		Tx:             tx,
		AddedTimestamp: time.Now().Unix(),
		Height:         height,
		IsScriptValid:  false,
	}
	tx_pool.hashToTx.Store(*txHash, txWrap)
	// outputs spent by this new tx
	for _, txIn := range tx.Vin {
		tx_pool.outPointToTx.Store(txIn.PrevOutPoint, tx)
	}

	// TODO: build address - tx index.
}

// Remove transaction from tx pool. Note we do not recursively remove dependent txs here
func (tx_pool *TransactionPool) removeTx(tx *types.Transaction, recursive bool) {
	txHash, _ := tx.TxHash()
	tx_pool.hashToTx.Delete(*txHash)

	// Unspend the referenced outpoints.
	for _, txIn := range tx.Vin {
		if dsTx := tx_pool.FindTransaction(&txIn.PrevOutPoint); dsTx != nil {
			tx_pool.outPointToTx.Delete(txIn.PrevOutPoint)
			doubleSpentTxHash, _ := dsTx.TxHash()
			if !doubleSpentTxHash.IsEqual(txHash) {
				logger.Warnf("Remove double spend tx when main chain update. TxHash: %v", doubleSpentTxHash)
				tx_pool.hashToTx.Delete(*doubleSpentTxHash)
			}
		}
	}

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

			childTx := tx_pool.FindTransaction(&outPoint)
			if childTx == nil {
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
		if dsTx := tx_pool.FindTransaction(&txIn.PrevOutPoint); dsTx != nil {
			tx_pool.removeTx(dsTx, true /* recursive */)
		}
	}
}

// Add orphan
func (tx_pool *TransactionPool) addOrphan(tx *types.Transaction) {
	txWrap := &types.TxWrap{
		Tx:             tx,
		AddedTimestamp: time.Now().Unix(),
	}
	txHash, _ := tx.TxHash()
	tx_pool.hashToOrphanTx.Store(*txHash, txWrap)
	for _, txIn := range tx.Vin {
		v := new(sync.Map)
		v.Store(*txHash, tx)
		tx_pool.outPointToOrphan.LoadOrStore(txIn.PrevOutPoint, v)
	}

	logger.Debugf("Stored orphan transaction %v", txHash.String())
}

// Remove orphan
func (tx_pool *TransactionPool) removeOrphan(tx *types.Transaction) {
	txHash, _ := tx.TxHash()
	// Outpoints this orphan spends
	for _, txIn := range tx.Vin {
		v, exists := tx_pool.outPointToOrphan.Load(txIn.PrevOutPoint)
		if !exists {
			continue
		}
		siblingOrphans := v.(*sync.Map)
		siblingOrphans.Delete(*txHash)

		var counter int
		siblingOrphans.Range(func(k, v interface{}) bool {
			counter++
			if counter > 0 {
				return false
			}
			return true
		})
		// Delete the outpoint entry entirely if there are no longer any dependent orphans.
		if counter == 0 {
			tx_pool.outPointToOrphan.Delete(txIn.PrevOutPoint)
		}
	}

	tx_pool.hashToOrphanTx.Delete(*txHash)
}

// removeDoubleSpendOrphans removes all orphans from the orphan pool, which double spend the passed transaction.
func (tx_pool *TransactionPool) removeDoubleSpendOrphans(tx *types.Transaction) {
	for _, txIn := range tx.Vin {
		if v, exists := tx_pool.outPointToOrphan.Load(txIn.PrevOutPoint); exists {
			temp := v.(*sync.Map)
			temp.Range(func(k, v interface{}) bool {
				orphan := v.(*types.Transaction)
				tx_pool.removeOrphan(orphan)
				return true
			})
		}
	}
}

// check admitted tx's script
func (tx_pool *TransactionPool) checkTxScript(txScript *txScriptWrap) {
	// verify crypto signatures for each input
	if _, err := chain.CheckTxScripts(txScript.utxoSet, txScript.tx, false); err != nil {
		// remove
		txHash, _ := txScript.tx.TxHash()
		logger.Errorf("tx %v script verification failed", txHash)
		tx_pool.removeTx(txScript.tx, true)
		return
	}

	txHash, _ := txScript.tx.TxHash()
	v, exists := tx_pool.hashToTx.Load(*txHash)
	if !exists {
		// already removed
		return
	}
	tx := v.(*types.TxWrap)
	tx.IsScriptValid = true
}

func (tx_pool *TransactionPool) cleanExpiredTxs() {
	var expiredOrphans []*types.Transaction
	var expiredTxs []*types.Transaction

	now := time.Now()
	tx_pool.hashToOrphanTx.Range(func(k, v interface{}) bool {
		orphan := v.(*types.TxWrap)
		if now.After(time.Unix(orphan.AddedTimestamp, 0).Add(txTTL)) {
			// Note: do not delete while range looping over map; delete after loop is over
			expiredOrphans = append(expiredOrphans, orphan.Tx)
		}
		return true
	})

	tx_pool.hashToTx.Range(func(k, v interface{}) bool {
		txWrap := v.(*types.TxWrap)
		if now.After(time.Unix(txWrap.AddedTimestamp, 0).Add(txTTL)) {
			// Note: do not delete while range looping over map; delete after loop is over
			expiredTxs = append(expiredTxs, txWrap.Tx)
		}
		return true
	})

	for _, expiredOrphan := range expiredOrphans {
		txHash, _ := expiredOrphan.TxHash()
		logger.Debugf("Remove expired orphan %v", txHash)
		tx_pool.removeOrphan(expiredOrphan)
	}

	for _, expiredTx := range expiredTxs {
		txHash, _ := expiredTx.TxHash()
		logger.Debugf("Remove expired tx %v", txHash)
		tx_pool.removeTx(expiredTx, true)
	}
}

// GetAllTxs returns all transactions in mempool
func (tx_pool *TransactionPool) GetAllTxs() []*types.TxWrap {
	var txs []*types.TxWrap
	tx_pool.hashToTx.Range(func(k, v interface{}) bool {
		txs = append(txs, v.(*types.TxWrap))
		return true
	})
	return txs
}

// GetOrphaTxs returns all orpha txs in mempool
func (tx_pool *TransactionPool) GetOrphaTxs() []*types.TxWrap {
	var txs []*types.TxWrap
	tx_pool.hashToOrphanTx.Range(func(k, v interface{}) bool {
		txs = append(txs, v.(*types.TxWrap))
		return true
	})
	return txs
}

// GetTxByHash get a transaction by hash from pool or orphan pool
// return a tx and tell whether it is in pool or in orphan pool
func (tx_pool *TransactionPool) GetTxByHash(hash *crypto.HashType) (*types.TxWrap, bool) {

	v, ok := tx_pool.hashToTx.Load(*hash)
	if ok {
		return v.(*types.TxWrap), true
	}
	v, ok = tx_pool.hashToOrphanTx.Load(*hash)
	if ok {
		return v.(*types.TxWrap), false
	}
	return nil, false
}

func lengthOfSyncMap(target *sync.Map) int {
	var length int
	target.Range(func(k, v interface{}) bool {
		length++
		return true
	})
	return length
}

// CheckGasAndNonce checks whether gas price of tx is valid
func (tx_pool *TransactionPool) CheckGasAndNonce(tx *types.Transaction, utxoSet *chain.UtxoSet) error {

	txFee, err := chain.ValidateTxInputs(utxoSet, tx)
	if err != nil {
		return err
	}
	if txlogic.GetTxType(tx, tx_pool.chain.TailState()) != types.ContractTx {
		if txFee != core.TransferFee+tx.ExtraFee() {
			return fmt.Errorf("%s(%d, need %d)", core.ErrInvalidFee, txFee, core.TransferFee+tx.ExtraFee())
		}
		return nil
	}
	// smart contract tx.
	contractVout := txlogic.GetContractVout(tx, tx_pool.chain.TailState())
	if tx.Data != nil && len(tx.Data.Content) > core.MaxCodeSize {
		return core.ErrMaxCodeSizeExceeded
	}
	sc := script.NewScriptFromBytes(contractVout.ScriptPubKey)
	param, ty, err := sc.ParseContractParams()
	if err != nil {
		return err
	}
	if txFee != param.GasLimit*core.FixedGasPrice+tx.ExtraFee() {
		return core.ErrInvalidFee
	}
	contractCreation := ty == types.ContractCreationType
	gas, err := chain.IntrinsicGas(tx.Data.Content, contractCreation)
	if err != nil {
		return err
	}
	if param.GasLimit < gas {
		return vm.ErrOutOfGas
	}
	// check contract tx from
	if addr, err := chain.FetchOutPointOwner(&tx.Vin[0].PrevOutPoint, utxoSet); err != nil ||
		*addr.Hash160() != *param.From {
		return fmt.Errorf("contract tx from address mismatched(%x, %x)", addr.Hash(), param.From[:])
	}
	// check whether contract utxo exists if it is a contract call tx
	// NOTE: here not to consider that a contract deploy tx is in tx pool
	if !contractCreation && !tx_pool.chain.TailState().Exist(*param.To) {
		return fmt.Errorf("contract call error: %s, block height: %d",
			core.ErrContractNotFound, tx_pool.chain.TailBlock().Header.Height)
	}
	// check sender nonce
	nonceOnChain := tx_pool.chain.TailState().GetNonce(*param.From)
	if param.Nonce < nonceOnChain+1 {
		return fmt.Errorf("%s(%d, %d on chain), block height: %d",
			core.ErrNonceTooLow, param.Nonce, nonceOnChain, tx_pool.chain.TailBlock().Header.Height)
	} else if param.Nonce >= nonceOnChain+1 {
		// check whether a tx is in pool that have a bigger nonce and remove it if it exists
		dsOps, dsTxs := tx_pool.getPoolDoubleSpendTxs(tx)
		for i := 0; i < len(dsTxs); i++ {
			if txlogic.GetTxType(dsTxs[i], tx_pool.chain.TailState()) != types.ContractTx {
				continue
			}
			contractVout := txlogic.GetContractVout(tx, tx_pool.chain.TailState())
			sc := script.NewScriptFromBytes(contractVout.ScriptPubKey)
			p, _, err := sc.ParseContractParams()
			if err != nil {
				// contract vout that pay to contract address
				continue
			}
			if param.Nonce < p.Nonce {
				txHash, _ := tx.TxHash()
				dsHash, _ := dsTxs[i].TxHash()
				logger.Warnf("remove tx %s(nonce %d) from pool since tx %s have a "+
					"lower nonce(%d)", dsHash, p.Nonce, txHash, param.Nonce)
				tx_pool.outPointToTx.Delete(*dsOps[i])
				tx_pool.hashToTx.Delete(*dsHash)
			}
		}
	}

	return nil
}
