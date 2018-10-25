// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"container/heap"
	"errors"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txpool"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("dpos") // logger

// Define const
const (
	SecondInMs           = int64(1000)
	NewBlockTimeInterval = int64(5000)
	MaxPackedTxTime      = int64(2000)
	PeriodSize           = 21
)

// Define err message
var (
	ErrNoLegalPowerToMint = errors.New("No legal power to mint")
	ErrNotMyTurnToMint    = errors.New("Not my turn to mint")
	ErrWrongTimeToMint    = errors.New("Wrong time to mint")
	ErrNotFoundMiner      = errors.New("Failed to find miner")
	ErrDuplicateSignUpTx  = errors.New("duplicate sign up tx")
	ErrCandidateNotFound  = errors.New("candidate not found")
)

// Config defines the configurations of dpos
type Config struct {
	// Index  int    `mapstructure:"index"`
	// Pubkey string `mapstructure:"pubkey"`
	keypath    string `mapstructure:"keypath"`
	enableMint bool   `mapstructure:"enable_mint"`
	passphrase string `mapstructure:"passphrase"`
}

// Dpos define dpos struct
type Dpos struct {
	chain      *chain.BlockChain
	txpool     *txpool.TransactionPool
	context    *ConsensusContext
	net        p2p.Net
	proc       goprocess.Process
	cfg        *Config
	miner      *wallet.Account
	enableMint bool
}

// NewDpos new a dpos implement.
func NewDpos(parent goprocess.Process, chain *chain.BlockChain, txpool *txpool.TransactionPool, net p2p.Net, cfg *Config) (*Dpos, error) {
	dpos := &Dpos{
		chain:  chain,
		txpool: txpool,
		net:    net,
		proc:   goprocess.WithParent(parent),
		cfg:    cfg,
	}

	tail := chain.TailBlock()
	context := &ConsensusContext{}
	period, err := dpos.LoadPeriodContext(tail.Header.PeriodHash)
	if err != nil {
		return nil, err
	}
	context.periodContext = period
	dpos.context = context
	return dpos, nil
}

// EnableMint return the peer mint status
func (dpos *Dpos) EnableMint() bool {
	return dpos.cfg.enableMint
}

