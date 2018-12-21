// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/client"
	"google.golang.org/grpc"
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
		logger.Infof("done circulation test doTx[%d]", index)
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
			txRepeatTest(cirInfo.Addr, toAddr, cirInfo.PeerAddr, utils.CircuRepeatTxTimes(), &c.txCnt)
		}
		if scopeValue(*scope) == basicScope {
			break
		}
	}
}

func txRepeatTest(fromAddr, toAddr string, execPeer string, times int, txCnt *uint64) {
	defer func() {
		if x := recover(); x != nil {
			utils.TryRecordError(fmt.Errorf("%v", x))
			logger.Error(x)
		}
	}()
	logger.Info("=== RUN   txRepeatTest")
	if times <= 0 {
		logger.Warn("times is 0, exit")
		return
	}
	//
	fromBalancePre := utils.BalanceFor(fromAddr, execPeer)
	if fromBalancePre == 0 {
		logger.Warnf("balance of %s is 0, exit", fromAddr)
		return
	}
	toBalancePre := utils.BalanceFor(toAddr, execPeer)
	logger.Infof("fromAddr[%s] balance: %d, toAddr[%s] balance: %d",
		fromAddr, fromBalancePre, toAddr, toBalancePre)
	logger.Infof("start to construct txs from %s to %s %d times", fromAddr, toAddr, times)
	start := time.Now()
	txss, transfer, fee, count, err := utils.NewTxs(AddrToAcc[fromAddr], toAddr,
		times, execPeer)
	eclipse := float64(time.Since(start).Nanoseconds()) / 1e6
	logger.Infof("create %d txs cost: %6.3f ms", count, eclipse)
	if err != nil {
		logger.Panic(err)
	}
	conn, _ := grpc.Dial(execPeer, grpc.WithInsecure())
	defer conn.Close()
	var wg sync.WaitGroup
	errChans := make(chan error, len(txss))
	logger.Infof("start to send tx from %s to %s %d times", fromAddr, toAddr, times)
	start = time.Now()
	for _, txs := range txss {
		wg.Add(1)
		go func(txs []*types.Transaction) {
			defer func() {
				wg.Done()
				if x := recover(); x != nil {
					errChans <- fmt.Errorf("%v", x)
				}
			}()
			for _, tx := range txs {
				if err := client.SendTransaction(conn, tx); err != nil {
					logger.Panic(err)
				}
				atomic.AddUint64(txCnt, 1)
			}
		}(txs)
	}
	wg.Wait()
	if len(errChans) > 0 {
		logger.Panic(<-errChans)
	}
	eclipse = float64(time.Since(start).Nanoseconds()) / 1e6
	logger.Infof("send %d txs cost: %6.3f ms", count, eclipse)

	logger.Infof("%s sent %d transactions total %d to %s on peer %s",
		fromAddr, count, transfer, toAddr, execPeer)
	logger.Infof("wait for balance of %s reach %d, timeout %v", toAddr,
		toBalancePre+transfer, timeoutToChain)
	toBalancePost, err := utils.WaitBalanceEnough(toAddr, toBalancePre+transfer,
		execPeer, timeoutToChain)
	if err != nil {
		utils.TryRecordError(err)
		logger.Warn(err)
	}
	// check the balance of sender
	fromBalancePost := utils.BalanceFor(fromAddr, execPeer)
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalancePost, toAddr, toBalancePost)
	// prerequisite: neither of fromAddr and toAddr are not miner address
	toGap := toBalancePost - toBalancePre
	fromGap := fromBalancePre - fromBalancePost
	if fromGap != fee+transfer || toGap != transfer {
		err := fmt.Errorf("txRepeatTest faild: fromGap %d toGap %d transfer %d and "+
			"fee %d", fromGap, toGap, transfer, fee)
		utils.TryRecordError(err)
		logger.Error(err)
	}
	logger.Infof("--- DONE: txRepeatTest")
}
