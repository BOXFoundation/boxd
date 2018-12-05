// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/wallet"
)

// Circulation manage circulation of transaction
type Circulation struct {
	accCnt     int
	partLen    int
	txCnt      uint64
	addrs      []string
	accAddrs   []string
	collAddrCh chan<- string
	cirInfoCh  <-chan CirInfo
	quitCh     []chan os.Signal
}

// NewCirculation construct a Circulation instance
func NewCirculation(accCnt, partLen int, collAddrCh chan<- string,
	cirInfoCh <-chan CirInfo) *Circulation {
	c := &Circulation{}
	// get account address
	c.accCnt = accCnt
	c.partLen = partLen
	logger.Infof("start to gen %d address for circulation", accCnt)
	c.addrs, c.accAddrs = utils.GenTestAddr(c.accCnt)
	logger.Debugf("addrs: %v\ntestsAcc: %v", c.addrs, c.accAddrs)
	for _, addr := range c.addrs {
		acc := utils.UnlockAccount(addr)
		AddrToAcc[addr] = acc
	}
	c.collAddrCh = collAddrCh
	c.cirInfoCh = cirInfoCh
	for i := 0; i < (accCnt+partLen-1)/partLen; i++ {
		c.quitCh = append(c.quitCh, make(chan os.Signal, 1))
		signal.Notify(c.quitCh[i], os.Interrupt, os.Kill)
	}
	return c
}

// TearDown clean test accounts files
func (c *Circulation) TearDown() {
	utils.RemoveKeystoreFiles(c.accAddrs...)
}

// Run consumes transaction pending on circulation channel
func (c *Circulation) Run() {
	var wg sync.WaitGroup
	for i := 0; i*c.partLen < len(c.addrs); i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			c.doTx(index)
		}(i)
	}
	wg.Wait()
}

func (c *Circulation) doTx(index int) {
	defer func() {
		logger.Infof("done doTx[%d]", index)
		if x := recover(); x != nil {
			utils.TryRecordError(fmt.Errorf("%v", x))
		}
	}()
	start := index * c.partLen
	end := start + c.partLen
	if end > len(c.addrs) {
		end = len(c.addrs)
	}
	addrs := c.addrs[start:end]
	addrIdx := 0
	logger.Infof("start circulation doTx[%d]", index)
	for {
		select {
		case s := <-c.quitCh[index]:
			logger.Infof("receive quit signal %v, quiting circulation[%d]!", s, index)
			close(c.collAddrCh)
			return
		default:
		}
		addrIdx = addrIdx % len(addrs)
		c.collAddrCh <- addrs[addrIdx]
		toIdx := (addrIdx + 1) % len(addrs)
		toAddr := addrs[toIdx]
		addrIdx = toIdx
		if cirInfo, ok := <-c.cirInfoCh; ok {
			logger.Infof("start box circulation between accounts on %s", cirInfo.PeerAddr)
			count := txRepeatTest(cirInfo.Addr, toAddr, cirInfo.PeerAddr, utils.CircuRepeatTxTimes())
			atomic.AddUint64(&c.txCnt, count)
		}
		if scopeValue(*scope) == basicScope {
			break
		}
	}
}

func txRepeatTest(fromAddr, toAddr string, execPeer string, times int) uint64 {
	defer func() {
		if x := recover(); x != nil {
			utils.TryRecordError(fmt.Errorf("%v", x))
			logger.Error(x)
		}
	}()
	txCnt := uint64(0)
	logger.Info("=== RUN   txRepeatTest")
	if times <= 0 {
		logger.Warn("times is 0, exit")
		return 0
	}
	//
	fromBalancePre := utils.BalanceFor(fromAddr, execPeer)
	if fromBalancePre == 0 {
		logger.Warnf("balance of %s is 0, exit", fromAddr)
		return 0
	}
	toBalancePre := utils.BalanceFor(toAddr, execPeer)
	logger.Infof("fromAddr[%s] balance: %d, toAddr[%s] balance: %d",
		fromAddr, fromBalancePre, toAddr, toBalancePre)
	transfer := uint64(0)
	logger.Infof("start to send tx from %s to %s %d times", fromAddr, toAddr, times)
	// remain at leat 1/5 balance as transaction fee
	base := fromBalancePre / uint64(times) / 5 * 2
	var wg sync.WaitGroup
	workers := utils.CircuWorkers()
	partLen := (times + workers - 1) / workers
	errChans := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		start := i * partLen
		go func(start, partLen int) {
			defer func() {
				wg.Done()
				if x := recover(); x != nil {
					errChans <- fmt.Errorf("%v", x)
				}
			}()
			for j := 0; j < partLen && start+j < times; j++ {
				amount := base + uint64(rand.Int63n(int64(base)))
				logger.Debugf("sent %d from %s to %s on peer %s", amount, fromAddr, toAddr, execPeer)
				utils.ExecTx(AddrToAcc[fromAddr], []string{toAddr}, []uint64{amount}, execPeer)
				atomic.AddUint64(&txCnt, 1)
				atomic.AddUint64(&transfer, amount)
			}
		}(start, partLen)
	}
	wg.Wait()
	if len(errChans) > 0 {
		logger.Panic(<-errChans)
	}
	logger.Infof("%s sent %d times total %d tx to %s on peer %s", fromAddr, times,
		transfer, toAddr, execPeer)
	logger.Infof("wait for balance of %s reach %d, timeout %v", toAddr,
		toBalancePre+transfer, timeoutToChain)
	toBalancePost, err := utils.WaitBalanceEnough(toAddr, toBalancePre+transfer,
		execPeer, timeoutToChain)
	if err != nil {
		utils.TryRecordError(err)
		logger.Warn(err)
	}
	// check the balance of miners
	fromBalancePost := utils.BalanceFor(fromAddr, execPeer)
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePost, toAddr, toBalancePost)
	// prerequisite: neither of fromAddr and toAddr are not miner address
	toGap := toBalancePost - toBalancePre
	fromGap := fromBalancePre - fromBalancePost
	if toGap > fromGap || toGap != transfer {
		err := fmt.Errorf("txRepeatTest faild: fromGap %d toGap %d and transfer %d",
			fromGap, toGap, transfer)
		utils.TryRecordError(err)
		logger.Error(err)
	}
	logger.Infof("--- DONE: txRepeatTest")
	return txCnt
}

