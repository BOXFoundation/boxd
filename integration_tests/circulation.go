// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"time"

	"github.com/BOXFoundation/boxd/wallet"
)

// Circulation manage circulation of transaction
type Circulation struct {
	accCnt     int
	addrs      []string
	accAddrs   []string
	collAddrCh chan<- string
	cirAddrCh  <-chan string
	quitCh     chan os.Signal
}

// NewCirculation construct a Circulation instance
func NewCirculation(accCnt int, collAddrCh chan<- string,
	cirAddrCh <-chan string) *Circulation {
	c := &Circulation{}
	// get account address
	c.accCnt = accCnt
	logger.Infof("start to gen %d address for circulation", accCnt)
	c.addrs, c.accAddrs = genTestAddr(c.accCnt)
	logger.Debugf("addrs: %v\ntestsAcc: %v", c.addrs, c.accAddrs)
	// get accounts for addrs
	logger.Infof("start to unlock all %d tests accounts", len(c.addrs))
	for _, addr := range c.addrs {
		acc := unlockAccount(addr)
		AddrToAcc[addr] = acc
	}
	c.collAddrCh = collAddrCh
	c.cirAddrCh = cirAddrCh
	c.quitCh = make(chan os.Signal, 1)
	signal.Notify(c.quitCh, os.Interrupt, os.Kill)
	return c
}

// TearDown clean test accounts files
func (c *Circulation) TearDown() {
	removeKeystoreFiles(c.accAddrs...)
}

// Run consumes transaction pending on circulation channel
func (c *Circulation) Run() {
	defer func() {
		if x := recover(); x != nil {
			logger.Error(x)
		}
	}()
	addrIdx := 0
	for {
		select {
		case s := <-c.quitCh:
			logger.Infof("receive quit signal %v, quiting collection!", s)
			close(c.collAddrCh)
			return
		default:
			logger.Info("start create transactions...")
		}
		idx := addrIdx % len(c.addrs)
		c.collAddrCh <- c.addrs[idx]
		toIdx := (addrIdx + 1) % len(c.addrs)
		toAddr := c.addrs[toIdx]
		logger.Info("start box circulation between accounts")
		txRepeatTest(<-c.cirAddrCh, toAddr, peersAddr[0])
		addrIdx++
	}
}

func txRepeatTest(fromAddr, toAddr string, execPeer string) {
	defer func() {
		if x := recover(); x != nil {
			logger.Error(x)
		}
	}()
	logger.Info("=== RUN   txRepeatTest")
	// create a transaction from addr 1 to addr 2
	transfer := uint64(0)
	utxos, err := utxosNoPanicFor(fromAddr, execPeer)
	if err != nil {
		logger.Warn(err)
	}
	if len(utxos) == 0 {
		return
	}
	times := len(utxos) - 1
	if len(utxos) <= times {
		logger.Infof("utxos count %d is less or equals than repeat times %d on %s,"+
			" set times--", len(utxos), times, fromAddr)
		times = len(utxos) - 1
	}
	sort.Sort(sort.Reverse(sortByUTXOValue(utxos)))
	toUtxos := utxosFor(toAddr, execPeer)
	//
	fromBalancePre := balanceFor(fromAddr, execPeer)
	toBalancePre := balanceFor(toAddr, execPeer)
	logger.Infof("fromAddr[%s] balance: %d, utox count: %d, max: %d, min: %d",
		fromAddr, fromBalancePre, len(utxos), utxos[0].TxOut.Value,
		utxos[len(utxos)-1].TxOut.Value)
	logger.Infof("toAddr[%s] balance: %d", toAddr, toBalancePre)
	for i := 0; i < times; i++ {
		//amount := utxos[i+1].TxOut.Value
		//amount := utxos[len(utxos)-1].TxOut.Value
		var amount uint64 = 1000
		logger.Debugf("sent %d from %s to %s on peer %s", amount, fromAddr, toAddr, execPeer)
		execTx(AddrToAcc[fromAddr], []string{toAddr}, []uint64{amount}, execPeer)
		transfer += amount
	}

	logger.Infof("wait for transaction brought on chain, timeout %v", timeoutToChain)
	m, err := waitUTXOsEnough(toAddr, len(toUtxos)+times, execPeer, timeoutToChain)
	time.Sleep(blockTime)
	if err != nil {
		logger.Warn(err)
	}
	logger.Infof("addr %s utxo count: %d", toAddr, m)

	// check the balance of miners
	logger.Infof("start to get balance of fromAddr[%s], toAddr[%s] from %s",
		fromAddr, toAddr, execPeer)
	fromBalancePost := balanceFor(fromAddr, execPeer)
	toBalancePost := balanceFor(toAddr, execPeer)
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePost, toAddr, toBalancePost)

	// prerequisite: neither of fromAddr and toAddr are not miner address
	toGap := toBalancePost - toBalancePre
	fromGap := fromBalancePre - fromBalancePost
	if toGap > fromGap || toGap != transfer {
		logger.Errorf("txRepeatTest faild: fromGap %d toGap %d and transfer %d",
			fromGap, toGap, transfer)
	}
	logger.Infof("--- DONE: txRepeatTest")
}

