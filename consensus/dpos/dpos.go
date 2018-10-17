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
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("dpos") // logger

// Define err message
var (
	ErrNoLegalPowerToMint = errors.New("No legal power to mint")
)

// Config defines the configurations of dpos
type Config struct {
	// Index  int    `mapstructure:"index"`
	// Pubkey string `mapstructure:"pubkey"`
	Keypath    string `mapstructure:"keypath"`
	EnableMint bool   `mapstructure:"enable_mint"`
}

// Dpos define dpos struct
type Dpos struct {
	chain  *chain.BlockChain
	txpool *txpool.TransactionPool
	net    p2p.Net
	proc   goprocess.Process
	cfg    *Config
	miner  *wallet.Account
	// miner      types.Address
	enableMint bool
}

// NewDpos new a dpos implement.
func NewDpos(chain *chain.BlockChain, txpool *txpool.TransactionPool, net p2p.Net, parent goprocess.Process, cfg *Config) *Dpos {

	dpos := &Dpos{
		chain:  chain,
		txpool: txpool,
		net:    net,
		proc:   goprocess.WithParent(parent),
		cfg:    cfg,
	}

	// pubkey, err := hex.DecodeString(dpos.cfg.Pubkey)
	// if err != nil {
	// 	panic("invalid hex in source file: " + dpos.cfg.Pubkey)
	// }
	// addr, err := types.NewAddressPubKeyHash(pubkey)
	// if err != nil {
	// 	panic("invalid public key in test source")
	// }
	// logger.Info("miner addr: ", addr.String())
	// dpos.miner = addr
	return dpos
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

// Run start dpos
func (dpos *Dpos) Run() {
	logger.Info("Dpos run")
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
	now := time.Now().Unix()
	// if int(now%15) != dpos.cfg.Index {
	// 	return ErrNoLegalPowerToMint
	// }
	if !dpos.enableMint {
		return ErrNoLegalPowerToMint
	}

	logger.Infof("My turn to mint a block, time: %d", now)
	dpos.mintBlock()
	return nil
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

	merkles := util.CalcTxsHash(blockTxns)
	block.Header.TxsRoot = *merkles
	block.Txs = blockTxns
	return nil
}