// TODO: have not been verified
func txOneToManyTest(fromAddr string, toAddrs []string, totalAmount uint64,
	execPeer string) {
	logger.Info("=== RUN   txOneToManyTest")
	// get balance of fromAddr and toAddrs
	logger.Infof("start to get balance of fromAddr[%s], toAddrs[%v] from %s",
		fromAddr, toAddrs, execPeer)
	fromBalancePre := utils.BalanceFor(fromAddr, execPeer)
	toBalancesPre := make([]uint64, len(toAddrs))
	for i := 0; i < len(toAddrs); i++ {
		b := utils.BalanceFor(toAddrs[i], execPeer)
		toBalancesPre[i] = b
	}
	logger.Infof("fromAddr[%s] balance: %d toAddrs[%v] balance: %v",
		fromAddr, fromBalancePre, toAddrs, toBalancesPre)

	// create a transaction from test account 1 to test accounts and execute it
	acc := utils.UnlockAccount(fromAddr)
	ave := totalAmount / uint64(len(toAddrs))
	transfer := uint64(0)
	for i := 0; i < len(toAddrs); i++ {
		amount := ave/2 + uint64(rand.Int63n(int64(ave)/2))
		utils.ExecTx(acc, []string{toAddrs[i]}, []uint64{amount}, execPeer)
		transfer += amount
		logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddr,
			toAddrs[i], execPeer)
	}
	logger.Infof("wait for transaction brought on chain, timeout %v", timeoutToChain)
	if _, err := utils.WaitBalanceEnough(toAddrs[len(toAddrs)-1], 100000, execPeer,
		timeoutToChain); err != nil {
		logger.Panic(err)
	}
	time.Sleep(blockTime)
	// get balance of fromAddr and toAddrs
	logger.Infof("start to get balance of fromAddr[%s], toAddrs[%v] from %s",
		fromAddr, toAddrs, execPeer)
	fromBalancePost := utils.BalanceFor(fromAddr, execPeer)
	toBalancesPost := make([]uint64, len(toAddrs))
	for i := 0; i < len(toAddrs); i++ {
		b := utils.BalanceFor(toAddrs[i], execPeer)
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

// TODO: have not been verified
func txManyToOneTest(fromAddrs []string, toAddr string, execPeer string) {
	logger.Info("=== RUN   txManyToOneTest")
	// get balance of fromAddrs and toAddr
	logger.Infof("start to get balance of fromAddrs[%v], toAddr[%s] from %s",
		fromAddrs, toAddr, execPeer)
	fromBalancesPre := make([]uint64, len(fromAddrs))
	for i := 0; i < len(fromAddrs); i++ {
		b := utils.BalanceFor(fromAddrs[i], execPeer)
		fromBalancesPre[i] = b
	}
	toBalancePre := utils.BalanceFor(toAddr, execPeer)
	logger.Debugf("fromAddrs[%v] balance: %v toAddr[%s] balance: %d",
		fromAddrs, fromBalancesPre, toAddr, toBalancePre)

	// create a transaction from test accounts to account and execute it
	accounts := make([]*wallet.Account, len(fromAddrs))
	for i := 0; i < len(fromAddrs); i++ {
		acc := utils.UnlockAccount(fromAddrs[i])
		accounts[i] = acc
	}
	transfer := uint64(0)
	for i := 0; i < len(fromAddrs); i++ {
		amount := fromBalancesPre[i] / 2
		utils.ExecTx(accounts[i], []string{toAddr}, []uint64{amount}, execPeer)
		transfer += amount
		logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddrs[i],
			toAddr, execPeer)
	}
	logger.Infof("wait for transaction brought on chain, timeout %v", timeoutToChain)
	if _, err := utils.WaitBalanceEnough(toAddr, 1000, execPeer, timeoutToChain); err != nil {
		logger.Panic(err)
	}
	time.Sleep(blockTime)
	// get balance of fromAddrs and toAddr
	logger.Infof("start to get balance of fromAddrs[%v], toAddr[%s] from %s",
		fromAddrs, toAddr, execPeer)
	fromBalancesPost := make([]uint64, len(fromAddrs))
	for i := 0; i < len(fromAddrs); i++ {
		b := utils.BalanceFor(fromAddrs[i], execPeer)
		fromBalancesPost[i] = b
	}
	toBalancePost := utils.BalanceFor(toAddr, execPeer)
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
