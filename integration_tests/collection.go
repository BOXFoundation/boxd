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
	"github.com/BOXFoundation/boxd/wallet"
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
	cirAddrCh  chan<- string
	quitCh     chan os.Signal
}

// NewCollection construct a Collection instance
func NewCollection(accCnt int, collAddrCh <-chan string,
	cirAddrCh chan<- string) *Collection {
	c := &Collection{}
	// get account address
	c.accCnt = accCnt
	logger.Infof("start to gen %d tests address", accCnt)
	c.addrs, c.accAddrs = genTestAddr(c.accCnt)
	logger.Debugf("addrs: %v\ntestsAcc: %v", c.addrs, c.accAddrs)
	// get accounts for addrs
	logger.Infof("start to unlock all %d tests accounts", len(c.addrs))
	for i := 0; i < len(c.addrs); i++ {
		acc := unlockAccount(c.addrs[i])
		AddrToAcc.Store(c.addrs[i], acc)
	}
	c.collAddrCh = collAddrCh
	c.cirAddrCh = cirAddrCh
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
		// wait for nodes to be ready
		logger.Info("waiting for minersAddr has 1 utxo at least")
		addr, _, err := waitOneAddrBalanceEnough(minerAddrs, chain.BaseSubsidy,
			peersAddr[0], timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}
		c.minerAddr = addr
		collAddr := <-c.collAddrCh
		logger.Info("start to create transactions")
		c.prepareUTXOs(collAddr, c.accCnt*c.accCnt, peersAddr[0])
		c.cirAddrCh <- collAddr
		peerIdx++
		select {
		case s := <-c.quitCh:
			logger.Infof("receive quit signal %v, quiting collection!", s)
			break
		default:
			logger.Info("start create transactions again ...")
		}
	}
}

// prepareUTXOs generates n*n utxos for addr
func (c *Collection) prepareUTXOs(addr string, n int, peerAddr string) {
	logger.Info("=== RUN   prepareUTXOs")
	count := int(math.Ceil(math.Sqrt(float64(n))))
	if count > len(c.addrs) {
		logger.Panicf("tests account is not enough for generate %d utxo", n)
	}
	//if !util.InArray(addr, c.addrs) {
	//	logger.Panicf("argument addr %s must be in tests accounts", addr)
	//}
	oldUtxos, err := utxosNoPanicFor(addr, peerAddr)
	if err != nil {
		logger.Warn(err)
	}
	logger.Infof("before prepareUTXOs, addr %s utxo count: %d", addr, len(oldUtxos))
	offset := 0
	// miner 0 to tests[0:len(addrs)-1]
	amount := chain.BaseSubsidy / (10 * uint64(count))
	amount += uint64(rand.Int63n(int64(amount)))
	amounts := make([]uint64, count)
	for i := 0; i < count; i++ {
		amounts[i] = amount/2 + uint64(rand.Int63n(int64(amount)/2))
	}
	minerBalancePre := balanceFor(c.minerAddr, peerAddr)
	if minerBalancePre < amount*uint64(count) {
		logger.Panicf("balance of miner[0](%d) is less than %d",
			minerBalancePre, amount*uint64(count))
	}
	addrUtxos := make([]int, count)
	testsAddrBalances := make([]uint64, count)
	for i := 0; i < count; i++ {
		utxos := utxosFor(c.addrs[i], peerAddr)
		addrUtxos[i] = len(utxos)
		testsAddrBalances[i] = balanceFor(c.addrs[i], peerAddr)
	}
	logger.Debugf("start to sent %v from addr %s to addrs on peer %s",
		amounts, c.minerAddr, peerAddr)
	acc, _ := AddrToAcc.Load(c.minerAddr)
	execTx(acc.(*wallet.Account), c.minerAddr, c.addrs[:count], amounts, peerAddr)
	for i, addr := range c.addrs[:count] {
		logger.Infof("wait for %s have utxos more than %d", c.addrs[i], addrUtxos[i]+1)
		_, err := waitUTXOsEnough(addr, addrUtxos[i]+1, peerAddr, timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}
	}
	//for i, addr := range c.addrs[:count] {
	//	b := balanceFor(addr, peerAddr)
	//	if b != amounts[i]+testsAddrBalances[i] {
	//		logger.Panicf("balance of addrs[%d] is %d, that is not equal to "+
	//			"%d transfered", i, b, amounts[i]+testsAddrBalances[i])
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
			addrAmountSum := uint64(0)
			for j := 0; j < count; j++ {
				base := amounts[i] / uint64(count) / 4
				amounts2[j] = base + uint64(rand.Int63n(int64(base)))
				if c.addrs[i] == addr {
					addrAmountSum += amounts2[j]
				}
			}
			allAmounts[i] = amounts2
			//utxos := utxosFor(c.addrs[i], peerAddr)
			//logger.Infof("start to sent %v from addr %d to addrs on peer %s",
			//	amounts2, i, peerAddr)
			fromAddr := c.addrs[i]
			acc, _ := AddrToAcc.Load(fromAddr)
			if c.addrs[i] == addr {
				bUtxos := utxosWithBalanceFor(addr, addrAmountSum, peerAddr)
				offset = len(bUtxos)
				logger.Infof("%s send tx %d used %d utxos", addr, addrAmountSum, offset)
			}
			execTx(acc.(*wallet.Account), fromAddr, c.addrs[:count], amounts2, peerAddr)
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
			if c.addrs[i] == addr {
				return
			}
			logger.Debugf("start to gather utxo from addr %d to addr %s on peer %s",
				i, addr, peerAddr)
			minAmount := allAmounts[0][i]
			for j := 0; j < count; j++ {
				if allAmounts[j][i] < minAmount {
					minAmount = allAmounts[j][i]
				}
			}
			for j := 0; j < count; j++ {
				amount := minAmount / 2
				//for j := 0; j < count; j++ {
				//	amount := allAmounts[j][i] / 2
				fromAddr := c.addrs[i]
				acc, _ := AddrToAcc.Load(fromAddr)
				execTx(acc.(*wallet.Account), fromAddr, []string{addr}, []uint64{amount},
					peerAddr)
				logger.Debugf("have sent %d from %s to %s", amount, c.addrs[i], addr)
			}
		}(i)
	}
	wg.Wait()
	if len(errChans) > 0 {
		logger.Panic(<-errChans)
	}
	logger.Infof("wait utxo count reach %d on %s, timeout %v",
		len(oldUtxos)+n-offset*3, addr, timeoutToChain)
	m, err := waitUTXOsEnough(addr, len(oldUtxos)+n-offset*3, peerAddr, blockTime)
	if err != nil {
		logger.Warn(err)
	}
	logger.Infof("addr %s utxo count: %d, wait for %v brought on chain",
		addr, m, blockTime)
	time.Sleep(blockTime)
	logger.Info("--- PASS: prepareUTXOs")
}
