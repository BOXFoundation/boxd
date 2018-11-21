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

	peerCnt = 6

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
	peersAddr []string

	scope        = flag.String("scope", "basic", "can select basic/main/full/continue cases")
	newNodes     = flag.Bool("nodes", false, "need to start nodes?")
	enableDocker = flag.Bool("docker", false, "test in docker containers?")
	testsCnt     = flag.Int("accounts", 10, "how many need to create test acconts?")

	minerAddrs []string
	//minerAccAddrs []string
	minerAccs []*wallet.Account

	//AddrToAcc stores addr to account
	AddrToAcc = make(map[string]*wallet.Account)
)

func init() {
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
	}

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

	coll := NewCollection(*testsCnt, collPartLen, collAddrCh, cirInfoCh)
	defer coll.TearDown()
	circu := NewCirculation(*testsCnt, cirPartLen, collAddrCh, cirInfoCh)
	defer circu.TearDown()

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

	// check whether integration success
	for _, e := range ErrItems {
		logger.Error(e)
	}
	if len(ErrItems) > 0 {
		// use panic to exit since it need to execute defer clause above
		logger.Panicf("integration tests exits with %d errors", len(ErrItems))
	}
}
