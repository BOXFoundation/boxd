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
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/wallet"
)

type scopeValue string

const (
	peerCnt = 6

	blockTime      = 1 * time.Second
	timeoutToChain = 15 * time.Second
	totalAmount    = 10000000

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

type picker struct {
	sync.Mutex
	status []bool
}

var (
	//minConsensusBlocks = 5*(peerCnt-1) + 1 // 5 is block count one peer gen once
	minConsensusBlocks = 26

	scope = flag.String("scope", "basic", "can select basic/main/full/continue cases")

	peersAddr  []string
	minerAddrs []string
	minerAccs  []*wallet.Account

	minerPicker = picker{status: make([]bool, peerCnt)}

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
	logger.Infof("minersAddrs: %v", minerAddrs)
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
		//defer utils.TearDown(peerCnt)

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
		time.Sleep(3 * time.Second) // wait for 3s to let boxd started
	}
	peersAddr = utils.PeerAddrs()

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
				txCnt := atomic.LoadUint64(&txTestTxCnt)
				tokenCnt := atomic.LoadUint64(&tokenTestTxCnt)
				totalTxs := txCnt + tokenCnt
				lastTotalTxs := lastTxTestTxCnt + lastTokenTestTxCnt
				txs := totalTxs - lastTotalTxs
				logger.Infof("TPS = %6.2f during last %v, total txs = %d",
					float64(txs)/float64(d/time.Second), d, totalTxs)
				lastTxTestTxCnt, lastTokenTestTxCnt = txCnt, tokenCnt
			case <-quitCh:
				logger.Info("txs ticker for main exit")
				return
			}
		}
	}()

	// start test
	timeout := blockTime * time.Duration(minConsensusBlocks+10)
	logger.Infof("wait for block height of all nodes reach %d, timeout %v",
		minConsensusBlocks, timeout)
	if err := utils.WaitAllNodesHeightHigher(peersAddr, minConsensusBlocks,
		timeout); err != nil {
		logger.Panic(err)
	}

	var wg sync.WaitGroup
	testItems := 3
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

	// test split address
	if true {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				if x := recover(); x != nil {
					errChans <- fmt.Errorf("%v", x)
				}
			}()
			splitAddrTest()
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

	// print tx count per TickerDurationTxs
	go func() {
		logger.Info("txs ticker for txTest start")
		t := time.NewTicker(time.Second)
		defer t.Stop()
		quitCh := make(chan os.Signal, 1)
		signal.Notify(quitCh, os.Interrupt, os.Kill)

		for {
			select {
			case <-t.C:
				atomic.StoreUint64(&txTestTxCnt, atomic.LoadUint64(&coll.txCnt)+
					atomic.LoadUint64(&circu.txCnt))
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
		coll.Run(coll.HandleFunc)
		logger.Info("done collection")
	}()

	// circulation process
	go func() {
		defer wg.Done()
		circu.Run(circu.HandleFunc)
		logger.Info("done circulation")
	}()

	wg.Wait()
	logger.Info("done transaction test")
}

func tokenTest() {
	t := NewTokenTest(utils.TokenAccounts(), utils.TokenUnitAccounts())
	defer t.TearDown()

	// print tx count per TickerDurationTxs
	go func() {
		logger.Info("txs ticker for token test start")
		tk := time.NewTicker(time.Second)
		defer tk.Stop()
		quitCh := make(chan os.Signal, 1)
		signal.Notify(quitCh, os.Interrupt, os.Kill)

		for {
			select {
			case <-tk.C:
				atomic.StoreUint64(&tokenTestTxCnt, atomic.LoadUint64(&t.txCnt))
			case <-quitCh:
				logger.Info("txs ticker for tokenTest exit")
				return
			}
		}
	}()

	t.Run(t.HandleFunc)
	logger.Info("done token test")
}

func splitAddrTest() {
	//t := NewSplitAddrTest(utils.TokenAccounts(), utils.TokenUnitAccounts())
	t := NewSplitAddrTest(5, 5)
	defer t.TearDown()
	t.Run(t.HandleFunc)
	logger.Info("done split address test")
}

// PickOneMiner picks a miner address that was not picked by other goroutine
func PickOneMiner() (string, bool) {
	minerPicker.Lock()
	defer minerPicker.Unlock()
	for i, picked := range minerPicker.status {
		if !picked {
			minerPicker.status[i] = true
			return minerAddrs[i], true
		}
	}
	return "", false
}

// UnpickMiner unpick a miner address
func UnpickMiner(addr string) bool {
	minerPicker.Lock()
	defer minerPicker.Unlock()
	for i, a := range minerAddrs {
		if a == addr {
			minerPicker.status[i] = false
			return true
		}
	}
	return false
}
