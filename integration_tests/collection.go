// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/core/chain"
)

const (
	timeoutToChain = 30 * time.Second
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
		logger.Info("waiting for minersAddr has BaseSubsidy utxo at least")
		peerIdx = peerIdx % len(peersAddr)
		peerAddr := peersAddr[peerIdx]
		peerIdx++
		addr, _, err := waitOneAddrBalanceEnough(minerAddrs, chain.BaseSubsidy,
			peerAddr, timeoutToChain)
		if err != nil {
			logger.Error(err)
			time.Sleep(blockTime)
			continue
		}
		c.minerAddr = addr
		collAddr := <-c.collAddrCh
		logger.Infof("start to create transactions on %s", peerAddr)
		n := c.prepareUTXOs(collAddr, c.accCnt*c.accCnt, peerAddr)
		c.cirInfoCh <- CirInfo{Addr: collAddr, UtxoCnt: n, PeerAddr: peerAddr}
	}
}

// prepareUTXOs generates n*n utxos for addr, addr must not be in c.addrs
func (c *Collection) prepareUTXOs(addr string, n int, peerAddr string) int {
	defer func() {
		if x := recover(); x != nil {
			logger.Error(x)
		}
	}()
	logger.Info("=== RUN   prepareUTXOs")
	count := int(math.Ceil(math.Sqrt(float64(n))))
	if count > len(c.addrs) {
		logger.Warnf("tests account is not enough for generate %d utxo", n)
		count = len(c.addrs)
	}
	oldUtxos := utxosFor(addr, peerAddr)
	logger.Infof("before prepareUTXOs %s utxo count: %d", addr, len(oldUtxos))
	// miner to tests[0:len(addrs)-1]
	amount := chain.BaseSubsidy / (10 * uint64(count))
	amount += uint64(rand.Int63n(int64(amount)))
	amounts := make([]uint64, count)
	for i := 0; i < count; i++ {
		amounts[i] = amount/2 + uint64(rand.Int63n(int64(amount)/2))
	}
	minerBalancePre := balanceFor(c.minerAddr, peerAddr)
	if minerBalancePre < amount*uint64(count) {
		logger.Panicf("balance of miner(%d) is less than %d", minerBalancePre,
			amount*uint64(count))
	}
	addrUtxos := make([]int, count)
	//balances := make([]uint64, count)
	for i := 0; i < count; i++ {
		utxos := utxosFor(c.addrs[i], peerAddr)
		addrUtxos[i] = len(utxos)
		//balances[i] = balanceFor(c.addrs[i], peerAddr)
	}
	logger.Debugf("sent %v from %s to others addrs on peer %s", amounts,
		c.minerAddr, peerAddr)
	execTx(AddrToAcc[c.minerAddr], c.addrs[:count], amounts, peerAddr)
	for i, addr := range c.addrs[:count] {
		logger.Infof("wait for %s utxos more than %d, timeout %v", c.addrs[i],
			addrUtxos[i]+1, timeoutToChain)
		_, err := waitUTXOsEnough(addr, addrUtxos[i]+1, peerAddr, timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}
	}
	// check balance
	//for i, addr := range c.addrs[:count] {
	//	b := balanceFor(addr, peerAddr)
	//	if b != amounts[i]+balances[i] {
	//		logger.Panicf("balance of addrs[%d] is %d, that is not equal to "+
	//			"%d transfered", i, b, amounts[i]+balances[i])
	//	}
	//	logger.Debugf("balance of addrs[%d] is %d", i, b)
	//}
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
			//utxos := utxosFor(c.addrs[i], peerAddr)
			//logger.Infof("start to sent %v from %d to addrs on peer %s",
			//	amounts2, i, peerAddr)
			fromAddr := c.addrs[i]
			execTx(AddrToAcc[fromAddr], c.addrs[:count], amounts2, peerAddr)
			//logger.Infof("wait for addr %s utxo more than %d, timeout %v",
			//	c.addrs[i], len(utxos)+count, timeoutToChain)
			//_, err := waitUTXOsEnough(c.addrs[i], len(utxos)+count, peerAddr, timeoutToChain)
			//if err != nil {
			//	errChans <- err
			//}
		}(i)
	}
	wg.Wait()
	if len(errChans) > 0 {
		logger.Panic(<-errChans)
	}
	// gather count*count utxo via transfering from others to the first one
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
			//minAmount := allAmounts[0][i]
			//for j := 0; j < count; j++ {
			//	if allAmounts[j][i] < minAmount {
			//		minAmount = allAmounts[j][i]
			//	}
			//}
			//for j := 0; j < count; j++ {
			//	amount := minAmount / 2
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
	logger.Infof("wait utxo count reach %d on %s, timeout %v", len(oldUtxos)+n,
		addr, blockTime)
	m, err := waitUTXOsEnough(addr, len(oldUtxos)+n, peerAddr, blockTime)
	if err != nil {
		logger.Warn(err)
	}
	logger.Infof("wait for %s balance reach %d timeout %v", addr, total, blockTime)
	if _, err := waitBalanceEnough(addr, total, peerAddr, blockTime); err != nil {
		logger.Warn(err)
	}
	logger.Info("--- DONE: prepareUTXOs")
	return m - len(oldUtxos)
}
