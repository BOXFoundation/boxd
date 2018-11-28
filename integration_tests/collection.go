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
	totalAmount    = 1000000
)

// Collection manage transaction creation and collection
type Collection struct {
	accCnt     int
	partLen    int
	minerAddr  string
	addrs      []string
	accAddrs   []string
	collAddrCh <-chan string
	cirInfoCh  chan<- CirInfo
	quitCh     []chan os.Signal
}

// NewCollection construct a Collection instance
func NewCollection(accCnt, partLen int, collAddrCh <-chan string,
	cirInfoCh chan<- CirInfo) *Collection {
	c := &Collection{}
	// get account address
	c.accCnt = accCnt
	c.partLen = partLen
	logger.Infof("start to gen %d tests address for tx collection", accCnt)
	c.addrs, c.accAddrs = genTestAddr(c.accCnt)
	logger.Debugf("addrs: %v\ntestsAcc: %v", c.addrs, c.accAddrs)
	// get accounts for addrs
	for _, addr := range c.addrs {
		acc := unlockAccount(addr)
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
func (c *Collection) TearDown() {
	removeKeystoreFiles(c.accAddrs...)
}

// Run create transaction and send them to circulation channel
func (c *Collection) Run() {
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

func (c *Collection) doTx(index int) {
	defer func() {
		if x := recover(); x != nil {
			logger.Error(x)
			TryRecordError(fmt.Errorf("%v", x))
		}
	}()
	start := index * c.partLen
	end := start + c.partLen
	if end > len(c.addrs) {
		end = len(c.addrs)
	}
	addrs := c.addrs[start:end]
	peerIdx := 0
	div := (len(c.addrs) + c.partLen - 1) / c.partLen
	logger.Infof("start collection doTx %d", index)
	for {
		select {
		case s := <-c.quitCh[index]:
			logger.Infof("receive quit signal %v, quiting collection!", s)
			close(c.cirInfoCh)
			return
		default:
		}
		// wait for nodes to be ready
		peerIdx = peerIdx % peerCnt
		peerAddr := peersAddr[peerIdx]
		peerIdx++
		logger.Infof("waiting for minersAddr has %d at least on %s", totalAmount*div,
			peerAddr)
		// totalAmount is enough, to multiply is to avoid concurrence balance insufficent
		// sleep index*rpcInterval to avoid "Output already spent by transaction in the
		// pool" error on the same minerAddr
		time.Sleep(time.Duration(index) * rpcInterval)
		addr, _, err := waitOneAddrBalanceEnough(minerAddrs, totalAmount*uint64(div),
			peerAddr, timeoutToChain)
		if err != nil {
			logger.Error(err)
			time.Sleep(blockTime)
			continue
		}
		c.minerAddr = addr
		if collAddr, ok := <-c.collAddrCh; ok {
			logger.Infof("start to launder some fund %d on %s", totalAmount, peerAddr)
			c.launderFunds(collAddr, addrs, peerAddr)
			c.cirInfoCh <- CirInfo{Addr: collAddr, PeerAddr: peerAddr}
		}
		if scopeValue(*scope) == basicScope {
			break
		}
	}
}

// launderFunds generates some money, addr must not be in c.addrs
func (c *Collection) launderFunds(addr string, addrs []string, peerAddr string) uint64 {
	defer func() {
		if x := recover(); x != nil {
			logger.Error(x)
			TryRecordError(fmt.Errorf("%v", x))
		}
	}()
	logger.Info("=== RUN   launderFunds")
	var err error
	count := len(addrs)
	// transfer miner to tests[0:len(addrs)-1]
	amount := totalAmount / uint64(count) / 2
	amounts := make([]uint64, count)
	for i := 0; i < count; i++ {
		amounts[i] = amount + uint64(rand.Int63n(int64(amount)))
	}
	balances := make([]uint64, count)
	for i := 0; i < count; i++ {
		b := balanceFor(addrs[i], peerAddr)
		balances[i] = b
	}
	logger.Debugf("sent %v from %s to others test addrs on peer %s", amounts,
		c.minerAddr, peerAddr)
	execTx(AddrToAcc[c.minerAddr], addrs, amounts, peerAddr)
	for i, addr := range addrs {
		logger.Infof("wait for balance of %s more than %d, timeout %v", addrs[i],
			balances[i]+amounts[i], timeoutToChain)
		balances[i], err = waitBalanceEnough(addr, balances[i]+amounts[i], peerAddr,
			timeoutToChain)
		if err != nil {
			logger.Panic(err)
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
			execTx(AddrToAcc[addrs[i]], addrs, amounts2, peerAddr)
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
		logger.Infof("wait for balance of %s reach %d, timeout %v", addrs[i], expect,
			timeoutToChain)
		balances[i], err = waitBalanceEnough(addrs[i], expect, peerAddr, timeoutToChain)
		if err != nil {
			logger.Panic(err)
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
				fromAddr := addrs[i]
				execTx(AddrToAcc[fromAddr], []string{addr}, []uint64{amount}, peerAddr)
				total += amount
				logger.Debugf("have sent %d from %s to %s", amount, addrs[i], addr)
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
		TryRecordError(err)
		logger.Warn(err)
	}
	logger.Infof("--- DONE: launderFunds, result balance: %d", b)
	return b
}
