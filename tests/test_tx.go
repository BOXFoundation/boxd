// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"math/rand"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/wallet"
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

	// prepare utxo for testsAddr[0]
	tt.prepareUTXOs(peersAddr[0], peersAddr[1])

	//// test single tx from miner addr 0 to test addr 0
	//amount := uint64(1000000 + rand.Intn(1000000))
	//txSingleTest(tt.minersAddr[0], tt.testsAddr[0], amount, peersAddr[0],
	//	peersAddr[1])

	//// test repeast tx test addr 0 to test addr 1
	//txRepeatTest(tt.testsAddr[0], tt.testsAddr[1], amount, peersAddr[0],
	//	peersAddr[1], 1)

	//// test tx between account A to many other accounts
	//txOneToManyTest(tt.testsAddr[1], tt.testsAddr[2:3], amount/2, peersAddr[1],
	//	peersAddr[0])

	//// test tx between accounts to a single account
	//// transfer half of balance from testsAddr[2:] to testsAddr[1]
	//txManyToOneTest(tt.testsAddr[2:3], tt.testsAddr[1], peersAddr[1], peersAddr[0])
}

func (tt *txTest) prepareUTXOs(execPeer, checkPeer string) {
	logger.Info("=== RUN   prepareUTXOs")

	testsCnt := len(tt.testsAddr)
	// 1. miner 0 to tests[0:len(testsAddr)-1]
	acc := unlockAccount(tt.minersAddr[0])
	amount := chain.BaseSubsidy / (10 * uint64(testsCnt))
	amount += uint64(rand.Int63n(int64(amount)))
	amounts := make([]uint64, testsCnt)
	for i := 0; i < testsCnt; i++ {
		amounts[i] = amount/2 + uint64(rand.Int63n(int64(amount)/2))
	}
	minerBalancePre := balanceFor(tt.minersAddr[0], execPeer)
	if minerBalancePre < amount*uint64(testsCnt) {
		logger.Fatalf("balance of miner[0] is less than %d", amount*uint64(testsCnt))
	}
	logger.Infof("start to sent %v from addr %s to testsAddr on peer %s",
		amounts, tt.minersAddr[0], execPeer)
	execTx(acc, tt.minersAddr[0], tt.testsAddr, amounts, execPeer)
	logger.Infof("wait for a block time %v", blockTime)
	time.Sleep(blockTime)
	for i, addr := range tt.testsAddr {
		b := balanceFor(addr, checkPeer)
		if b != amounts[i] {
			logger.Panicf("balance of testsAddr[%d] is %d, that is not equal to "+
				"%d transfered", i, b, amounts[i])
		}
	}
	// get accounts for testsAddr
	logger.Infof("start to unlock all %d tests accounts", testsCnt)
	accounts := make([]*wallet.Account, testsCnt)
	for i := 0; i < testsCnt; i++ {
		accounts[i] = unlockAccount(tt.testsAddr[i])
	}
	// gen peersCnt*peerCnt utxos via sending from each to each
	allAmounts := make([][]uint64, testsCnt)
	var wg sync.WaitGroup
	for i := 0; i < testsCnt; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			amounts2 := make([]uint64, testsCnt)
			for j := 0; j < testsCnt; j++ {
				base := amounts[i] / uint64(testsCnt) / 4
				amounts2[j] = base + uint64(rand.Int63n(int64(base)))
			}
			allAmounts[i] = amounts2
			logger.Infof("start to sent %v from addr %d to testsAddr on peer %s",
				amounts2, i, execPeer)
			execTx(accounts[i], tt.testsAddr[i], tt.testsAddr, amounts2, execPeer)
		}(i)
	}
	wg.Wait()
	logger.Infof("wait for a block time %v", 2*blockTime)
	time.Sleep(2 * blockTime)
	// gather peersCnt*peersCnt utxo via transfering
	// from others to the first one
	for i := 1; i < testsCnt; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			logger.Infof("start to gather utxo from addr %d to addr %d on peer %s",
				i, 0, execPeer)
			for j := 0; j < testsCnt; j++ {
				amount := allAmounts[j][i] / 2
				execTx(accounts[i], tt.testsAddr[i], []string{tt.testsAddr[0]},
					[]uint64{amount}, execPeer)
				logger.Infof("have sent %d from %s to %s", amount, tt.testsAddr[i],
					tt.testsAddr[0])
			}
		}(i)
	}
	wg.Wait()
	logger.Infof("wait for a block time %v", blockTime)
	// check utxo count of testsAddr[0]
	count := txCountFor(tt.testsAddr[0], execPeer)
	logger.Infof("testsAddr[0] tx count: %d", count)
}

