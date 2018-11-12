// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/wallet"
)

const (
	timeoutToChain = 20
)

type txTest struct {
	minersAddr []string
	testsAddr  []string
	repeat     int
}

func newTxTest(minersAddr, testsAddr []string, count int) *txTest {
	return &txTest{
		minersAddr: minersAddr,
		testsAddr:  testsAddr,
		repeat:     count,
	}
}

func (tt *txTest) testTx() {
	// wait all peers' heights are same
	logger.Info("waiting for all the peers' heights are the same")
	height, err := waitHeightSame()
	if err != nil {
		logger.Panic(err)
	}
	logger.Infof("now the height of all peers is %d", height)

	utxoCnt := 100
	// prepare utxo for testsAddr[0]
	tt.prepareUTXOs(tt.testsAddr[0], utxoCnt, peersAddr[0])

	amount := balanceFor(tt.testsAddr[0], peersAddr[0])
	logger.Infof("balance for addr %s is %d", tt.testsAddr[0], amount)

	// test repeast tx test addr 0 to test addr 1
	txRepeatTest(tt.testsAddr[0], tt.testsAddr[1], peersAddr[0], 100)

	//// test tx between account A to many other accounts
	//txOneToManyTest(tt.testsAddr[1], tt.testsAddr[2:3], amount/2, peersAddr[1],
	//	peersAddr[0])

	//// test tx between accounts to a single account
	//// transfer half of balance from testsAddr[2:] to testsAddr[1]
	//txManyToOneTest(tt.testsAddr[2:3], tt.testsAddr[1], peersAddr[1], peersAddr[0])
}

// prepareUTXOs generates n*n utxos for addr
func (tt *txTest) prepareUTXOs(addr string, n int, peerAddr string) {
	logger.Info("=== RUN   prepareUTXOs")
	count := int(math.Ceil(math.Sqrt(float64(n))))
	if count > len(tt.testsAddr) {
		logger.Panicf("tests account is not enough for generate %d utxo", n)
	}
	if !util.InArray(addr, tt.testsAddr) {
		logger.Panicf("argument addr %s must be in tests accounts", addr)
	}
	// miner 0 to tests[0:len(testsAddr)-1]
	acc := unlockAccount(tt.minersAddr[0])
	amount := chain.BaseSubsidy / (10 * uint64(count))
	amount += uint64(rand.Int63n(int64(amount)))
	amounts := make([]uint64, count)
	for i := 0; i < count; i++ {
		amounts[i] = amount/2 + uint64(rand.Int63n(int64(amount)/2))
	}
	minerBalancePre := balanceFor(tt.minersAddr[0], peerAddr)
	if minerBalancePre < amount*uint64(count) {
		logger.Panicf("balance of miner[0](%d) is less than %d",
			minerBalancePre, amount*uint64(count))
	}
	logger.Debugf("start to sent %v from addr %s to testsAddr on peer %s",
		amounts, tt.minersAddr[0], peerAddr)
	execTx(acc, tt.minersAddr[0], tt.testsAddr[:count], amounts, peerAddr)
	logger.Infof("wait for all test addresses have utxos more than %d", 1)
	for _, addr := range tt.testsAddr[:count] {
		_, err := waitUTXOsEnough(addr, 1, peerAddr, timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}
	}
	for i, addr := range tt.testsAddr {
		b := balanceFor(addr, peerAddr)
		if b != amounts[i] {
			logger.Panicf("balance of testsAddr[%d] is %d, that is not equal to "+
				"%d transfered", i, b, amounts[i])
		}
	}
	// get accounts for testsAddr
	logger.Infof("start to unlock all %d tests accounts", count)
	accounts := make([]*wallet.Account, count)
	for i := 0; i < count; i++ {
		accounts[i] = unlockAccount(tt.testsAddr[i])
	}
	// gen peersCnt*peerCnt utxos via sending from each to each
	allAmounts := make([][]uint64, count)
	var wg sync.WaitGroup
	errChans := make(chan error, count)
	logger.Infof("start to send tx from each to each")
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer func() {
				wg.Done()
				if x := recover(); x != nil {
					errChans <- fmt.Errorf("%v", x)
				}
			}()
			amounts2 := make([]uint64, count)
			for j := 0; j < count; j++ {
				base := amounts[i] / uint64(count) / 4
				amounts2[j] = base + uint64(rand.Int63n(int64(base)))
			}
			allAmounts[i] = amounts2
			logger.Debugf("start to sent %v from addr %d to testsAddr on peer %s",
				amounts2, i, peerAddr)
			execTx(accounts[i], tt.testsAddr[i], tt.testsAddr[:count], amounts2, peerAddr)
			logger.Debugf("wait for addr %s utxo more than %d, timeout %ds",
				tt.testsAddr[i], count, timeoutToChain)
			time.Sleep(blockTime)
			_, err := waitUTXOsEnough(tt.testsAddr[i], count, peerAddr, timeoutToChain)
			if err != nil {
				errChans <- err
			}
		}(i)
	}
	wg.Wait()
	if len(errChans) > 0 {
		logger.Panic(<-errChans)
	}
	// gather count*count utxo via transfering from others to the first one
	errChans = make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer func() {
				wg.Done()
				if x := recover(); x != nil {
					errChans <- fmt.Errorf("%v", x)
				}
			}()
			if tt.testsAddr[i] == addr {
				return
			}
			logger.Debugf("start to gather utxo from addr %d to addr %s on peer %s",
				i, addr, peerAddr)
			for j := 0; j < count; j++ {
				amount := allAmounts[j][i] / 2
				execTx(accounts[i], tt.testsAddr[i], []string{addr}, []uint64{amount},
					peerAddr)
				logger.Debugf("have sent %d from %s to %s", amount, tt.testsAddr[i], addr)
			}
		}(i)
	}
	wg.Wait()
	if len(errChans) > 0 {
		logger.Panic(<-errChans)
	}
	logger.Infof("wait utxo count reach %d on %s, timeout %ds", n, addr, timeoutToChain)
	m, err := waitUTXOsEnough(addr, n, peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}
	logger.Infof("addr %s utxo count: %d", addr, m)
	logger.Info("--- PASS: prepareUTXOs")
}

