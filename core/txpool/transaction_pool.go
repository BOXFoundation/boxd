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
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/util"
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
				logger.Errorf("Failed to processTxMsg. Err: %v", err)
			}
		case msg := <-tx_pool.newChainUpdateMsgCh:
			tx_pool.processChainUpdateMsg(msg)
		case <-metricsTicker.C:
			metrics.MetricsTxPoolSizeGauge.Update(int64(lengthOfSyncMap(tx_pool.hashToTx)))
			metrics.MetricsOrphanTxPoolSizeGauge.Update(int64(lengthOfSyncMap(tx_pool.hashToOrphanTx)))
			if time.Now().Unix()%30 == 0 {
				var hashstr string
				tx_pool.hashToTx.Range(func(k, v interface{}) bool {
					txwrap := v.(*chain.TxWrap)
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
		logger.Debugf("Block %v disconnects from main chain", v.BlockHash())
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
				childTx, exists := tx_pool.findTransaction(outPoint)
				if !exists {
					continue
				}
				hash, _ := childTx.TxHash()
				logger.Debugf("Remove related child tx %s when block %v disconnects from main chain", hash, v.BlockHash())
				tx_pool.removeTx(childTx, true)
			}
		}
	}

	for _, v := range msg.AttachBlocks {
		logger.Debugf("Block %v connects to main chain", v.BlockHash())
		tx_pool.removeBlockTxs(v)
	}

}

// Remove all transactions contained in this block and their double spends from main and orphan pool
func (tx_pool *TransactionPool) removeBlockTxs(block *types.Block) error {
	for _, tx := range block.Txs[1:] {
		txHash, _ := tx.TxHash()
		tx_pool.txcache.Add(*txHash, true)
		// Since the passed tx is confirmed in a new block, all its childrent remain valid, thus no recursive removal.
		tx_pool.removeTx(tx, true /* non-recursive */)
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

	if err := tx_pool.ProcessTx(tx, core.RelayMode); err != nil && util.InArray(err, core.EvilBehavior) {
		tx_pool.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.BadTxEvent)
		return err
	}
	tx_pool.chain.Bus().Publish(eventbus.TopicConnEvent, msg.From(), eventbus.NewTxEvent)
	return nil
}

// ProcessTx is used to handle new transactions.
// utxoSet: utxos associated with the tx
func (tx_pool *TransactionPool) ProcessTx(tx *types.Transaction, transferMode core.TransferMode) error {
	if err := tx_pool.maybeAcceptTx(tx, transferMode, true); err != nil {
		txHash, _ := tx.TxHash()
		logger.Errorf("Failed to accept tx. TxHash: %s, Err: %v", txHash, err)
		return err
	}
	return tx_pool.processOrphans(tx)
}

