// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"container/heap"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txpool"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
	acc "github.com/BOXFoundation/boxd/wallet/account"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("dpos") // logger

// Define const
const (
	SecondInMs           = int64(1000)
	MinerRefreshInterval = int64(5000)
	MaxPackedTxTime      = int64(100)
	PeriodSize           = 6
	BlockNumPerPeiod     = 5
)

// Config defines the configurations of dpos
type Config struct {
	Keypath    string `mapstructure:"keypath"`
	EnableMint bool   `mapstructure:"enable_mint"`
	Passphrase string `mapstructure:"passphrase"`
}

// Dpos define dpos struct
type Dpos struct {
	chain                       *chain.BlockChain
	txpool                      *txpool.TransactionPool
	context                     *ConsensusContext
	net                         p2p.Net
	proc                        goprocess.Process
	cfg                         *Config
	miner                       *acc.Account
	canMint                     bool
	disableMint                 bool
	bftservice                  *BftService
	blockHashToCandidateContext *lru.Cache
}

// NewDpos new a dpos implement.
func NewDpos(parent goprocess.Process, chain *chain.BlockChain, txpool *txpool.TransactionPool, net p2p.Net, cfg *Config) (*Dpos, error) {
	dpos := &Dpos{
		chain:   chain,
		txpool:  txpool,
		net:     net,
		proc:    goprocess.WithParent(parent),
		cfg:     cfg,
		canMint: false,
	}
	dpos.blockHashToCandidateContext, _ = lru.New(512)
	context := &ConsensusContext{}
	dpos.context = context
	period, err := dpos.LoadPeriodContext()
	if err != nil {
		return nil, err
	}
	context.periodContext = period
	if err := dpos.LoadCandidates(); err != nil {
		return nil, err
	}

	return dpos, nil
}

// EnableMint return the peer mint status
func (dpos *Dpos) EnableMint() bool {
	return dpos.cfg.EnableMint
}

// Setup setup dpos
func (dpos *Dpos) Setup() error {
	account, err := acc.NewAccountFromFile(dpos.cfg.Keypath)
	if err != nil {
		return err
	}
	dpos.miner = account

	return nil
}

// implement interface service.Server
var _ service.Server = (*Dpos)(nil)

// Run start dpos
func (dpos *Dpos) Run() error {
	logger.Info("Dpos run")
	if !dpos.ValidateMiner() {
		logger.Warn("You have no authority to mint block")
		return ErrNoLegalPowerToMint
	}

	// peer can mint, start bftService.
	bftService, err := NewBftService(dpos)
	if err != nil {
		return err
	}
	dpos.bftservice = bftService
	dpos.subscribe()
	bftService.Start()
	dpos.proc.Go(dpos.loop)

	return nil
}

// Proc returns the goprocess running the service
func (dpos *Dpos) Proc() goprocess.Process {
	return dpos.proc
}

// Stop dpos
func (dpos *Dpos) Stop() {
	dpos.proc.Close()
}

// StopMint stops generating blocks.
func (dpos *Dpos) StopMint() {
	dpos.disableMint = true
}

// RecoverMint resumes generating blocks.
func (dpos *Dpos) RecoverMint() {
	dpos.disableMint = false
}

func (dpos *Dpos) loop(p goprocess.Process) {
	logger.Info("Start block mint loop")
	timeChan := time.NewTicker(time.Second)
	defer timeChan.Stop()
	for {
		select {
		case <-timeChan.C:
			if !dpos.chain.IsBusy() {
				dpos.mint(time.Now().Unix())
			}

		case <-p.Closing():
			logger.Info("Stopped Dpos Mining.")
			return
		}
	}
}

func (dpos *Dpos) mint(timestamp int64) error {

	// disableMint might be set true by sync business or others
	if dpos.disableMint {
		return ErrNoLegalPowerToMint
	}

	if err := dpos.checkMiner(timestamp); err != nil {
		return err
	}
	dpos.context.timestamp = timestamp
	MetricsMintTurnCounter.Inc(1)

	logger.Infof("My turn to mint a block, time: %d", timestamp)
	return dpos.mintBlock()
}

