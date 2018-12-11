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

	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txpool"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("dpos") // logger

// Define const
const (
	SecondInMs           = int64(1000)
	MinerRefreshInterval = int64(5000)
	MaxPackedTxTime      = int64(200)
	MaxBlockTimeOut      = 2
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
	chain       *chain.BlockChain
	txpool      *txpool.TransactionPool
	context     *ConsensusContext
	net         p2p.Net
	proc        goprocess.Process
	cfg         *Config
	miner       *wallet.Account
	canMint     bool
	disableMint bool
	bftservice  *BftService
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

	context := &ConsensusContext{}
	dpos.context = context
	period, err := dpos.LoadPeriodContext()
	if err != nil {
		return nil, err
	}
	context.periodContext = period

	return dpos, nil
}

// EnableMint return the peer mint status
func (dpos *Dpos) EnableMint() bool {
	return dpos.cfg.EnableMint
}

// Setup setup dpos
func (dpos *Dpos) Setup() error {
	account, err := wallet.NewAccountFromFile(dpos.cfg.Keypath)
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
	logger.Info("Start block mint")
	timeChan := time.NewTicker(time.Second)
	defer timeChan.Stop()
	for {
		select {
		case <-timeChan.C:
			dpos.mint(time.Now().Unix())
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
	MetricsMintTurnCounter.Inc(1)
	logger.Infof("My turn to mint a block, time: %d", timestamp)
	if err := dpos.LoadCandidates(); err != nil {
		return err
	}
	dpos.context.timestamp = timestamp

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
	if err := dpos.chain.ProcessBlock(block, p2p.BroadcastMode, true, ""); err != nil {
		logger.Warnf("Failed to process block. err: %s", err.Error())
		return err
	}
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

// sort pending transactions in mempool
func (dpos *Dpos) sortPendingTxs() []*chain.TxWrap {
	pool := util.NewPriorityQueue(lessFunc)
	pendingTxs := dpos.txpool.GetAllTxs()
	for _, pendingTx := range pendingTxs {
		// place onto heap sorted by FeePerKB
		heap.Push(pool, pendingTx)
	}

	var sortedTxs []*chain.TxWrap
	for pool.Len() > 0 {
		txWrap := heap.Pop(pool).(*chain.TxWrap)
		sortedTxs = append(sortedTxs, txWrap)
	}
	return sortedTxs
}

// PackTxs packed txs and add them to block.
func (dpos *Dpos) PackTxs(block *types.Block, scriptAddr []byte) error {

	// We sort txs in mempool by fees when packing while ensuring child tx is not packed before parent tx.
	// otherwise the former's utxo is missing
	sortedTxs := dpos.sortPendingTxs()
	// if i-th sortedTxs is packed into the block
	txPacked := make([]bool, len(sortedTxs))

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
	stopPackCh := make(chan bool)

	go func() {
		found := true
		for found {
			found = false
			for i, txWrap := range sortedTxs {
				if stopPack {
					return
				}
				if txPacked[i] {
					continue
				}

				if err := dpos.prepareCandidateContext(txWrap.Tx); err != nil {
					// TODO: abandon the error tx
					continue
				}

				txHash, _ := txWrap.Tx.TxHash()
				utxoSet, err := chain.GetExtendedTxUtxoSet(txWrap.Tx, dpos.chain.DB(), spendableTxs)
				if err != nil {
					logger.Warnf("Could not get extended utxo set for tx %v", txHash)
					continue
				}

				// This is to ensure within mempool, a parent tx is packed before its children txs
				totalInputAmount := utxoSet.TxInputAmount(txWrap.Tx)
				if totalInputAmount == 0 {
					// This can only occur for a mempool tx if its parent txs (also in mempool) are not packed yet
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
				txPacked[i] = true
				found = true
			}
		}
		stopPackCh <- true
	}()

	select {
	case <-time.After(time.Duration(remainTimeInMs) * time.Millisecond):
		stopPack = true
	case <-stopPackCh:
	}

	// Pay tx fees to miner in addition to block reward in coinbase
	blockTxns[0].Vout[0].Value += totalTxFee

	candidateHash, err := dpos.context.candidateContext.CandidateContextHash()
	if err != nil {
		return err
	}
	block.Header.CandidatesHash = *candidateHash
	merkles := chain.CalcTxsHash(blockTxns)
	block.Header.TxsRoot = *merkles
	block.Txs = blockTxns
	if block.IrreversibleInfo, err = dpos.bftservice.FetchIrreversibleInfo(); err != nil {
		return err
	}
	logger.Infof("Finish packing txs. Hash: %v, Height: %d, TxsNum: %d", block.BlockHash().String(), block.Height, len(blockTxns))
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
	eternalBlockMsg.hash = *hash
	eternalBlockMsg.signature = signature
	eternalBlockMsg.timestamp = block.Header.TimeStamp
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
		candidatesContext.height = tail.Height + 1
		dpos.context.candidateContext = candidatesContext
		return nil
	}

	candidatesContext := InitCandidateContext()
	dpos.context.candidateContext = candidatesContext
	return nil
}

// StoreCandidateContext store candidate context
func (dpos *Dpos) StoreCandidateContext(hash *crypto.HashType, batch storage.Batch) error {
	if dpos.context.candidateContext == nil {
		if err := dpos.LoadCandidates(); err != nil {
			return err
		}
	}
	bytes, err := dpos.context.candidateContext.Marshal()
	if err != nil {
		return err
	}
	batch.Put(chain.CandidatesKey(hash), bytes)
	return nil
}

// prepareCandidateContext prepare to update CandidateContext.
func (dpos *Dpos) prepareCandidateContext(tx *types.Transaction) error {

	if tx.Data == nil {
		return nil
	}
	content := tx.Data.Content
	candidateContext := dpos.context.candidateContext
	switch int(tx.Data.Type) {
	case types.RegisterCandidateTx:
		signUpContent := new(types.SignUpContent)
		if err := signUpContent.Unmarshal(content); err != nil {
			return err
		}
		if util.InArray(signUpContent.Addr(), candidateContext.addrs) {
			return ErrDuplicateSignUpTx
		}
		candidate := &Candidate{
			addr:  signUpContent.Addr(),
			votes: 0,
		}
		candidateContext.candidates = append(candidateContext.candidates, candidate)
	case types.VoteTx:
		votesContent := new(types.VoteContent)
		if err := votesContent.Unmarshal(content); err != nil {
			return err
		}
		if !util.InArray(votesContent.Addr(), dpos.context.candidateContext.addrs) {
			return ErrCandidateNotFound
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
	if irreversibleInfo == nil {
		return
	}
	block, _ := dpos.chain.LoadBlockByHash(irreversibleInfo.Hash)
	if block != nil {
		dpos.bftservice.updateEternal(block)
	}
}
