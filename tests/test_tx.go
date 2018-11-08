// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"math/rand"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
)

type txTest struct {
	minersAddr []string
	testsAddr  []string
}

func newTxTest(minersAddr, testsAddr []string) *txTest {
	return &txTest{
		minersAddr: minersAddr,
		testsAddr:  testsAddr,
	}
}

func (tt *txTest) testTx(testsAccCnt int) {
	// check
	if len(tt.testsAddr) < testsAccCnt {
		logger.Panicf("test accounts count %d is less %d", len(tt.testsAddr),
			testsAccCnt)
	}

	// wait all peers' heights are same
	logger.Info("waiting for all the peers' heights are the same")
	height, err := waitHeightSame()
	if err != nil {
		logger.Panic(err)
	}
	logger.Infof("now the height of all peers is %d", height)

	// test single tx from miner addr 0 to test addr 0
	amount := uint64(1000000 + rand.Intn(1000000))
	txSingleTest(tt.minersAddr[0], tt.testsAddr[0], amount, peersAddr[0],
		peersAddr[1])

	// test repeast tx test addr 0 to test addr 1
	txRepeatTest(tt.testsAddr[0], tt.testsAddr[1], amount, peersAddr[0],
		peersAddr[1], 1)

	// test tx between account A to many other accounts
	txOneToManyTest(tt.testsAddr[1], tt.testsAddr[2:3], amount/2, peersAddr[1],
		peersAddr[0])

	// test tx between accounts to a single account
	// transfer half of balance from testsAddr[2:] to testsAddr[1]
	txManyToOneTest(tt.testsAddr[2:3], tt.testsAddr[1], peersAddr[1], peersAddr[0])
}