// checkMiner check to verify if miner can mint at the timestamp
func (dpos *Dpos) checkMiner(timestamp int64) error {

	miner, err := dpos.context.periodContext.FindMinerWithTimeStamp(timestamp)
	if err != nil {
		return err
	}
	addr, err := types.NewAddress(dpos.miner.Addr())
	if err != nil {
		return err
	}
	if *miner != *addr.Hash160() {
		return ErrNotMyTurnToMint
	}
	return nil
}

// ValidateMiner verifies whether the miner has authority to mint.
func (dpos *Dpos) ValidateMiner() bool {

	if dpos.miner == nil {
		return false
	}

	if dpos.canMint {
		return true
	}

	addr, err := types.NewAddress(dpos.miner.Addr())
	if err != nil {
		return false
	}
	if !util.InArray(*addr.Hash160(), dpos.context.periodContext.periodAddrs) {
		return false
	}
	if err := dpos.miner.UnlockWithPassphrase(dpos.cfg.Passphrase); err != nil {
		logger.Error(err)
		return false
	}
	dpos.canMint = true
	return true
}

func (dpos *Dpos) mintBlock() error {

	tail := dpos.chain.TailBlock()
	block := types.NewBlock(tail)
	block.Header.TimeStamp = dpos.context.timestamp
	if block.Height > 0 && block.Height%chain.PeriodDuration == 0 {
		// TODO: period changed
	} else {
		block.Header.PeriodHash = tail.Header.PeriodHash
	}
	if err := dpos.PackTxs(block, dpos.miner.PubKeyHash()); err != nil {
		logger.Warnf("Failed to pack txs. err: %s", err.Error())
		return err
	}
	if err := dpos.signBlock(block); err != nil {
		logger.Warnf("Failed to sign block. err: %s", err.Error())
		return err
	}

	go func() {
		if err := dpos.chain.ProcessBlock(block, core.BroadcastMode, true, ""); err != nil {
			logger.Warnf("Failed to process block mint by self. err: %s", err.Error())
		}
	}()

	return nil
}

func lessFunc(queue *util.PriorityQueue, i, j int) bool {

	txi := queue.Items(i).(*chain.TxWrap)
	txj := queue.Items(j).(*chain.TxWrap)
	if txi.FeePerKB == txj.FeePerKB {
		return txi.AddedTimestamp < txj.AddedTimestamp
	}
	return txi.FeePerKB < txj.FeePerKB
}

// getChainedTxs returns all chained ancestor txs in mempool of the passed tx, including itself
// From child to ancestors
func getChainedTxs(tx *chain.TxWrap, hashToTx map[crypto.HashType]*chain.TxWrap) []*chain.TxWrap {
	hashSet := make(map[crypto.HashType]struct{})
	chainedTxs := []*chain.TxWrap{tx}

	// Note: use index here instead of range because chainedTxs can be extended inside the loop
	for i := 0; i < len(chainedTxs); i++ {
		tx := chainedTxs[i].Tx

		for _, txIn := range tx.Vin {
			prevTxHash := txIn.PrevOutPoint.Hash
			if prevTx, exists := hashToTx[prevTxHash]; exists {
				if _, exists := hashSet[prevTxHash]; !exists {
					chainedTxs = append(chainedTxs, prevTx)
					hashSet[prevTxHash] = struct{}{}
				}
			}
		}
	}

	return chainedTxs
}

// sort pending transactions in mempool
func (dpos *Dpos) sortPendingTxs() ([]*chain.TxWrap, map[crypto.HashType]*chain.TxWrap) {
	pool := util.NewPriorityQueue(lessFunc)
	pendingTxs := dpos.txpool.GetAllTxs()
	for _, pendingTx := range pendingTxs {
		// place onto heap sorted by FeePerKB
		// only pack txs whose scripts have been verified
		if pendingTx.IsScriptValid {
			heap.Push(pool, pendingTx)
		}
	}

	var sortedTxs []*chain.TxWrap
	hashToTx := make(map[crypto.HashType]*chain.TxWrap)
	for pool.Len() > 0 {
		txWrap := heap.Pop(pool).(*chain.TxWrap)
		sortedTxs = append(sortedTxs, txWrap)
		txHash, _ := txWrap.Tx.TxHash()
		hashToTx[*txHash] = txWrap
	}
	return sortedTxs, hashToTx
}

