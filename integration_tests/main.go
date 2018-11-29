// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/wallet"
)

type scopeValue string

const (
	peerCnt = 6

	blockTime = 5 * time.Second

	basicScope    scopeValue = "basic"
	mainScope     scopeValue = "main"
	fullScope     scopeValue = "full"
	continueScope scopeValue = "continue"
)

var logger = log.NewLogger("integration") // logger

// CirInfo defines circulation information
type CirInfo struct {
	Addr     string
	PeerAddr string
}

var (
	minConsensusBlocks = 5

	scope = flag.String("scope", "basic", "can select basic/main/full/continue cases")

	peersAddr  []string
	minerAddrs []string
	minerAccs  []*wallet.Account

	//AddrToAcc stores addr to account
	AddrToAcc = make(map[string]*wallet.Account)

	lastTxTestTxCnt    = uint64(0)
	lastTokenTestTxCnt = uint64(0)
	txTestTxCnt        = uint64(0)
	tokenTestTxCnt     = uint64(0)
)

func init() {
	rand.Seed(time.Now().Unix())
	// get addresses of three miners
	files := make([]string, peerCnt)
	for i := 0; i < peerCnt; i++ {
		files[i] = utils.LocalConf.KeyDir + fmt.Sprintf("key%d.keystore", i+1)
	}
	minerAddrs, minerAccs = utils.MinerAccounts(files...)
	logger.Debugf("minersAddrs: %v", minerAddrs)
	for i, addr := range minerAddrs {
		AddrToAcc[addr] = minerAccs[i]
	}
}

func main() {
	defer func() {
		if x := recover(); x != nil {
			os.Exit(1)
		}
	}()
	flag.Parse()
	if err := utils.LoadConf(); err != nil {
		logger.Panic(err)
	}
	if *utils.NewNodes {
		// prepare environment and clean history data
		if err := utils.PrepareEnv(peerCnt); err != nil {
			logger.Panic(err)
		}
		//defer tearDown(peerCnt)

		// start nodes
		if *utils.EnableDocker {
			if err := utils.StartNodes(); err != nil {
				logger.Panic(err)
			}
			defer utils.StopNodes()
		} else {
			processes, err := utils.StartLocalNodes(peerCnt)
			defer utils.StopLocalNodes(processes...)
			if err != nil {
				logger.Panic(err)
			}
		}
	}
	peersAddr = utils.PeerAddrs()
	minConsensusBlocks = (len(peersAddr)+2)/3*2 + 1

	// print tx count per TickerDurationTxs
	go func() {
		logger.Info("txs ticker for main start")
		time.Sleep(timeoutToChain)
		d := utils.TickerDurationTxs()
		t := time.NewTicker(d)
		defer t.Stop()
		quitCh := make(chan os.Signal, 1)
		signal.Notify(quitCh, os.Interrupt, os.Kill)

		for {
			select {
			case <-t.C:
				totalTxs := txTestTxCnt + tokenTestTxCnt
				lastTotalTxs := lastTxTestTxCnt + lastTokenTestTxCnt
				txs := totalTxs - lastTotalTxs
				logger.Infof("TPS = %6.2f during last %v, total txs = %d",
					float64(txs)/float64(d/time.Second), d, totalTxs)
				lastTxTestTxCnt, lastTokenTestTxCnt = txTestTxCnt, tokenTestTxCnt
			case <-quitCh:
				logger.Info("txs ticker for main exit")
				return
			}
		}
	}()

	// start test
	var wg sync.WaitGroup
	testItems := 2
	errChans := make(chan error, testItems)

	// test tx
	if utils.TxTestEnable() {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				if x := recover(); x != nil {
					errChans <- fmt.Errorf("%v", x)
				}
			}()
			txTest()
		}()
	}

	// test token
	if utils.TokenTestEnable() {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				if x := recover(); x != nil {
					errChans <- fmt.Errorf("%v", x)
				}
			}()
			tokenTest()
		}()
	}

	wg.Wait()
	for len(errChans) > 0 {
		utils.TryRecordError(<-errChans)
	}
	// check whether integration success
	for _, e := range utils.ErrItems {
		logger.Error(e)
	}
	if len(utils.ErrItems) > 0 {
		// use panic to exit since it need to execute defer clause above
		logger.Panicf("integration tests exits with %d errors", len(utils.ErrItems))
	}
	logger.Info("All test cases passed, great job!")
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

	coll := NewCollection(utils.CollAccounts(), utils.CollUnitAccounts(), collAddrCh,
		cirInfoCh)
	defer coll.TearDown()
	circu := NewCirculation(utils.CircuAccounts(), utils.CircuUnitAccounts(), collAddrCh,
		cirInfoCh)
	defer circu.TearDown()

	timeout := blockTime * time.Duration(len(peersAddr)*2)
	logger.Infof("wait for block height of all nodes reach %d, timeout %v",
		minConsensusBlocks, timeout)
	if err := utils.WaitAllNodesHeightHigher(peersAddr, minConsensusBlocks,
		timeout); err != nil {
		logger.Panic(err)
	}

	// print tx count per TickerDurationTxs
	go func() {
		logger.Info("txs ticker for txTest start")
		time.Sleep(timeoutToChain)
		d := utils.TickerDurationTxs()
		t := time.NewTicker(d)
		defer t.Stop()
		quitCh := make(chan os.Signal, 1)
		signal.Notify(quitCh, os.Interrupt, os.Kill)

		for {
			select {
			case <-t.C:
				txTestTxCnt = coll.txCnt + circu.txCnt
			case <-quitCh:
				logger.Info("txs ticker for txTest exit")
				return
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	// collection process
	go func() {
		defer wg.Done()
		coll.Run()
		logger.Info("done collection")
	}()

	// circulation process
	go func() {
		defer wg.Done()
		circu.Run()
		logger.Info("done circulation")
	}()

	wg.Wait()
	logger.Info("done transaction test")
}

func tokenTest() {
	t := NewTokenTest(utils.TokenAccounts())
	timeout := blockTime * time.Duration(len(peersAddr)*2)
	logger.Infof("wait for block height of all nodes reach %d, timeout %v",
		minConsensusBlocks, timeout)
	if err := utils.WaitAllNodesHeightHigher(peersAddr, minConsensusBlocks,
		timeout); err != nil {
		logger.Panic(err)
	}
	defer t.TearDown()

	// print tx count per TickerDurationTxs
	go func() {
		logger.Info("txs ticker for token test start")
		time.Sleep(timeoutToChain)
		d := utils.TickerDurationTxs()
		tk := time.NewTicker(d)
		defer tk.Stop()
		quitCh := make(chan os.Signal, 1)
		signal.Notify(quitCh, os.Interrupt, os.Kill)

		for {
			select {
			case <-tk.C:
				tokenTestTxCnt = t.txCnt
			case <-quitCh:
				logger.Info("txs ticker for tokenTest exit")
				return
			}
		}
	}()

	t.Run()
	logger.Info("done token test")
}