func txRepeatTest(fromAddr, toAddr string, execPeer string, times int) {
	logger.Info("=== RUN   txRepeatTest")
	// get balance of miners
	logger.Infof("start to get balance of fromAddr[%s], toAddr[%s] from %s",
		fromAddr, toAddr, execPeer)
	fromBalancePre := balanceFor(fromAddr, execPeer)
	toBalancePre := balanceFor(toAddr, execPeer)
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePre, toAddr, toBalancePre)

	// create a transaction from addr 1 to addr 2
	acc := unlockAccount(fromAddr)
	transfer := uint64(0)
	utxos := utxosFor(fromAddr, execPeer)
	if len(utxos) <= times {
		logger.Infof("utxos count %d is less or equals than repeat times %d on %s,"+
			" set times--", len(utxos), times, fromAddr)
		times--
	}
	sort.Sort(sort.Reverse(sortByUTXOValue(utxos)))
	for i := 0; i < times; i++ {
		//amount := utxos[i+1].TxOut.Value
		amount := utxos[times-1].TxOut.Value
		logger.Infof("sent %d from %s to %s on peer %s", amount, fromAddr, toAddr, execPeer)
		execTx(acc, fromAddr, []string{toAddr}, []uint64{amount}, execPeer)
		transfer += amount
	}

	logger.Infof("wait for transaction brought on chain, timeout %ds", timeoutToChain)
	if err := waitBalanceChanged(toAddr, execPeer, timeoutToChain); err != nil {
		logger.Panic(err)
	}
	time.Sleep(blockTime)
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
		logger.Panicf("txRepeatTest faild: fromGap %d toGap %d and transfer %d",
			fromGap, toGap, transfer)
	}
	logger.Infof("--- PASS: txRepeatTest")
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
		execTx(acc, fromAddr, []string{toAddrs[i]}, []uint64{amount}, execPeer)
		transfer += amount
		logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddr,
			toAddrs[i], execPeer)
	}
	logger.Infof("wait for transaction brought on chain, timeout %ds", timeoutToChain)
	if err := waitBalanceChanged(toAddrs[len(toAddrs)-1], execPeer,
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
		execTx(accounts[i], fromAddrs[i], []string{toAddr}, []uint64{amount}, execPeer)
		transfer += amount
		logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddrs[i],
			toAddr, execPeer)
	}
	logger.Infof("wait for transaction brought on chain, timeout %ds", timeoutToChain)
	if err := waitBalanceChanged(toAddr, execPeer, timeoutToChain); err != nil {
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