// PackTxs packed txs and add them to block.
func (dpos *Dpos) PackTxs(block *types.Block, scriptAddr []byte) error {

	// We sort txs in mempool by fees when packing while ensuring child tx is not packed before parent tx.
	// otherwise the former's utxo is missing
	sortedTxs, hashToTx := dpos.sortPendingTxs()
	candidateContext, err := dpos.LoadCandidateByBlockHash(&block.Header.PrevBlockHash)
	if err != nil {
		logger.Error("Failed to load candidate context")
		return err
	}

	var blockTxns []*types.Transaction
	coinbaseTx, err := chain.CreateCoinbaseTx(scriptAddr, dpos.chain.LongestChainHeight+1)
	if err != nil || coinbaseTx == nil {
		return errors.New("Failed to create coinbaseTx")
	}
	blockTxns = append(blockTxns, coinbaseTx)
	remainTimeInMs := dpos.context.timestamp*SecondInMs + MaxPackedTxTime - time.Now().Unix()*SecondInMs
	spendableTxs := new(sync.Map)

	// Total fees of all packed txs
	totalTxFee := uint64(0)
	stopPack := false
	stopPackCh := make(chan bool, 1)
	continueCh := make(chan bool, 1)

	go func() {
		for txIdx, tx := range sortedTxs {
			if stopPack {
				continueCh <- true
				logger.Debugf("stops at %d-th tx: packed %d txs out of %d", txIdx, len(blockTxns)-1, len(sortedTxs))
				return
			}

			// logger.Debugf("Iterating over %d-th tx: packed %d txs out of %d so far", txIdx, len(blockTxns)-1, len(sortedTxs))
			chainedTxs := getChainedTxs(tx, hashToTx)
			// Add ancestors first
			for i := len(chainedTxs) - 1; i >= 0; i-- {
				txWrap := chainedTxs[i]
				txHash, _ := txWrap.Tx.TxHash()
				// Already packed
				if _, exists := spendableTxs.Load(*txHash); exists {
					continue
				}

				if err := dpos.prepareCandidateContext(candidateContext, txWrap.Tx); err != nil {
					// TODO: abandon the error tx
					continue
				}

				utxoSet, err := chain.GetExtendedTxUtxoSet(txWrap.Tx, dpos.chain.DB(), spendableTxs)
				if err != nil {
					logger.Warnf("Could not get extended utxo set for tx %v", txHash)
					continue
				}

				totalInputAmount := utxoSet.TxInputAmount(txWrap.Tx)
				if totalInputAmount == 0 {
					// This can only occur when a tx's parent is removed from mempool but not written to utxo db yet
					logger.Errorf("This can not occur totalInputAmount == 0, tx hash: %v", txHash)
					continue
				}
				totalOutputAmount := txWrap.Tx.OutputAmount()
				if totalInputAmount < totalOutputAmount {
					// This must not happen since the tx already passed the check when admitted into mempool
					logger.Warnf("total value of all transaction outputs for "+
						"transaction %v is %v, which exceeds the input amount "+
						"of %v", txHash, totalOutputAmount, totalInputAmount)
					// TODO: abandon the error tx from pool.
					continue
				}
				txFee := totalInputAmount - totalOutputAmount
				totalTxFee += txFee

				spendableTxs.Store(*txHash, txWrap)
				blockTxns = append(blockTxns, txWrap.Tx)
			}
		}
		continueCh <- true
		stopPackCh <- true
	}()

	select {
	case <-time.After(time.Duration(remainTimeInMs) * time.Millisecond):
		logger.Debug("Packing timeout")
		stopPack = true
	case <-stopPackCh:
		logger.Debug("Packing completed")
	}

	// Important: wait for packing complete and exit
	<-continueCh

	// Pay tx fees to miner in addition to block reward in coinbase
	blockTxns[0].Vout[0].Value += totalTxFee

	candidateHash, err := candidateContext.CandidateContextHash()
	if err != nil {
		return err
	}
	block.Header.CandidatesHash = *candidateHash
	merkles := chain.CalcTxsHash(blockTxns)
	block.Header.TxsRoot = *merkles
	block.Txs = blockTxns
	block.IrreversibleInfo = dpos.bftservice.FetchIrreversibleInfo()
	logger.Infof("Finish packing txs. Hash: %v, Height: %d, Block TxsNum: %d, Mempool TxsNum: %d", block.BlockHash(), block.Height, len(blockTxns), len(sortedTxs))
	return nil
}

