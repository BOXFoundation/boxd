// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"container/heap"
	"errors"
	"time"

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
	NewBlockTimeInterval = int64(5)
	PeriodSize           = 21
)

// Define err message
var (
	ErrNoLegalPowerToMint = errors.New("No legal power to mint")
	ErrNotMyTurnToMint    = errors.New("Not my turn to mint")
	ErrWrongTimeToMint    = errors.New("Wrong time to mint")
	ErrNotFoundMiner      = errors.New("Failed to find miner")
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
func NewDpos(chain *chain.BlockChain, txpool *txpool.TransactionPool, net p2p.Net, parent goprocess.Process, cfg *Config) (*Dpos, error) {

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

// Run start dpos
func (dpos *Dpos) Run() {
	logger.Info("Dpos run")
	if !dpos.validateMiner() {
		logger.Warn("You have no authority to mint block")
		return
	}
	go dpos.loop()
}

// Stop dpos
func (dpos *Dpos) Stop() {

}

func (dpos *Dpos) loop() {
	logger.Info("Start block mint")
	time.Sleep(10 * time.Second)
	timeChan := time.NewTicker(time.Second).C
	for {
		select {
		case <-timeChan:
			dpos.mint()
		case <-dpos.proc.Closing():
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
	tail, _ := dpos.chain.LoadTailBlock()
	block := types.NewBlock(tail)
	dpos.PackTxs(block, nil)
	// block.setMiner()
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

	// TODO: @Leon Each time you packtxs, a new queue is generated.
	pool := dpos.sortPendingTxs()
	// blockUtxos := NewUtxoUnspentCache()
	var blockTxns []*types.Transaction
	coinbaseTx, err := chain.CreateCoinbaseTx(addr, dpos.chain.LongestChainHeight+1)
	if err != nil || coinbaseTx == nil {
		logger.Error("Failed to create coinbaseTx")
		return errors.New("Failed to create coinbaseTx")
	}
	blockTxns = append(blockTxns, coinbaseTx)
	for pool.Len() > 0 {
		txWrap := heap.Pop(pool).(*txpool.TxWrap)
		blockTxns = append(blockTxns, txWrap.Tx)
	}

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
		dpos.context.candidateContext = candidatesContext
		return nil
	}

	candidatesContext := InitCandidateContext()
	dpos.context.candidateContext = candidatesContext
	return nil
}