// Potentially accept the transaction to the memory pool.
func (tx_pool *TransactionPool) maybeAcceptTx(tx *types.Transaction,
	transferMode core.TransferMode, detectDupOrphan bool) error {

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

	// TODO: check tx is already exist in the main chain??

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
	if err := tx_pool.checkTransactionStandard(tx); err != nil {
		logger.Errorf("Tx %v is not standard: %v", txHash.String(), err)
		return core.ErrNonStandardTransaction
	}

	if err := tx_pool.checkRegisterOrVoteTx(tx); err != nil {
		logger.Errorf("Tx %v is a invalid Register or Vote tx. Err: %v", txHash.String(), err)
		return err
	}

	// Quickly detects if the tx double spends with any transaction in the pool.
	// Double spending with the main chain txs will be checked in ValidateTxInputs.
	if err := tx_pool.checkPoolDoubleSpend(tx); err != nil {
		logger.Errorf("Tx %v double spends outputs spent by other pending txs: %v", txHash.String(), err)
		return err
	}

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

	// To check script later so main thread is not blocked
	tx_pool.newTxScriptCh <- &txScriptWrap{tx, utxoSet}

	feePerKB := txFee * 1000 / (uint64)(txSize)
	// add transaction to pool.
	tx_pool.addTx(tx, nextBlockHeight, feePerKB)
	// body, _ := conv.MarshalConvertible(tx)
	// key := fmt.Sprintf("%x", (md5.Sum(body)))
	// logger.Infof("Accepted new tx. Hash: %v, transferMode: %v, key: %s", txHash, transferMode, key)
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

func (tx_pool *TransactionPool) findTransaction(outpoint types.OutPoint) (*types.Transaction, bool) {
	if tx, exists := tx_pool.outPointToTx.Load(outpoint); exists {
		return tx.(*types.Transaction), true
	}
	return nil, false
}

func (tx_pool *TransactionPool) isOrphanInPool(txHash *crypto.HashType) bool {
	_, exists := tx_pool.hashToOrphanTx.Load(*txHash)
	return exists
}

func (tx_pool *TransactionPool) checkTransactionStandard(tx *types.Transaction) error {
	// TODO:
	return nil
}

func (tx_pool *TransactionPool) checkRegisterOrVoteTx(tx *types.Transaction) error {
	if tx.Data == nil {
		return nil
	}
	content := tx.Data.Content
	switch int(tx.Data.Type) {
	case types.RegisterCandidateTx:
		registerCandidateContent := new(types.RegisterCandidateContent)
		if err := registerCandidateContent.Unmarshal(content); err != nil {
			return err
		}
		if tx_pool.chain.Consensus().IsCandidateExist(registerCandidateContent.Addr()) {
			return core.ErrCandidateIsAlreadyExist
		}
		if !tx_pool.checkRegisterCandidateOrVoteTx(tx) {
			return core.ErrInvalidRegisterCandidateOrVoteTx
		}
	case types.VoteTx:
		votesContent := new(types.VoteContent)
		if err := votesContent.Unmarshal(content); err != nil {
			return err
		}
		if !tx_pool.chain.Consensus().IsCandidateExist(votesContent.Addr()) {
			return core.ErrCandidateNotFound
		}
		if !tx_pool.checkRegisterCandidateOrVoteTx(tx) {
			return core.ErrInvalidRegisterCandidateOrVoteTx
		}
	}

	return nil
}

func (tx_pool *TransactionPool) checkRegisterCandidateOrVoteTx(tx *types.Transaction) bool {
	for _, vout := range tx.Vout {
		scriptPubKey := script.NewScriptFromBytes(vout.ScriptPubKey)
		if scriptPubKey.IsRegisterCandidateScript(chain.CalcCandidatePledgeHeight(int64(tx_pool.chain.TailBlock().Height))) {
			if tx.Data.Type == types.RegisterCandidateTx {
				if vout.Value >= chain.CandidatePledge {
					return true
				}
			} else if tx.Data.Type == types.VoteTx {
				if vout.Value >= chain.MinNumOfVotes {
					votesContent := new(types.VoteContent)
					if err := votesContent.Unmarshal(tx.Data.Content); err != nil {
						return false
					}
					if votesContent.Votes() == int64(vout.Value) {
						return true
					}
				}
			}
		}
	}
	return false
}

func (tx_pool *TransactionPool) checkPoolDoubleSpend(tx *types.Transaction) error {
	for _, txIn := range tx.Vin {
		if _, exists := tx_pool.findTransaction(txIn.PrevOutPoint); exists {
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
			v, exists := tx_pool.outPointToOrphan.Load(outPoint)
			if !exists {
				continue
			}
			orphans := v.(*sync.Map)
			orphans.Range(func(k, v interface{}) bool {
				orphan := v.(*types.Transaction)
				if err := tx_pool.maybeAcceptTx(orphan, core.DefaultMode, false); err != nil {
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
func (tx_pool *TransactionPool) addTx(tx *types.Transaction, height uint32, feePerKB uint64) {
	txHash, _ := tx.TxHash()

	txWrap := &chain.TxWrap{
		Tx:             tx,
		AddedTimestamp: time.Now().Unix(),
		Height:         height,
		FeePerKB:       feePerKB,
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

	// Unspend the referenced outpoints.
	for _, txIn := range tx.Vin {
		if doubleSpentTx, exists := tx_pool.findTransaction(txIn.PrevOutPoint); exists {
			// tx_pool.removeTx(doubleSpentTx, true /* recursive */)
			doubleSpentTxHash, _ := doubleSpentTx.TxHash()
			tx_pool.hashToTx.Delete(*doubleSpentTxHash)
		}
		tx_pool.outPointToTx.Delete(txIn.PrevOutPoint)
	}
	tx_pool.hashToTx.Delete(*txHash)

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

			childTx, exists := tx_pool.findTransaction(outPoint)
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
		if doubleSpentTx, exists := tx_pool.findTransaction(txIn.PrevOutPoint); exists {
			tx_pool.removeTx(doubleSpentTx, true /* recursive */)
		}
	}
}

// Add orphan
func (tx_pool *TransactionPool) addOrphan(tx *types.Transaction) {
	txWrap := &chain.TxWrap{
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
	if _, err := chain.CheckTxScripts(txScript.utxoSet, txScript.tx, false /* validate script */); err != nil {
		// remove
		tx_pool.removeTx(txScript.tx, true)
		return
	}

	txHash, _ := txScript.tx.TxHash()
	v, exists := tx_pool.hashToTx.Load(*txHash)
	if !exists {
		// already removed
		return
	}
	tx := v.(*chain.TxWrap)
	tx.IsScriptValid = true
}

func (tx_pool *TransactionPool) cleanExpiredTxs() {
	var expiredOrphans []*types.Transaction
	var expiredTxs []*types.Transaction

	now := time.Now()
	tx_pool.hashToOrphanTx.Range(func(k, v interface{}) bool {
		orphan := v.(*chain.TxWrap)
		if now.After(time.Unix(orphan.AddedTimestamp, 0).Add(txTTL)) {
			// Note: do not delete while range looping over map; delete after loop is over
			expiredOrphans = append(expiredOrphans, orphan.Tx)
		}
		return true
	})

	tx_pool.hashToTx.Range(func(k, v interface{}) bool {
		txWrap := v.(*chain.TxWrap)
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
func (tx_pool *TransactionPool) GetAllTxs() []*chain.TxWrap {
	var txs []*chain.TxWrap
	tx_pool.hashToTx.Range(func(k, v interface{}) bool {
		txs = append(txs, v.(*chain.TxWrap))
		return true
	})
	return txs
}

// GetOrphaTxs returns all orpha txs in mempool
func (tx_pool *TransactionPool) GetOrphaTxs() []*chain.TxWrap {
	var txs []*chain.TxWrap
	tx_pool.hashToOrphanTx.Range(func(k, v interface{}) bool {
		txs = append(txs, v.(*chain.TxWrap))
		return true
	})
	return txs
}

// GetTransactionsInPool gets all transactions in memory pool
func (tx_pool *TransactionPool) GetTransactionsInPool() ([]*types.Transaction, []int64) {

	allTxs := tx_pool.GetAllTxs()

	txs := make([]*types.Transaction, 0, len(allTxs))
	addedTimes := make([]int64, 0, len(allTxs))
	for _, tx := range allTxs {
		txs = append(txs, tx.Tx)
		addedTimes = append(addedTimes, tx.AddedTimestamp)
	}
	return txs, addedTimes
}

func calcRequiredMinFee(txSize int) uint64 {
	return 0
}

func lengthOfSyncMap(target *sync.Map) int {
	var length int
	target.Range(func(k, v interface{}) bool {
		length++
		return true
	})
	return length
}