// LoadPeriodContext load period context
func (dpos *Dpos) LoadPeriodContext() (*PeriodContext, error) {

	db := dpos.chain.DB()
	period, err := db.Get(chain.PeriodKey)
	if err != nil {
		return nil, err
	}
	if period != nil {
		periodContext := new(PeriodContext)
		if err := periodContext.Unmarshal(period); err != nil {
			return nil, err
		}
		return periodContext, nil
	}
	periodContext, err := InitPeriodContext()
	if err != nil {
		return nil, err
	}
	dpos.context.periodContext = periodContext
	if err := dpos.StorePeriodContext(); err != nil {
		return nil, err
	}
	return periodContext, nil
}

// BroadcastEternalMsgToMiners broadcast eternal message to miners
func (dpos *Dpos) BroadcastEternalMsgToMiners(block *types.Block) error {

	eternalBlockMsg := &EternalBlockMsg{}
	hash := block.BlockHash()
	signature, err := crypto.SignCompact(dpos.miner.PrivateKey(), hash[:])
	if err != nil {
		return err
	}
	eternalBlockMsg.Hash = *hash
	eternalBlockMsg.Signature = signature
	eternalBlockMsg.Timestamp = block.Header.TimeStamp
	miners := dpos.context.periodContext.periodPeers

	return dpos.net.BroadcastToMiners(p2p.EternalBlockMsg, eternalBlockMsg, miners)
}

// StorePeriodContext store period context
func (dpos *Dpos) StorePeriodContext() error {

	db := dpos.chain.DB()
	context, err := dpos.context.periodContext.Marshal()
	if err != nil {
		return err
	}
	return db.Put(chain.PeriodKey, context)
}

// LoadCandidates load candidates info.
func (dpos *Dpos) LoadCandidates() error {

	tail := dpos.chain.TailBlock()
	db := dpos.chain.DB()

	candidates, err := db.Get(tail.Header.CandidatesHash[:])
	if err != nil {
		return err
	}
	if candidates != nil {
		candidatesContext := new(CandidateContext)
		if err := candidatesContext.Unmarshal(candidates); err != nil {
			return err
		}
		dpos.context.candidateContext = candidatesContext
		return nil
	}

	candidatesContext := InitCandidateContext()
	dpos.context.candidateContext = candidatesContext
	return nil
}

// UpdateCandidateContext update candidate context in memory.
func (dpos *Dpos) UpdateCandidateContext(block *types.Block) error {
	candidateContext, err := dpos.LoadCandidateByBlockHash(block.BlockHash())
	if err != nil {
		return err
	}
	dpos.context.candidateContext = candidateContext
	return nil
}

// LoadCandidateByBlockHash load candidate by block hash
func (dpos *Dpos) LoadCandidateByBlockHash(hash *crypto.HashType) (*CandidateContext, error) {

	if v, ok := dpos.blockHashToCandidateContext.Get(*hash); ok {
		return v.(*CandidateContext), nil
	}
	candidateContextBin, err := dpos.chain.DB().Get(chain.CandidatesKey(hash))
	if err != nil {
		return nil, err
	}
	candidateContext := new(CandidateContext)
	if err := candidateContext.Unmarshal(candidateContextBin); err != nil {
		return nil, err
	}
	return candidateContext, nil
}

// StoreCandidateContext store candidate context
// The cache is not used here to avoid problems caused by revert block.
// So when block revert occurs, here we don't have to do revert.
func (dpos *Dpos) StoreCandidateContext(block *types.Block, batch storage.Batch) error {

	parentBlock := dpos.chain.GetParentBlock(block)
	candidateContext, err := dpos.LoadCandidateByBlockHash(parentBlock.BlockHash())
	if err != nil {
		return err
	}
	for _, tx := range block.Txs {
		if err := dpos.prepareCandidateContext(candidateContext, tx); err != nil {
			return err
		}
	}
	bytes, err := candidateContext.Marshal()
	if err != nil {
		return err
	}
	batch.Put(chain.CandidatesKey(block.BlockHash()), bytes)
	dpos.blockHashToCandidateContext.Add(*block.BlockHash(), candidateContext)
	return nil
}