// Setup setup dpos
func (dpos *Dpos) Setup() error {
	account, err := wallet.NewAccountFromFile(dpos.cfg.keypath)
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
	if !dpos.validateMiner() {
		logger.Warn("You have no authority to mint block")
		return ErrNoLegalPowerToMint
	}
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

func (dpos *Dpos) loop(p goprocess.Process) {
	logger.Info("Start block mint")
	time.Sleep(10 * time.Second)
	timeChan := time.NewTicker(time.Second).C
	for {
		select {
		case <-timeChan:
			dpos.mint()
		case <-p.Closing():
			logger.Info("Stopped Dpos Mining.")
			return
		}
	}
}

func (dpos *Dpos) mint() error {

	timestamp := time.Now().Unix()

	if err := dpos.checkMiner(timestamp); err != nil {
		return err
	}

	logger.Infof("My turn to mint a block, time: %d", timestamp)

	if err := dpos.LoadCandidates(); err != nil {
		return err
	}
	dpos.context.timestamp = timestamp
	dpos.mintBlock()
	return nil
}

func (dpos *Dpos) checkMiner(timestamp int64) error {

	miner, err := dpos.context.periodContext.FindMinerWithTimeStamp(timestamp)
	if err != nil {
		return err
	}
	addr, err := types.ParseAddress(dpos.miner.Addr())
	if err != nil {
		return err
	}
	if *miner != *addr.Hash160() {
		return ErrNotMyTurnToMint
	}
	return nil
}

func (dpos *Dpos) validateMiner() bool {

	addr, err := types.ParseAddress(dpos.miner.Addr())
	if err != nil {
		return false
	}
	if !util.InArray(*addr.Hash160(), dpos.context.periodContext) {
		return false
	}
	if err := dpos.miner.UnlockWithPassphrase(dpos.cfg.passphrase); err != nil {
		return false
	}
	return true
}

func (dpos *Dpos) mintBlock() {
	// tail, _ := dpos.chain.LoadTailBlock()
	tail := dpos.chain.TailBlock()
	block := types.NewBlock(tail)
	block.Header.TimeStamp = dpos.context.timestamp
	if block.Height > 0 && block.Height%chain.PeriodDuration == 0 {
		// TODO: period changed
	} else {
		block.Header.PeriodHash = tail.Header.PeriodHash
	}
	dpos.PackTxs(block, nil)
	if err := dpos.signBlock(block); err != nil {
		return
	}
	dpos.chain.ProcessBlock(block, true)
}

func lessFunc(queue *util.PriorityQueue, i, j int) bool {

	txi := queue.Items(i).(*txpool.TxWrap)
	txj := queue.Items(j).(*txpool.TxWrap)
	if txi.FeePerKB == txj.FeePerKB {
		return txi.AddedTimestamp < txj.AddedTimestamp
	}
	return txi.FeePerKB < txj.FeePerKB
}

// sort pending transactions in mempool
func (dpos *Dpos) sortPendingTxs() *util.PriorityQueue {
	pool := util.NewPriorityQueue(lessFunc)
	pendingTxs := dpos.txpool.GetAllTxs()
	for _, pendingTx := range pendingTxs {
		// place onto heap sorted by FeePerKB
		heap.Push(pool, pendingTx)
	}
	return pool
}

// PackTxs packed txs and add them to block.
func (dpos *Dpos) PackTxs(block *types.Block, addr types.Address) error {

	pool := dpos.sortPendingTxs()
	var blockTxns []*types.Transaction
	coinbaseTx, err := chain.CreateCoinbaseTx(addr, dpos.chain.LongestChainHeight+1)
	if err != nil || coinbaseTx == nil {
		logger.Error("Failed to create coinbaseTx")
		return errors.New("Failed to create coinbaseTx")
	}
	blockTxns = append(blockTxns, coinbaseTx)

	remainTimeInMs := dpos.context.timestamp + MaxPackedTxTime - time.Now().Unix()*SecondInMs
	remainTimer := time.NewTimer(time.Duration(remainTimeInMs) * time.Millisecond)

PackingTxs:
	for {
		select {
		case <-remainTimer.C:
			break PackingTxs
		default:
			for pool.Len() > 0 {
				txWrap := heap.Pop(pool).(*txpool.TxWrap)
				if err := dpos.prepareCandidateContext(txWrap.Tx); err != nil {
					// TODO: abandon the error tx
					continue
				}
				blockTxns = append(blockTxns, txWrap.Tx)
			}
		}
	}
	candidateHash, err := dpos.context.candidateContext.CandidateContextHash()
	if err != nil {
		return err
	}
	block.Header.CandidatesHash = *candidateHash
	merkles := chain.CalcTxsHash(blockTxns)
	block.Header.TxsRoot = *merkles
	block.Txs = blockTxns
	return nil
}

// LoadPeriodContext load period context
func (dpos *Dpos) LoadPeriodContext(hash crypto.HashType) (*PeriodContext, error) {

	db := dpos.chain.GetChainDB()
	period, err := db.Get(hash[:])
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

// StorePeriodContext store period context
func (dpos *Dpos) StorePeriodContext() error {

	db := dpos.chain.GetChainDB()
	context, err := dpos.context.periodContext.Marshal()
	if err != nil {
		return err
	}
	hash := crypto.DoubleHashH(context)
	return db.Put(hash[:], context)
}

// LoadCandidates load candidates info.
func (dpos *Dpos) LoadCandidates() error {

	tail := dpos.chain.TailBlock()
	db := dpos.chain.GetChainDB()

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
func (dpos *Dpos) StoreCandidateContext(hash crypto.HashType) error {

	bytes, err := dpos.context.candidateContext.Marshal()
	if err != nil {
		return err
	}
	db := dpos.chain.GetChainDB()
	return db.Put(hash[:], bytes)
}

// prepareCandidateContext prepare to update CandidateContext.
func (dpos *Dpos) prepareCandidateContext(tx *types.Transaction) error {

	content := tx.Data.Content
	candidateContext := dpos.context.candidateContext
	switch int(tx.Data.Type) {
	case types.SignUpTx:
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
	case types.VotesTx:
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
	signature, err := crypto.Sign(dpos.miner.PrivateKey(), hash)
	if err != nil {
		return err
	}
	block.Header.Signature = signature.Serialize()
	return nil
}

// VerifySign consensus verify sign info.
func (dpos *Dpos) VerifySign(block *types.Block) (bool, error) {

	miner, err := dpos.context.periodContext.FindMinerWithTimeStamp(block.Header.TimeStamp)
	if err != nil {
		return false, err
	}
	if miner == nil {
		return false, ErrNotFoundMiner
	}

	signature, err := crypto.SigFromBytes(block.Header.Signature)
	if err != nil {
		return false, err
	}

	pubkey, ok := signature.Recover(block.BlockHash()[:])
	if ok {
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
