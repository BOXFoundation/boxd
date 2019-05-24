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
	acc "github.com/BOXFoundation/boxd/wallet/account"
)

type scopeValue string

const (
	voidScope     scopeValue = "void"
	basicScope    scopeValue = "basic"
	mainScope     scopeValue = "main"
	fullScope     scopeValue = "full"
	continueScope scopeValue = "continue"
)

var logger = log.NewLogger("integration") // logger

var (
	scope = flag.String("scope", "basic", "can select basic/main/full/continue cases")

	peersAddr  []string
	minerAddrs []string
	minerAccs  []*acc.Account

	//AddrToAcc stores addr to account
	AddrToAcc = new(sync.Map)
)

func init() {
	rand.Seed(time.Now().Unix())
}

func initMinerAcc() {
	minerCnt := len(utils.MinerAddrs())
	files := make([]string, minerCnt)
	for i := 0; i < minerCnt; i++ {
		files[i] = utils.LocalConf.KeyDir + fmt.Sprintf("key%d.keystore", i+1)
	}
	minerAddrs, minerAccs = utils.MinerAccounts(files...)
	logger.Infof("minersAddrs: %v", minerAddrs)
	for i, addr := range minerAddrs {
		AddrToAcc.Store(addr, minerAccs[i])
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
	initMinerAcc()
	initMinerPicker(len(minerAddrs))
	peersAddr = utils.PeerAddrs()

	if *utils.NewNodes {
		// prepare environment and clean history data
		if err := utils.PrepareEnv(len(minerAddrs)); err != nil {
			logger.Panic(err)
		}
		//defer utils.TearDown(len(minerAddrs))

		// start nodes
		if *utils.EnableDocker {
			if err := utils.StartDockerNodes(); err != nil {
				logger.Panic(err)
			}
			defer utils.StopNodes()
		} else {
			processes, err := utils.StartLocalNodes(len(minerAddrs))
			defer utils.StopLocalNodes(processes...)
			if err != nil {
				logger.Panic(err)
			}
		}
		time.Sleep(3 * time.Second) // wait for 3s to let boxd started
	}

	switch scopeValue(*scope) {
	case continueScope:
		// print tx count per TickerDurationTxs
		go CountGlobalTxs()
	case voidScope:
		logger.Info("integration run in void mode, nodes run without txs")
		quitCh := make(chan os.Signal, 1)
		signal.Notify(quitCh, os.Interrupt, os.Kill)
		<-quitCh
		logger.Info("quit integration in void mode")
		return
	}

	var wg sync.WaitGroup
	testCnt := 3
	errChans := make(chan error, testCnt)

	for _, f := range testItems() {
		runItem(&wg, errChans, f)
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
	logger.Info("\r\n\n====>> CONGRATULATION! All CASES PASSED, GREAT JOB! <<====\n\n\r")
}

func testItems() []func() {
	var items []func()
	// test tx
	if utils.TxTestEnable() {
		items = append(items, txTest)
	}

	// test token
	if utils.TokenTestEnable() {
		items = append(items, tokenTest)
	}

	// test split address
	if utils.SplitAddrTestEnable() {
		items = append(items, splitAddrTest)
	}
	return items
}
