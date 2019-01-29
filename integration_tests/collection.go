// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/client"
)

// Collection manage transaction creation and collection
type Collection struct {
	*BaseFmw
	minerAddr  string
	collAddrCh <-chan string
	cirInfoCh  chan<- CirInfo
}

// CirInfo defines circulation information
type CirInfo struct {
	Addr     string
	PeerAddr string
}

func txTest() {
	// define chan
	collPartLen, cirPartLen := utils.CollUnitAccounts(), utils.CircuUnitAccounts()
	collLen := (utils.CollAccounts() + collPartLen - 1) / collPartLen
	cirLen := (utils.CircuAccounts() + cirPartLen - 1) / cirPartLen
	buffLen := collLen
	if collLen < cirLen {
		buffLen = cirLen
	}
	collAddrCh := make(chan string, buffLen)
	cirInfoCh := make(chan CirInfo, buffLen)

	var (
		coll  *Collection
		circu *Circulation
		wg    sync.WaitGroup
	)

	wg.Add(2)
	// collection process
	go func() {
		defer wg.Done()
		coll = NewCollection(utils.CollAccounts(), utils.CollUnitAccounts(),
			collAddrCh, cirInfoCh)
		defer coll.TearDown()
		coll.Run(coll.HandleFunc)
		logger.Info("done collection")
	}()

	// circulation process
	go func() {
		defer wg.Done()
		circu = NewCirculation(utils.CircuAccounts(), utils.CircuUnitAccounts(),
			collAddrCh, cirInfoCh)
		defer circu.TearDown()
		circu.Run(circu.HandleFunc)
		logger.Info("done circulation")
	}()

	// print tx count per TickerDurationTxs
	for coll == nil || circu == nil {
		time.Sleep(time.Millisecond)
	}
	if scopeValue(*scope) == continueScope {
		go CountTxs(&txTestTxCnt, &coll.txCnt, &circu.txCnt)
	}

	wg.Wait()

	logger.Info("done transaction test")
}

// NewCollection construct a Collection instance
func NewCollection(accCnt, partLen int, collAddrCh <-chan string,
	cirInfoCh chan<- CirInfo) *Collection {
	c := &Collection{}
	// get account address
	c.BaseFmw = NewBaseFmw(accCnt, partLen)
	c.collAddrCh = collAddrCh
	c.cirInfoCh = cirInfoCh
	return c
}

// HandleFunc hooks test func
func (c *Collection) HandleFunc(addrs []string, idx *int) (exit bool) {
	// wait for nodes to be ready
	peerAddr := peersAddr[*idx%len(peersAddr)]
	*idx++
	//
	maddr, ok := PickOneMiner()
	if !ok {
		logger.Warnf("have no miner address to pick")
		return true
	}
	defer UnpickMiner(maddr)
	c.minerAddr = maddr
	//
	logger.Infof("waiting for minersAddr %s has %d at least on %s for collection test",
		maddr, totalAmount, peerAddr)
	_, err := utils.WaitBalanceEnough(c.minerAddr, totalAmount, peerAddr, timeoutToChain)
	if err != nil {
		logger.Warn(err)
		return true
	}
	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, os.Kill)
	select {
	case collAddr := <-c.collAddrCh:
		logger.Infof("start to launder some fund %d on %s", totalAmount, peerAddr)
		for !c.launderFunds(collAddr, addrs, peerAddr, &c.txCnt) {
			select {
			case s := <-quitCh:
				logger.Infof("receive quit signal %v, quiting HandleFunc[%d]!", s, idx)
				return true
			default:
				time.Sleep(time.Second)
			}
		}
		select {
		case c.cirInfoCh <- CirInfo{Addr: collAddr, PeerAddr: peerAddr}:
			return false
		case s := <-quitCh:
			logger.Infof("receive quit signal %v, quiting HandleFunc[%d]!", s, idx)
			return true
		}
	case s := <-quitCh:
		logger.Infof("receive quit signal %v, quiting HandleFunc[%d]!", s, idx)
		return true
	}
}

