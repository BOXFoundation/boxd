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
	"github.com/BOXFoundation/boxd/core/state"
	"github.com/BOXFoundation/boxd/core/txpool"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
	acc "github.com/BOXFoundation/boxd/wallet/account"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("dpos") // logger

// Define const
const (
	SecondInMs                = int64(1000)
	BookkeeperRefreshInterval = int64(5000)
	MaxPackedTxTime           = int64(100)
	PeriodSize                = 6
	BlockNumPerPeiod          = 5
	PeriodDuration            = 21 * 5 * 10000

	// CandidatePledge is pledge for candidate to mint.
	CandidatePledge = (uint64)(1e6 * core.DuPerBox)
	// MinNumOfVotes is Minimum number of votes
	MinNumOfVotes = (uint64)(100)
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
	bookkeeper                  *acc.Account
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
	dpos.bookkeeper = account

	return nil
}

// implement interface service.Server
var _ service.Server = (*Dpos)(nil)

// Run start dpos
func (dpos *Dpos) Run() error {
	logger.Info("Dpos run")
	if !dpos.IsBookkeeper() {
		logger.Warn("You have no authority to produce block")
		return ErrNoLegalPowerToProduce
	}

	// peer is bookkeeper, start bftService.
	bftService, err := NewBftService(dpos)
	if err != nil {
		return err
	}
	dpos.bftservice = bftService
	dpos.subscribe()
	bftService.Run()
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

// StopMint stops producing blocks.
func (dpos *Dpos) StopMint() {
	dpos.disableMint = true
}

// RecoverMint resumes producing blocks.
func (dpos *Dpos) RecoverMint() {
	dpos.disableMint = false
}

// Verify check the legality of the block.
func (dpos *Dpos) Verify(block *types.Block) error {
	ok, err := dpos.verifySign(block)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("Failed to verify sign block")
	}

	if err := dpos.verifyCandidates(block); err != nil {
		return err
	}

	return dpos.verifyIrreversibleInfo(block)
}

// Finalize notify consensus to change new tail.
func (dpos *Dpos) Finalize(tail *types.Block) error {
	if err := dpos.UpdateCandidateContext(tail); err != nil {
		return err
	}
	if dpos.IsBookkeeper() && time.Now().Unix()-tail.Header.TimeStamp < MaxEternalBlockMsgCacheTime {
		go dpos.BroadcastBFTMsgToBookkeepers(tail, p2p.BlockPrepareMsg)
	}
	go dpos.TryToUpdateEternalBlock(tail)
	return nil
}

// Process notify consensus to process new block.
func (dpos *Dpos) Process(block *types.Block, db interface{}) error {
	return dpos.StoreCandidateContext(block, db.(storage.Table))
}

// VerifyTx notify consensus to verify new tx.
func (dpos *Dpos) VerifyTx(tx *types.Transaction) error {
	return dpos.checkRegisterOrVoteTx(tx)
}

func (dpos *Dpos) loop(p goprocess.Process) {
	logger.Info("Start dpos loop")
	timeChan := time.NewTicker(time.Second)
	defer timeChan.Stop()
	for {
		select {
		case <-timeChan.C:
			if !dpos.chain.IsBusy() {
				dpos.run(time.Now().Unix())
			}

		case <-p.Closing():
			logger.Info("Stopped Dpos Mining.")
			return
		}
	}
}

func (dpos *Dpos) run(timestamp int64) error {

	// disableMint might be set true by sync business or others
	if dpos.disableMint {
		return ErrNoLegalPowerToProduce
	}

	if err := dpos.verifyBookkeeper(timestamp); err != nil {
		return err
	}
	dpos.context.timestamp = timestamp
	MetricsMintTurnCounter.Inc(1)

	logger.Infof("My turn to produce a block, time: %d", timestamp)
	return dpos.produceBlock()
}

// verifyProposer check to verify if bookkeeper can mint at the timestamp
func (dpos *Dpos) verifyBookkeeper(timestamp int64) error {

	bookkeeper, err := dpos.context.periodContext.FindProposerWithTimeStamp(timestamp)
	if err != nil {
		return err
	}
	addr, err := types.NewAddress(dpos.bookkeeper.Addr())
	if err != nil {
		return err
	}
	if *bookkeeper != *addr.Hash160() {
		return ErrNotMyTurnToProduce
	}
	return nil
}