func txOneToManyTest(fromAddr string, toAddrs []string, totalAmount uint64,
	execPeer string) {
	logger.Info("=== RUN   txOneToManyTest")
	// get balance of fromAddr and toAddrs
	logger.Infof("start to get balance of fromAddr[%s], toAddrs[%v] from %s",
		fromAddr, toAddrs, execPeer)
	fromBalancePre := balanceFor(fromAddr, execPeer)
	toBalancesPre := make([]uint64, len(toAddrs))
	for i := 0; i < len(toAddrs); i++ {
		b := balanceFor(toAddrs[i], execPeer)
		toBalancesPre[i] = b
	}
	logger.Infof("fromAddr[%s] balance: %d toAddrs[%v] balance: %v",
		fromAddr, fromBalancePre, toAddrs, toBalancesPre)

	// create a transaction from test account 1 to test accounts and execute it
	acc := unlockAccount(fromAddr)
	ave := totalAmount / uint64(len(toAddrs))
	transfer := uint64(0)
	for i := 0; i < len(toAddrs); i++ {
		amount := ave/2 + uint64(rand.Int63n(int64(ave)/2))
		execTx(acc, []string{toAddrs[i]}, []uint64{amount}, execPeer)
		transfer += amount
		logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddr,
			toAddrs[i], execPeer)
	}
	logger.Infof("wait for transaction brought on chain, timeout %v", timeoutToChain)
	if _, err := waitBalanceEnough(toAddrs[len(toAddrs)-1], 100000, execPeer,
		timeoutToChain); err != nil {
		logger.Panic(err)
	}
	time.Sleep(blockTime)
	// get balance of fromAddr and toAddrs
	logger.Infof("start to get balance of fromAddr[%s], toAddrs[%v] from %s",
		fromAddr, toAddrs, execPeer)
	fromBalancePost := balanceFor(fromAddr, execPeer)
	toBalancesPost := make([]uint64, len(toAddrs))
	for i := 0; i < len(toAddrs); i++ {
		b := balanceFor(toAddrs[i], execPeer)
		toBalancesPost[i] = b
	}
	logger.Infof("fromAddr[%s] balance: %d toAddrs[%v] balance: %v",
		fromAddr, fromBalancePost, toAddrs, toBalancesPost)
	//
	fromGap := fromBalancePre - fromBalancePost
	toGap := uint64(0)
	for i := 0; i < len(toAddrs); i++ {
		toGap += toBalancesPost[i] - toBalancesPre[i]
	}
	if toGap > fromGap || toGap != transfer {
		logger.Panicf("txOneToManyTest faild: fromGap %d toGap %d and transfer %d",
			fromGap, toGap, transfer)
	}
	logger.Info("--- PASS: txOneToManyTest")
}

func txManyToOneTest(fromAddrs []string, toAddr string, execPeer string) {
	logger.Info("=== RUN   txManyToOneTest")
	// get balance of fromAddrs and toAddr
	logger.Infof("start to get balance of fromAddrs[%v], toAddr[%s] from %s",
		fromAddrs, toAddr, execPeer)
	fromBalancesPre := make([]uint64, len(fromAddrs))
	for i := 0; i < len(fromAddrs); i++ {
		b := balanceFor(fromAddrs[i], execPeer)
		fromBalancesPre[i] = b
	}
	toBalancePre := balanceFor(toAddr, execPeer)
	logger.Debugf("fromAddrs[%v] balance: %v toAddr[%s] balance: %d",
		fromAddrs, fromBalancesPre, toAddr, toBalancePre)

	// create a transaction from test accounts to account and execute it
	accounts := make([]*wallet.Account, len(fromAddrs))
	for i := 0; i < len(fromAddrs); i++ {
		acc := unlockAccount(fromAddrs[i])
		accounts[i] = acc
	}
	transfer := uint64(0)
	for i := 0; i < len(fromAddrs); i++ {
		amount := fromBalancesPre[i] / 2
		execTx(accounts[i], []string{toAddr}, []uint64{amount}, execPeer)
		transfer += amount
		logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddrs[i],
			toAddr, execPeer)
	}
	logger.Infof("wait for transaction brought on chain, timeout %v", timeoutToChain)
	if _, err := waitBalanceEnough(toAddr, 1000, execPeer, timeoutToChain); err != nil {
		logger.Panic(err)
	}
	time.Sleep(blockTime)
	// get balance of fromAddrs and toAddr
	logger.Infof("start to get balance of fromAddrs[%v], toAddr[%s] from %s",
		fromAddrs, toAddr, execPeer)
	fromBalancesPost := make([]uint64, len(fromAddrs))
	for i := 0; i < len(fromAddrs); i++ {
		b := balanceFor(fromAddrs[i], execPeer)
		fromBalancesPost[i] = b
	}
	toBalancePost := balanceFor(toAddr, execPeer)
	logger.Debugf("fromAddrs[%v] balance: %v toAddr[%s] balance: %d",
		fromAddrs, fromBalancesPost, toAddr, toBalancePost)

	//
	fromGap := uint64(0)
	for i := 0; i < len(fromAddrs); i++ {
		fromGap += fromBalancesPre[i] - fromBalancesPost[i]
	}
	toGap := toBalancePost - toBalancePre
	if fromGap < toGap || toGap != transfer {
		logger.Panicf("txManyToOneTest faild: fromGap %d toGap %d and transfer %d",
			fromGap, toGap, transfer)
	}
	logger.Info("--- PASS: txManyToOneTest")
}
