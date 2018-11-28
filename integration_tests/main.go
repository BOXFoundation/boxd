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

	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/wallet"
)

type scopeValue string

const (
	walletDir         = "./.devconfig/ws1/box_keystore/"
	dockerComposeFile = "../docker/docker-compose.yml"

	testPassphrase = "1"
	peerCnt        = 6

	blockTime = 5 * time.Second

	basicScope    scopeValue = "basic"
	mainScope     scopeValue = "main"
	fullScope     scopeValue = "full"
	continueScope scopeValue = "continue"
)

var (
	localConf = struct {
		ConfDir, WorkDir, KeyDir string
	}{"./.devconfig/", "./.devconfig/", "./.devconfig/keyfile/"}

	dockerConf = struct {
		ConfDir, WorkDir, KeyDir string
	}{"../docker/.devconfig/", "../docker/.devconfig/", "../docker/.devconfig/keyfile/"}
)

var logger = log.NewLogger("integration_tests") // logger

// CirInfo defines circulation information
type CirInfo struct {
	Addr     string
	PeerAddr string
}

var (
	peersAddr          []string
	minConsensusBlocks = 5

	scope        = flag.String("scope", "basic", "can select basic/main/full/continue cases")
	newNodes     = flag.Bool("nodes", true, "need to start nodes?")
	enableDocker = flag.Bool("docker", false, "test in docker containers?")
	testsCnt     = flag.Int("accounts", 10, "how many need to create test acconts?")

	minerAddrs []string
	//minerAccAddrs []string
	minerAccs []*wallet.Account

	//AddrToAcc stores addr to account
	AddrToAcc = make(map[string]*wallet.Account)
)

func init() {
	if err := InitConf("./param.json"); err != nil {
		logger.Panicf("init conf using param.json error: %s", err)
	}
	if err := loadConf(); err != nil {
		logger.Panic(err)
	}
	rand.Seed(time.Now().Unix())
	// get addresses of three miners
	files := make([]string, peerCnt)
	for i := 0; i < peerCnt; i++ {
		files[i] = localConf.KeyDir + fmt.Sprintf("key%d.keystore", i+1)
	}
	minerAddrs, minerAccs = minerAccounts(files...)
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
	var err error
	if *newNodes {
		// prepare environment and clean history data
		if err := prepareEnv(peerCnt); err != nil {
			logger.Panic(err)
		}
		//defer tearDown(peerCnt)

		// start nodes
		if *enableDocker {
			peersAddr, err = parseIPlist(".devconfig/docker.iplist")
			if err != nil {
				logger.Panic(err)
			}
			if err := startNodes(); err != nil {
				logger.Panic(err)
			}
			defer stopNodes()
		} else {
			peersAddr, err = parseIPlist(".devconfig/local.iplist")
			if err != nil {
				logger.Panic(err)
			}
			processes, err := startLocalNodes(peerCnt)
			defer stopLocalNodes(processes...)
			if err != nil {
				logger.Panic(err)
			}
		}
	} else {
		peersAddr, err = parseIPlist(".devconfig/testnet.iplist")
		if err != nil {
			logger.Panic(err)
		}
	}
	minConsensusBlocks = (len(peersAddr)+2)/3*2 + 1

	// start test
	var wg sync.WaitGroup
	testItems := 2
	errChans := make(chan error, testItems)
	wg.Add(testItems)
	// test tx
	go func() {
		defer func() {
			wg.Done()
			if x := recover(); x != nil {
				errChans <- fmt.Errorf("%v", x)
			}
		}()
		txTest()
	}()

	// test token
	go func() {
		defer func() {
			wg.Done()
			if x := recover(); x != nil {
				errChans <- fmt.Errorf("%v", x)
			}
		}()
		tokenTest()
	}()

	wg.Wait()
	for len(errChans) > 0 {
		TryRecordError(<-errChans)
	}
	// check whether integration success
	for _, e := range ErrItems {
		logger.Error(e)
	}
	if len(ErrItems) > 0 {
		// use panic to exit since it need to execute defer clause above
		logger.Panicf("integration tests exits with %d errors", len(ErrItems))
	}
	logger.Info("All test cases passed, great job!")
}

func txTest() {
	// define chan
	collPartLen, cirPartLen := 5, 5
	collLen := (*testsCnt + collPartLen - 1) / collPartLen
	cirLen := (*testsCnt + cirPartLen - 1) / cirPartLen
	buffLen := collLen
	if collLen < cirLen {
		buffLen = cirLen
	}
	collAddrCh := make(chan string, buffLen)
	cirInfoCh := make(chan CirInfo, buffLen)

	coll := NewCollection(CollAccounts(), CollUnitAccounts(), collAddrCh, cirInfoCh)
	defer coll.TearDown()
	circu := NewCirculation(CircuAccounts(), CircuUnitAccounts(), collAddrCh, cirInfoCh)
	defer circu.TearDown()

	timeout := blockTime * time.Duration(len(peersAddr)*2)
	logger.Infof("wait for block height of all nodes reach %d, timeout %v",
		minConsensusBlocks, timeout)
	if err := waitAllNodesHeightHigher(peersAddr, minConsensusBlocks, timeout); err != nil {
		logger.Panic(err)
	}

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
	t := NewTokenTest(TokenAccounts())
	timeout := blockTime * time.Duration(len(peersAddr)*2)
	logger.Infof("wait for block height of all nodes reach %d, timeout %v",
		minConsensusBlocks, timeout)
	if err := waitAllNodesHeightHigher(peersAddr, minConsensusBlocks, timeout); err != nil {
		logger.Panic(err)
	}
	defer t.TearDown()
	t.Run()
	logger.Info("done token test")
}
