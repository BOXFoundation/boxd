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
	"time"
)

const (
	timeoutToChain = 30 * time.Second
	totalAmount    = 1e6
)

// Collection manage transaction creation and collection
type Collection struct {
	accCnt     int
	minerAddr  string
	addrs      []string
	accAddrs   []string
	collAddrCh <-chan string
	cirInfoCh  chan<- CirInfo
	quitCh     chan os.Signal
}

// NewCollection construct a Collection instance
func NewCollection(accCnt int, collAddrCh <-chan string,
	cirInfoCh chan<- CirInfo) *Collection {
	c := &Collection{}
	// get account address
	c.accCnt = accCnt
	logger.Infof("start to gen %d tests address", accCnt)
	c.addrs, c.accAddrs = genTestAddr(c.accCnt)
	logger.Debugf("addrs: %v\ntestsAcc: %v", c.addrs, c.accAddrs)
	// get accounts for addrs
	logger.Infof("start to unlock all %d tests accounts", len(c.addrs))
	for _, addr := range c.addrs {
		acc := unlockAccount(addr)
		AddrToAcc[addr] = acc
	}
	c.collAddrCh = collAddrCh
	c.cirInfoCh = cirInfoCh
	c.quitCh = make(chan os.Signal, 1)
	signal.Notify(c.quitCh, os.Interrupt, os.Kill)
	return c
}

// TearDown clean test accounts files
func (c *Collection) TearDown() {
	removeKeystoreFiles(c.accAddrs...)
}

// Run create transaction and send them to circulation channel
func (c *Collection) Run() {
	defer func() {
		if x := recover(); x != nil {
			logger.Error(x)
		}
	}()
	peerIdx := 0
	for {
		select {
		case s := <-c.quitCh:
			logger.Infof("receive quit signal %v, quiting collection!", s)
			close(c.cirInfoCh)
			return
		default:
		}
		// wait for nodes to be ready
		peerIdx = peerIdx % peerCnt
		peerAddr := peersAddr[peerIdx]
		peerIdx++
		logger.Infof("waiting for minersAddr has BaseSubsidy utxo at least on %s",
			peerAddr)
		addr, _, err := waitOneAddrBalanceEnough(minerAddrs, totalAmount,
			peerAddr, timeoutToChain)
		if err != nil {
			logger.Error(err)
			time.Sleep(blockTime)
			continue
		}
		c.minerAddr = addr
		collAddr := <-c.collAddrCh
		logger.Infof("start to create transactions on %s", peerAddr)
		c.launderFunds(collAddr, peerAddr)
		c.cirInfoCh <- CirInfo{Addr: collAddr, PeerAddr: peerAddr}
	}
}

// launderFunds generates some money, addr must not be in c.addrs
func (c *Collection) launderFunds(addr string, peerAddr string) uint64 {
	defer func() {
		if x := recover(); x != nil {
			logger.Error(x)
		}
	}()
	logger.Info("=== RUN   launderFunds")
	var err error
	count := len(c.addrs)
	// transfer miner to tests[0:len(addrs)-1]
	amount := totalAmount / uint64(count) / 2
	amounts := make([]uint64, count)
	for i := 0; i < count; i++ {
		amounts[i] = amount + uint64(rand.Int63n(int64(amount)))
	}
	balances := make([]uint64, count)
	for i := 0; i < count; i++ {
		b := balanceFor(c.addrs[i], peerAddr)
		balances[i] = b
	}
	logger.Debugf("sent %v from %s to others test addrs on peer %s", amounts,
		c.minerAddr, peerAddr)
	execTx(AddrToAcc[c.minerAddr], c.addrs, amounts, peerAddr)
	for i, addr := range c.addrs {
		logger.Infof("wait for %s balance more than %d, timeout %v", c.addrs[i],
			balances[i]+amounts[i], timeoutToChain)
		balances[i], err = waitBalanceEnough(addr, balances[i]+amounts[i], peerAddr,
			blockTime)
		if err != nil {
			logger.Warn(err)
		}
	}
	allAmounts := make([][]uint64, count)
	amountsRecv := make([]uint64, count)
	amountsSend := make([]uint64, count)
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
				base := amounts[i] / uint64(count) / 2
				amounts2[j] = base + uint64(rand.Int63n(int64(base)))
				amountsSend[i] += amounts2[j]
				amountsRecv[j] += amounts2[j]
			}
			allAmounts[i] = amounts2
			execTx(AddrToAcc[c.addrs[i]], c.addrs, amounts2, peerAddr)
		}(i)
	}
	wg.Wait()
	if len(errChans) > 0 {
		logger.Panic(<-errChans)
	}
	logger.Infof("complete to send tx from each to each")
	// check balance
	for i := 0; i < count; i++ {
		expect := balances[i] + amountsRecv[i] - amountsSend[i]/4*5
		logger.Infof("wait for %s's balance reach %d, timeout %v", c.addrs[i],
			expect, timeoutToChain)
		balances[i], err = waitBalanceEnough(c.addrs[i], expect, peerAddr, timeoutToChain)
		if err != nil {
			logger.Warn(err)
		}
	}
	// gather count*count utxo via transfering from others to the first one
	lastBalance := balanceFor(addr, peerAddr)
	total := uint64(0)
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
			logger.Debugf("start to gather utxo from addr %d to addr %s on peer %s",
				i, addr, peerAddr)
			for j := 0; j < count; j++ {
				amount := allAmounts[j][i] / 2
				fromAddr := c.addrs[i]
				execTx(AddrToAcc[fromAddr], []string{addr}, []uint64{amount}, peerAddr)
				total += amount
				logger.Debugf("have sent %d from %s to %s", amount, c.addrs[i], addr)
			}
		}(i)
	}
	wg.Wait()
	if len(errChans) > 0 {
		logger.Panic(<-errChans)
	}
	logger.Infof("wait for %s balance reach %d timeout %v", addr, total, blockTime)
	b, err := waitBalanceEnough(addr, lastBalance+total, peerAddr, timeoutToChain)
	if err != nil {
		logger.Warn(err)
	}
	logger.Info("--- DONE: launderFunds")
	return b
}