// IsBookkeeper verifies whether the peer has authority to produce block.
func (dpos *Dpos) IsBookkeeper() bool {

	if dpos.bookkeeper == nil {
		return false
	}

	if dpos.canMint {
		return true
	}

	addr, err := types.NewAddress(dpos.bookkeeper.Addr())
	if err != nil {
		return false
	}
	if !util.InArray(*addr.Hash160(), dpos.context.periodContext.periodAddrs) {
		return false
	}
	if err := dpos.bookkeeper.UnlockWithPassphrase(dpos.cfg.Passphrase); err != nil {
		logger.Error(err)
		return false
	}
	dpos.canMint = true
	return true
}

func (dpos *Dpos) produceBlock() error {

	tail := dpos.chain.TailBlock()
	block := types.NewBlock(tail)
	block.Header.TimeStamp = dpos.context.timestamp
	if block.Header.Height > 0 && block.Header.Height%chain.PeriodDuration == 0 {
		// TODO: period changed
	} else {
		block.Header.PeriodHash = tail.Header.PeriodHash
	}
	if err := dpos.PackTxs(block, dpos.bookkeeper.PubKeyHash()); err != nil {
		logger.Warnf("Failed to pack txs. err: %s", err.Error())
		return err
	}
	if err := dpos.signBlock(block); err != nil {
		logger.Warnf("Failed to sign block. err: %s", err.Error())
		return err
	}

	go func() {
		dpos.chain.BroadcastOrRelayBlock(block, core.BroadcastMode)
		if err := dpos.chain.ProcessBlock(block, core.DefaultMode, ""); err != nil {
			logger.Warnf("Failed to process block mint by self. err: %s", err.Error())
		}
	}()

	return nil
}

func lessFunc(queue *util.PriorityQueue, i, j int) bool {

	txi := queue.Items(i).(*types.TxWrap)
	txj := queue.Items(j).(*types.TxWrap)
	if txi.GasPrice == txj.GasPrice {
		return txi.AddedTimestamp < txj.AddedTimestamp
	}
	return txi.GasPrice < txj.GasPrice
}