// IsCandidateExist check candidate is exist.
func (dpos *Dpos) IsCandidateExist(addr types.AddressHash) bool {

	for _, v := range dpos.context.candidateContext.addrs {
		if v == addr {
			return true
		}
	}
	return false
}

// VerifyCandidates vefiry if the block candidates hash is right.
func (dpos *Dpos) VerifyCandidates(block *types.Block) error {

	candidateContext := dpos.context.candidateContext.Copy()
	for _, tx := range block.Txs {
		if err := dpos.prepareCandidateContext(candidateContext, tx); err != nil {
			return err
		}
	}
	candidateHash, err := candidateContext.CandidateContextHash()
	if err != nil {
		return err
	}
	if !candidateHash.IsEqual(&block.Header.CandidatesHash) {
		return ErrInvalidCandidateHash
	}

	return nil
}

// prepareCandidateContext prepare to update CandidateContext.
func (dpos *Dpos) prepareCandidateContext(candidateContext *CandidateContext, tx *types.Transaction) error {

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
		candidate := &Candidate{
			addr:  registerCandidateContent.Addr(),
			votes: 0,
		}
		candidateContext.candidates = append(candidateContext.candidates, candidate)
	case types.VoteTx:
		votesContent := new(types.VoteContent)
		if err := votesContent.Unmarshal(content); err != nil {
			return err
		}
		for _, v := range candidateContext.candidates {
			if v.addr == votesContent.Addr() {
				atomic.AddInt64(&v.votes, votesContent.Votes())
			}
		}
	default:
	}
	return nil
}

func (dpos *Dpos) signBlock(block *types.Block) error {

	hash := block.BlockHash()
	signature, err := crypto.SignCompact(dpos.miner.PrivateKey(), hash[:])
	if err != nil {
		return err
	}
	block.Signature = signature
	return nil
}

// VerifyMinerEpoch verifies miner epoch.
func (dpos *Dpos) VerifyMinerEpoch(block *types.Block) error {

	tail := dpos.chain.TailBlock()
	miner, err := dpos.context.periodContext.FindMinerWithTimeStamp(block.Header.TimeStamp)
	if err != nil {
		return err
	}

	for idx := 0; idx < 2*PeriodSize/3; {
		height := tail.Height - uint32(idx)
		if height == 0 {
			break
		}
		block, err := dpos.chain.LoadBlockByHeight(height)
		if err != nil {
			return err
		}
		target, err := dpos.context.periodContext.FindMinerWithTimeStamp(block.Header.TimeStamp)
		if err != nil {
			return err
		}
		if target == miner {
			return ErrInvalidMinerEpoch
		}
		idx++
	}
	return nil
}

// VerifySign consensus verifies signature info.
func (dpos *Dpos) VerifySign(block *types.Block) (bool, error) {

	miner, err := dpos.context.periodContext.FindMinerWithTimeStamp(block.Header.TimeStamp)
	if err != nil {
		return false, err
	}
	if miner == nil {
		return false, ErrNotFoundMiner
	}

	if pubkey, ok := crypto.RecoverCompact(block.BlockHash()[:], block.Signature); ok {
		addr, err := types.NewAddressFromPubKey(pubkey)
		if err != nil {
			return false, err
		}
		if *addr.Hash160() == *miner {
			return true, nil
		}
	}

	return false, nil
}

// TryToUpdateEternalBlock try to update eternal block.
func (dpos *Dpos) TryToUpdateEternalBlock(src *types.Block) {
	irreversibleInfo := src.IrreversibleInfo
	if irreversibleInfo != nil && len(irreversibleInfo.Signatures) > MinConfirmMsgNumberForEternalBlock {
		block, err := dpos.chain.LoadBlockByHash(irreversibleInfo.Hash)
		if err != nil {
			logger.Warnf("Failed to update eternal block. Err: %s", err.Error())
			return
		}
		dpos.bftservice.updateEternal(block)
	}
}

func (dpos *Dpos) subscribe() {
	dpos.chain.Bus().Reply(eventbus.TopicMiners, func(out chan<- []string) {
		out <- dpos.context.periodContext.periodPeers
	}, false)
	dpos.chain.Bus().Reply(eventbus.TopicCheckMiner, func(timestamp int64, out chan<- error) {
		out <- dpos.checkMiner(timestamp)
	}, false)
}
