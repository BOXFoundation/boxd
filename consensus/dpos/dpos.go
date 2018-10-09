// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"container/heap"
	"encoding/hex"
	"errors"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txpool"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/core/utils"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/util"
	"github.com/jbenet/goprocess"
)

var (
	logger = log.NewLogger("dpos") // logger
)

func init() {
}

// Config defines the configurations of dpos
type Config struct {
	Index  int    `mapstructure:"index"`
	Pubkey string `mapstructure:"pubkey"`
}

// Dpos define dpos struct
type Dpos struct {
	chain  *chain.BlockChain
	txpool *txpool.TransactionPool
	net    p2p.Net
	proc   goprocess.Process
	cfg    *Config
	miner  types.Address
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

	pubkey, err := hex.DecodeString(dpos.cfg.Pubkey)
	if err != nil {
		panic("invalid hex in source file: " + dpos.cfg.Pubkey)
	}
	addr, err := types.NewAddressPubKeyHash(pubkey, 0x00)
	if err != nil {
		panic("invalid public key in test source")
	}
	logger.Info("miner addr: ", addr.String())
	dpos.miner = addr
	return dpos
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

func (dpos *Dpos) mint() {
	now := time.Now().Unix()
	if int(now%15) != dpos.cfg.Index {
		return
	}

	logger.Infof("My turn to mint a block, time: %d", now)
	dpos.mintBlock()
}

func (dpos *Dpos) mintBlock() {
	tail, _ := dpos.chain.LoadTailBlock()
	block := types.NewBlock(tail)
	dpos.PackTxs(block, dpos.miner)
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
	coinbaseTx, err := dpos.createCoinbaseTx(addr)
	if err != nil || coinbaseTx == nil {
		logger.Error("Failed to create coinbaseTx")
		return errors.New("Failed to create coinbaseTx")
	}
	blockTxns = append(blockTxns, coinbaseTx)
	for pool.Len() > 0 {
		txwrap := heap.Pop(pool).(*txpool.TxWrap)
		tx := txwrap.Tx
		// unspentUtxoCache, err := chain.LoadUnspentUtxo(tx)
		// if err != nil {
		// 	continue
		// }
		// mergeUtxoCache(blockUtxos, unspentUtxoCache)
		// spent tx
		// chain.spendTransaction(blockUtxos, tx, chain.tail.Height)
		blockTxns = append(blockTxns, tx)
	}

	merkles := util.CalcTxsHash(blockTxns)
	block.Header.TxsRoot = *merkles
	for _, tx := range blockTxns {
		block.Txs = append(block.Txs, tx)
	}
	return nil
}

func (dpos *Dpos) createCoinbaseTx(addr types.Address) (*types.Transaction, error) {

	var pkScript []byte
	var err error
	nextBlockHeight := dpos.chain.LongestChainHeight + 1
	blockReward := utils.CalcBlockSubsidy(nextBlockHeight)
	coinbaseScript, err := script.StandardCoinbaseScript(nextBlockHeight)
	if err != nil {
		return nil, err
	}
	if addr != nil {
		pkScript, err = script.PayToPubKeyHashScript(addr.ScriptAddress())
		if err != nil {
			return nil, err
		}
	} else {
		scriptBuilder := script.NewBuilder()
		pkScript, err = scriptBuilder.AddOp(script.OPTRUE).Script()
		if err != nil {
			return nil, err
		}
	}

	tx := &types.Transaction{
		Version: 1,
		Vin: []*types.TxIn{
			{
				PrevOutPoint: types.OutPoint{
					Hash:  crypto.HashType{},
					Index: 0xffffffff,
				},
				ScriptSig: coinbaseScript,
				Sequence:  0xffffffff,
			},
		},
		Vout: []*types.TxOut{
			{
				Value:        blockReward,
				ScriptPubKey: pkScript,
			},
		},
	}
	return tx, nil
}