// launderFunds generates some money, addr must not be in c.addrs
func (c *Collection) launderFunds(addr string, addrs []string, peerAddr string, txCnt *uint64) (ok bool) {
	logger.Info("=== RUN   launderFunds")
	defer func() {
		if x := recover(); x != nil {
			logger.Error(x)
			utils.TryRecordError(fmt.Errorf("%v", x))
			ok = false
		}
	}()
	var err error
	count := len(addrs)
	// transfer from miner to tests[0:len(addrs)-1]
	amount := totalAmount / uint64(count) / 3
	amounts := make([]uint64, count)
	for i := 0; i < count; i++ {
		amounts[i] = amount + uint64(rand.Int63n(int64(amount)))
	}
	balances := make([]uint64, count)
	for i := 0; i < count; i++ {
		balances[i] = utils.BalanceFor(addrs[i], peerAddr)
	}
	logger.Debugf("sent %v from %s to others test addrs on peer %s", amounts,
		c.minerAddr, peerAddr)
	conn, err := client.GetGRPCConn(peerAddr)
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	tx, _, _, err := client.NewTx(AddrToAcc[c.minerAddr], addrs, amounts, conn)
	if err != nil {
		logger.Panic(err)
	}
	err = client.SendTransaction(conn, tx)
	if err != nil && !strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		logger.Panic(err)
	}
	UnpickMiner(c.minerAddr)
	atomic.AddUint64(txCnt, 1)
	logger.Infof("wait for test addrs received fund, timeout %v", timeoutToChain)
	for i, addr := range addrs {
		logger.Debugf("wait for balance of %s more than %d, timeout %v", addrs[i],
			balances[i]+amounts[i], timeoutToChain)
		balances[i], err = utils.WaitBalanceEnough(addr, balances[i]+amounts[i], peerAddr,
			timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}
	}

	// send tx from each to each
	amountsRecv := make([]uint64, count)
	amountsSend := make([]uint64, count)
	amountsFees := make([]uint64, count)
	var txs []*types.Transaction
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
			base := balances[i] / uint64(count) / 6
			for j := 0; j < count; j++ {
				amounts2[j] = base + uint64(rand.Int63n(int64(base)))
				amountsSend[i] += amounts2[j]
				amountsRecv[j] += amounts2[j]
			}
			tx, _, fee, err := client.NewTx(AddrToAcc[addrs[i]], addrs, amounts2, conn)
			if err != nil {
				logger.Panic(err)
			}
			amountsFees[i] = fee
			txs = append(txs, tx)
			atomic.AddUint64(txCnt, 1)
		}(i)
	}
	wg.Wait()
	if len(errChans) > 0 {
		logger.Panic(<-errChans)
	}
	for _, tx := range txs {
		err = client.SendTransaction(conn, tx)
		if err != nil && !strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
			logger.Panic(err)
		}
		//bytes, _ := json.MarshalIndent(tx, "", "  ")
		//hash, _ := tx.CalcTxHash()
		//logger.Infof("tx: %s, hash: %s", string(bytes), hash)
	}
	logger.Infof("complete to send tx from each to each")
	// check balance
	logger.Infof("wait for test addrs received fund, timeout %v", timeoutToChain)
	for i := 0; i < count; i++ {
		expect := balances[i] + amountsRecv[i] - amountsSend[i] - amountsFees[i]
		logger.Debugf("wait for balance of %s reach %d, timeout %v", addrs[i], expect,
			timeoutToChain)
		balances[i], err = utils.WaitBalanceEnough(addrs[i], expect, peerAddr, timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}
	}

	// gather count*count utxo via transfering from others to the first one
	lastBalance := utils.BalanceFor(addr, peerAddr)
	total := uint64(0)
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
			fromAddr := addrs[i]
			txss, transfer, _, _, err := client.NewTxs(AddrToAcc[fromAddr], addr, count, conn)
			if err != nil {
				logger.Panic(err)
			}

			for _, txs := range txss {
				for _, tx := range txs {
					err := client.SendTransaction(conn, tx)
					if err != nil && !strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
						logger.Panic(err)
					}
				}
				atomic.AddUint64(txCnt, uint64(len(txs)))
			}
			total += transfer
			logger.Debugf("have sent %d from %s to %s", transfer, addrs[i], addr)
		}(i)
	}
	wg.Wait()
	if len(errChans) > 0 {
		logger.Panic(<-errChans)
	}
	// check balance
	logger.Infof("wait for %s balance reach %d timeout %v", addr, total, timeoutToChain)
	b, err := utils.WaitBalanceEnough(addr, lastBalance+total, peerAddr, timeoutToChain)
	if err != nil {
		utils.TryRecordError(err)
		logger.Warn(err)
	}
	logger.Infof("--- DONE: launderFunds, result balance: %d", b)

	return true
}
