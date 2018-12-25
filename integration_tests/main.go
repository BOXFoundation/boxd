// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/wallet"
)

type scopeValue string

const (
	peerCnt = 6

	basicScope    scopeValue = "basic"
	mainScope     scopeValue = "main"
	fullScope     scopeValue = "full"
	continueScope scopeValue = "continue"
)

var logger = log.NewLogger("integration") // logger

var (
	//minConsensusBlocks = 5*(peerCnt-1) + 1 // 5 is block count one peer gen once
	minConsensusBlocks = 26

	scope = flag.String("scope", "basic", "can select basic/main/full/continue cases")

	peersAddr  []string
	minerAddrs []string
	minerAccs  []*wallet.Account

	//AddrToAcc stores addr to account
	AddrToAcc = make(map[string]*wallet.Account)
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

	// start test
	//timeout := blockTime * time.Duration(minConsensusBlocks+10)
	//logger.Infof("wait for block height of all nodes reach %d, timeout %v",
	//	minConsensusBlocks, timeout)
	//if err := utils.WaitAllNodesHeightHigher(peersAddr, minConsensusBlocks,
	//	timeout); err != nil {
	//	logger.Panic(err)
	//}

	// print tx count per TickerDurationTxs
	go CountGlobalTxs()

	var wg sync.WaitGroup
	testItems := 3
	errChans := make(chan error, testItems)

	// test tx
	if utils.TxTestEnable() {
		runItem(&wg, errChans, txTest)
	}

	// test token
	if utils.TokenTestEnable() {
		runItem(&wg, errChans, tokenTest)
	}

	// test split address
	if true {
		runItem(&wg, errChans, splitAddrTest)
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