// getChainedTxs returns all chained ancestor txs in mempool of the passed tx, including itself
// From child to ancestors
func getChainedTxs(tx *types.TxWrap, hashToTx map[crypto.HashType]*types.TxWrap) []*types.TxWrap {
	hashSet := make(map[crypto.HashType]struct{})
	chainedTxs := []*types.TxWrap{tx}

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
func (dpos *Dpos) sortPendingTxs() ([]*types.TxWrap, map[crypto.HashType]*types.TxWrap) {
	pool := util.NewPriorityQueue(lessFunc)
	pendingTxs := dpos.txpool.GetAllTxs()
	for _, pendingTx := range pendingTxs {
		// place onto heap sorted by gasPrice
		// only pack txs whose scripts have been verified
		if pendingTx.IsScriptValid {
			heap.Push(pool, pendingTx)
		}
	}

	var sortedTxs []*types.TxWrap
	hashToTx := make(map[crypto.HashType]*types.TxWrap)
	for pool.Len() > 0 {
		txWrap := heap.Pop(pool).(*types.TxWrap)
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

	// Pay tx fees to bookkeeper in addition to block reward in coinbase
	blockTxns[0].Vout[0].Value += totalTxFee
	block.Txs = blockTxns

	candidateHash, err := candidateContext.CandidateContextHash()
	if err != nil {
		return err
	}

	parentHash := block.Header.PrevBlockHash
	parent, err := dpos.chain.LoadBlockByHash(parentHash)
	if err != nil {
		return err
	}
	statedb, err := state.New(&parent.Header.RootHash, &parent.Header.UtxoRoot, dpos.chain.DB())
	if err != nil {
		return err
	}
	utxoSet := chain.NewUtxoSet()
	if err := utxoSet.LoadBlockUtxos(block, dpos.chain.DB()); err != nil {
		return err
	}
	blockCopy := block.Copy()
	dpos.chain.SplitBlockOutputs(blockCopy)
	if err := utxoSet.ApplyBlock(blockCopy, statedb, dpos.chain.DB()); err != nil {
		return err
	}

	gasUsed, gasRemainingFee, utxoTxs, err := dpos.chain.StateProcessor().Process(block, statedb, utxoSet)
	if err != nil {
		return err
	}
	// apply internal txs.
	if len(block.InternalTxs) > 0 {
		if err := utxoSet.ApplyInternalTxs(block, statedb, dpos.chain.DB()); err != nil {
			return err
		}
	}
	if err := dpos.chain.UpdateUtxoState(statedb, utxoSet); err != nil {
		return err
	}
	dpos.chain.UpdateNormalTxBalanceState(utxoSet, statedb)
	root, utxoRoot, err := statedb.Commit(false)
	if err != nil {
		return err
	}

	dpos.chain.StateDBCache()[block.Header.Height] = statedb
	dpos.chain.UtxoSetCache()[block.Header.Height] = utxoSet

	block.Txs[0].Vout[0].Value -= gasRemainingFee
	// handle coinbase utxo
	for _, v := range utxoSet.GetUtxos() {
		if v.IsCoinBase() {
			v.SetValue(block.Txs[0].Vout[0].Value)
		}
	}
	block.Header.CandidatesHash = *candidateHash
	block.Header.GasUsed = gasUsed
	block.Header.RootHash = *root
	txsRoot := chain.CalcTxsHash(block.Txs)
	block.Header.TxsRoot = *txsRoot
	// block.Txs = blockTxns
	if len(utxoTxs) > 0 {
		internalTxsRoot := chain.CalcTxsHash(utxoTxs)
		block.Header.InternalTxsRoot = *internalTxsRoot
		block.InternalTxs = utxoTxs
		block.Header.UtxoRoot = *utxoRoot
	}

	block.IrreversibleInfo = dpos.bftservice.FetchIrreversibleInfo()
	logger.Infof("Finish packing txs. Hash: %v, Height: %d, Block TxsNum: %d, Mempool TxsNum: %d", block.BlockHash(), block.Header.Height, len(block.Txs), len(sortedTxs))
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

// BroadcastBFTMsgToBookkeepers broadcast block BFT message to bookkeepers
func (dpos *Dpos) BroadcastBFTMsgToBookkeepers(block *types.Block, messageID uint32) error {

	prepareBlockMsg := &EternalBlockMsg{}
	hash := block.BlockHash()
	signature, err := crypto.SignCompact(dpos.bookkeeper.PrivateKey(), hash[:])
	if err != nil {
		return err
	}
	prepareBlockMsg.Hash = *hash
	prepareBlockMsg.Signature = signature
	prepareBlockMsg.Timestamp = block.Header.TimeStamp
	bookkeepers := dpos.context.periodContext.periodPeers

	return dpos.net.BroadcastToBookkeepers(messageID, prepareBlockMsg, bookkeepers)
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
func (dpos *Dpos) StoreCandidateContext(block *types.Block, db storage.Table) error {

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
	db.Put(chain.CandidatesKey(block.BlockHash()), bytes)
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

// verifyCandidates vefiry if the block candidates hash is right.
func (dpos *Dpos) verifyCandidates(block *types.Block) error {

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

// verifyIrreversibleInfo vefiry if the block irreversibleInfo is right.
func (dpos *Dpos) verifyIrreversibleInfo(block *types.Block) error {

	irreversibleInfo := block.IrreversibleInfo
	if irreversibleInfo != nil {
		if len(irreversibleInfo.Signatures) <= MinConfirmMsgNumberForEternalBlock {
			return errors.New("the number of irreversibleInfo signatures is not enough")
		}
		// check hash is exist
		block, _ := dpos.chain.LoadBlockByHash(irreversibleInfo.Hash)
		if block == nil {
			logger.Errorf("Invalid irreversible info. The block hash %s is not exist.", irreversibleInfo.Hash.String())
			return errors.New("the block hash is not exist on the chain")
		}
		//TODO: period switching requires extra processing
		addrs := dpos.context.periodContext.periodAddrs
		remains := []types.AddressHash{}
		for _, v := range irreversibleInfo.Signatures {
			if pubkey, ok := crypto.RecoverCompact(irreversibleInfo.Hash[:], v); ok {
				addrPubKeyHash, err := types.NewAddressFromPubKey(pubkey)
				if err != nil {
					return err
				}
				addr := *addrPubKeyHash.Hash160()
				if util.InArray(addr, addrs) {
					if !util.InArray(addr, remains) {
						remains = append(remains, addr)
					} else {
						logger.Errorf("Duplicated irreversible signature %v in block. Hash: %s, Height: %d",
							v, block.BlockHash().String(), block.Header.Height)
						return errors.New("Duplicated irreversible signature in block")
					}
				} else {
					logger.Errorf("Invalid irreversible signature %v in block. Hash: %s, Height: %d",
						v, block.BlockHash().String(), block.Header.Height)
					return errors.New("Invalid irreversible signature in block")
				}
			} else {
				return errors.New("Invalid irreversible signature in block")
			}
		}
		if len(remains) <= MinConfirmMsgNumberForEternalBlock {
			logger.Errorf("Invalid irreversible info in block. Hash: %s, Height: %d, remains: %d", block.BlockHash().String(), block.Header.Height, len(remains))
			return errors.New("Invalid irreversible info in block")
		}
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
	signature, err := crypto.SignCompact(dpos.bookkeeper.PrivateKey(), hash[:])
	if err != nil {
		return err
	}
	block.Signature = signature
	return nil
}

// verifies bookkeeper epoch.
func (dpos *Dpos) verifyBookkeeperEpoch(block *types.Block) error {

	tail := dpos.chain.TailBlock()
	bookkeeper, err := dpos.context.periodContext.FindProposerWithTimeStamp(block.Header.TimeStamp)
	if err != nil {
		return err
	}

	for idx := 0; idx < 2*PeriodSize/3; {
		height := tail.Header.Height - uint32(idx)
		if height == 0 {
			break
		}
		block, err := dpos.chain.LoadBlockByHeight(height)
		if err != nil {
			return err
		}
		target, err := dpos.context.periodContext.FindProposerWithTimeStamp(block.Header.TimeStamp)
		if err != nil {
			return err
		}
		if target == bookkeeper {
			return ErrInvalidBookkeeperEpoch
		}
		idx++
	}
	return nil
}

// verifySign consensus verifies signature info.
func (dpos *Dpos) verifySign(block *types.Block) (bool, error) {

	bookkeeper, err := dpos.context.periodContext.FindProposerWithTimeStamp(block.Header.TimeStamp)
	if err != nil {
		return false, err
	}
	if bookkeeper == nil {
		return false, ErrNotFoundBookkeeper
	}

	if pubkey, ok := crypto.RecoverCompact(block.BlockHash()[:], block.Signature); ok {
		addr, err := types.NewAddressFromPubKey(pubkey)
		if err != nil {
			return false, err
		}
		if *addr.Hash160() == *bookkeeper {
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
		out <- dpos.verifyBookkeeper(timestamp)
	}, false)
}

func (dpos *Dpos) checkRegisterOrVoteTx(tx *types.Transaction) error {
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
		if dpos.IsCandidateExist(registerCandidateContent.Addr()) {
			return ErrCandidateIsAlreadyExist
		}
		if !dpos.checkRegisterCandidateOrVoteTx(tx) {
			return ErrInvalidRegisterCandidateOrVoteTx
		}
	case types.VoteTx:
		votesContent := new(types.VoteContent)
		if err := votesContent.Unmarshal(content); err != nil {
			return err
		}
		if !dpos.IsCandidateExist(votesContent.Addr()) {
			return ErrCandidateNotFound
		}
		if !dpos.checkRegisterCandidateOrVoteTx(tx) {
			return ErrInvalidRegisterCandidateOrVoteTx
		}
	}

	return nil
}

func (dpos *Dpos) checkRegisterCandidateOrVoteTx(tx *types.Transaction) bool {
	for _, vout := range tx.Vout {
		scriptPubKey := script.NewScriptFromBytes(vout.ScriptPubKey)
		if scriptPubKey.IsRegisterCandidateScript(calcCandidatePledgeHeight(int64(dpos.chain.TailBlock().Header.Height))) {
			if tx.Data.Type == types.RegisterCandidateTx {
				if vout.Value >= CandidatePledge {
					return true
				}
			} else if tx.Data.Type == types.VoteTx {
				if vout.Value >= MinNumOfVotes {
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

// calcCandidatePledgeHeight calc current candidate pledge height
func calcCandidatePledgeHeight(tailHeight int64) int64 {
	return (tailHeight/PeriodDuration + 2) * PeriodDuration
}