func txSingleTest(fromAddr, toAddr string, amount uint64, execPeer, checkPeer string) {
	logger.Info("=== RUN   txSingleTest")
	// get balance of miners
	logger.Infof("start to get balance of fromAddr[%s], toAddr[%s] from %s",
		fromAddr, toAddr, execPeer)
	fromBalancePre, err1 := balanceFor(fromAddr, execPeer)
	toBalancePre, err2 := balanceFor(toAddr, execPeer)
	if err1 != nil || err2 != nil {
		logger.Panicf("%s, %s", err1, err2)
	}
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePre, toAddr, toBalancePre)

	// create a transaction from miner 1 to miner 2 and execute it
	if err := execTx(fromAddr, toAddr, amount, execPeer); err != nil {
		logger.Panic(err)
	}
	logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddr,
		toAddr, execPeer)

	logger.Infof("after %v, the transaction would be on the chain", 2*blockTime)
	time.Sleep(2 * blockTime)

	// check the balance of miners
	logger.Infof("start to get balance of fromAddr[%s], toAddr[%s] from %s",
		fromAddr, toAddr, checkPeer)
	fromBalancePost, err1 := balanceFor(fromAddr, checkPeer)
	toBalancePost, err2 := balanceFor(toAddr, checkPeer)
	if err1 != nil || err2 != nil {
		logger.Panic("%s, %s", err1, err2)
	}
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePost, toAddr, toBalancePost)

	// prerequisite: amount is less than base subsity(50 billion)
	supplement := 2 * chain.BaseSubsidy // n*BaseSubsidy is a supplement
	toGap := toBalancePost - toBalancePre
	fromGap := (supplement + fromBalancePre - fromBalancePost) % chain.BaseSubsidy
	if toGap != fromGap || toGap != amount {
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
	fromBalancePre, err1 := balanceFor(fromAddr, execPeer)
	toBalancePre, err2 := balanceFor(toAddr, execPeer)
	if err1 != nil || err2 != nil {
		logger.Panic("%s, %s", err1, err2)
	}
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePre, toAddr, toBalancePre)

	// create a transaction from miner 1 to miner 2 and execute it
	ave := totalAmount / uint64(times)
	transfer := uint64(0)
	for i := 0; i < times; i++ {
		amount := ave/2 + uint64(rand.Int63n(int64(ave)/2))
		if err := execTx(fromAddr, toAddr, amount, execPeer); err != nil {
			logger.Panic(err)
		}
		transfer += amount
		logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddr,
			toAddr, execPeer)
	}

	logger.Infof("after %v, the transaction would be on the chain", 2*blockTime)
	time.Sleep(2 * blockTime)

	// check the balance of miners
	logger.Infof("start to get balance of fromAddr[%s], toAddr[%s] from %s",
		fromAddr, toAddr, checkPeer)
	fromBalancePost, err1 := balanceFor(fromAddr, checkPeer)
	toBalancePost, err2 := balanceFor(toAddr, checkPeer)
	if err1 != nil || err2 != nil {
		logger.Panic("%s, %s", err1, err2)
	}
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePost, toAddr, toBalancePost)

	// prerequisite: neither of fromAddr and toAddr are not miner address
	toGap := toBalancePost - toBalancePre
	fromGap := fromBalancePre - fromBalancePost
	if toGap != fromGap || toGap != transfer {
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
	fromBalancePre, err := balanceFor(fromAddr, execPeer)
	if err != nil {
		logger.Panic(err)
	}
	toBalancesPre := make([]uint64, len(toAddrs))
	for i := 0; i < len(toAddrs); i++ {
		b, err := balanceFor(toAddrs[i], execPeer)
		if err != nil {
			logger.Panic(err)
		}
		toBalancesPre[i] = b
	}
	logger.Infof("fromAddr[%s] balance: %d toAddrs[%v] balance: %v",
		fromAddr, fromBalancePre, toAddrs, toBalancesPre)

	// create a transaction from test account 1 to test accounts and execute it
	ave := totalAmount / uint64(len(toAddrs))
	transfer := uint64(0)
	for i := 0; i < len(toAddrs); i++ {
		amount := ave/2 + uint64(rand.Int63n(int64(ave)/2))
		if err := execTx(fromAddr, toAddrs[i], amount, execPeer); err != nil {
			logger.Panic(err)
		}
		transfer += amount
		logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddr,
			toAddrs[i], execPeer)
	}

	logger.Infof("after %v, the transaction would be on the chain", 2*blockTime)
	time.Sleep(2 * blockTime)

	// get balance of fromAddr and toAddrs
	logger.Infof("start to get balance of fromAddr[%s], toAddrs[%v] from %s",
		fromAddr, toAddrs, checkPeer)
	fromBalancePost, err := balanceFor(fromAddr, checkPeer)
	if err != nil {
		logger.Panic(err)
	}
	toBalancesPost := make([]uint64, len(toAddrs))
	for i := 0; i < len(toAddrs); i++ {
		b, err := balanceFor(toAddrs[i], execPeer)
		if err != nil {
			logger.Panic(err)
		}
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
	if toGap != fromGap || toGap != transfer {
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
		b, err := balanceFor(fromAddrs[i], execPeer)
		if err != nil {
			logger.Panic(err)
		}
		fromBalancesPre[i] = b
	}
	toBalancePre, err := balanceFor(toAddr, execPeer)
	if err != nil {
		logger.Panic(err)
	}
	logger.Debugf("fromAddrs[%v] balance: %v toAddr[%s] balance: %d",
		fromAddrs, fromBalancesPre, toAddr, toBalancePre)

	// create a transaction from test accounts to account and execute it
	transfer := uint64(0)
	for i := 0; i < len(fromAddrs); i++ {
		amount := fromBalancesPre[i] / 2
		if err := execTx(fromAddrs[i], toAddr, amount, execPeer); err != nil {
			logger.Panic(err)
		}
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
		b, err := balanceFor(fromAddrs[i], execPeer)
		if err != nil {
			logger.Panic(err)
		}
		fromBalancesPost[i] = b
	}
	toBalancePost, err := balanceFor(toAddr, execPeer)
	if err != nil {
		logger.Panic(err)
	}
	logger.Debugf("fromAddrs[%v] balance: %v toAddr[%s] balance: %d",
		fromAddrs, fromBalancesPost, toAddr, toBalancePost)

	//
	fromGap := uint64(0)
	for i := 0; i < len(fromAddrs); i++ {
		fromGap += fromBalancesPre[i] - fromBalancesPost[i]
	}
	toGap := toBalancePost - toBalancePre
	if fromGap != toGap || toGap != transfer {
		logger.Panicf("txManyToOneTest faild: fromGap %d toGap %d and transfer %d",
			fromGap, toGap, transfer)
	}
	logger.Info("--- PASS: txManyToOneTest")
}