func txSingleTest(fromAddr, toAddr string, amount uint64, execPeer, checkPeer string) {
	logger.Info("=== RUN   txSingleTest")
	// get balance of miners
	logger.Infof("start to get balance of fromAddr[%s], toAddr[%s] from %s",
		fromAddr, toAddr, execPeer)
	fromBalancePre := balanceFor(fromAddr, execPeer)
	toBalancePre := balanceFor(toAddr, execPeer)
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePre, toAddr, toBalancePre)

	// create a transaction from miner 1 to miner 2 and execute it
	acc := unlockAccount(fromAddr)
	execTx(acc, fromAddr, []string{toAddr}, []uint64{amount}, execPeer)
	logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddr,
		toAddr, execPeer)

	logger.Infof("after %v, the transaction would be on the chain", 2*blockTime)
	time.Sleep(2 * blockTime)

	// check the balance of miners
	logger.Infof("start to get balance of fromAddr[%s], toAddr[%s] from %s",
		fromAddr, toAddr, checkPeer)
	fromBalancePost := balanceFor(fromAddr, checkPeer)
	toBalancePost := balanceFor(toAddr, checkPeer)
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePost, toAddr, toBalancePost)

	// prerequisite: amount is less than base subsity(50 billion)
	supplement := 2 * chain.BaseSubsidy // n*BaseSubsidy is a supplement
	toGap := toBalancePost - toBalancePre
	fromGap := (supplement + fromBalancePre - fromBalancePost) % chain.BaseSubsidy
	if toGap > fromGap || toGap != amount {
		logger.Panicf("txSingleTest faild: fromGap %d toGap %d and transfer %d",
			fromGap, toGap, amount)
	}
	logger.Info("--- PASS: txSingleTest")
}

func txRepeatTest(fromAddr, toAddr string, totalAmount uint64, execPeer,
	checkPeer string, times int) {
	logger.Info("=== RUN   txRepeatTest")
	// get balance of miners
	logger.Infof("start to get balance of fromAddr[%s], toAddr[%s] from %s",
		fromAddr, toAddr, execPeer)
	fromBalancePre := balanceFor(fromAddr, execPeer)
	toBalancePre := balanceFor(toAddr, execPeer)
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePre, toAddr, toBalancePre)

	// create a transaction from miner 1 to miner 2 and execute it
	acc := unlockAccount(fromAddr)
	ave := totalAmount / uint64(times)
	transfer := uint64(0)
	for i := 0; i < times; i++ {
		amount := ave/2 + uint64(rand.Int63n(int64(ave)/2))
		execTx(acc, fromAddr, []string{toAddr}, []uint64{amount}, execPeer)
		transfer += amount
		logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddr,
			toAddr, execPeer)
	}

	logger.Infof("after %v, the transaction would be on the chain", 2*blockTime)
	time.Sleep(2 * blockTime)

	// check the balance of miners
	logger.Infof("start to get balance of fromAddr[%s], toAddr[%s] from %s",
		fromAddr, toAddr, checkPeer)
	fromBalancePost := balanceFor(fromAddr, checkPeer)
	toBalancePost := balanceFor(toAddr, checkPeer)
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
	execPeer, checkPeer string) {
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

	logger.Infof("after %v, the transaction would be on the chain", 2*blockTime)
	time.Sleep(2 * blockTime)

	// get balance of fromAddr and toAddrs
	logger.Infof("start to get balance of fromAddr[%s], toAddrs[%v] from %s",
		fromAddr, toAddrs, checkPeer)
	fromBalancePost := balanceFor(fromAddr, checkPeer)
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

func txManyToOneTest(fromAddrs []string, toAddr string, execPeer, checkPeer string) {
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

	logger.Infof("after %v, the transaction would be on the chain", 2*blockTime)
	time.Sleep(2 * blockTime)

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
