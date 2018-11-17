// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/wallet"
)

const (
	walletDir         = "./.devconfig/ws1/box_keystore/"
	dockerComposeFile = "../docker/docker-compose.yml"

	testPassphrase = "1"

	peerCnt = 6

	blockTime = 5 * time.Second
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
	UtxoCnt  int
	PeerAddr string
}

var (
	allAddrs = []string{
		// localhost
		"127.0.0.1:19111", // n1
		"127.0.0.1:19121", // n2
		"127.0.0.1:19131", // n3
		"127.0.0.1:19141", // n4
		"127.0.0.1:19151", // n5
		"127.0.0.1:19161", // n6
		// docker
		"127.0.0.1:19111", // n1
		"127.0.0.1:19121", // n2
		"127.0.0.1:19131", // n3
		"127.0.0.1:19141", // n4
		"127.0.0.1:19151", // n5
		"127.0.0.1:19161", // n6
		// testnet
		"192.168.0.227:19111", // n1
		"192.168.0.227:19121", // n2
		"192.168.0.227:19131", // n3
		"192.168.0.226:19141", // n4
		"192.168.0.226:19151", // n5
		"192.168.0.226:19161", // n6
	}

	peersAddr []string

	newNodes     = flag.Bool("nodes", false, "need to start nodes?")
	enableDocker = flag.Bool("docker", false, "test on docker?")
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
	flag.Parse()
	peersAddr = allAddrs[2*peerCnt:]
	if *newNodes {
		// prepare environment and clean history data
		if err := prepareEnv(peerCnt); err != nil {
			logger.Panic(err)
		}
		//defer tearDown(peerCnt)

		// start nodes
		if *enableDocker {
			peersAddr = allAddrs[peerCnt : 2*peerCnt]
			if err := startNodes(); err != nil {
				logger.Panic(err)
			}
			defer stopNodes()
		} else {
			peersAddr = allAddrs[:peerCnt]
			processes, err := startLocalNodes(peerCnt)
			defer stopLocalNodes(processes...)
			if err != nil {
				logger.Panic(err)
			}
		}
	}

	// define chan
	cirPartLen := 5
	collAddrCh := make(chan string, 2)
	cirInfoCh := make(chan CirInfo, cirPartLen)

	coll := NewCollection(*testsCnt, collAddrCh, cirInfoCh)
	defer coll.TearDown()
	circu := NewCirculation(*testsCnt, cirPartLen, collAddrCh, cirInfoCh)
	defer circu.TearDown()

	var wg sync.WaitGroup
	wg.Add(2)
	// collection process
	go func() {
		defer wg.Done()
		coll.Run()
		logger.Info("done coll")
	}()

	// circulation process
	go func() {
		defer wg.Done()
		circu.Run()
		logger.Info("done circu")
	}()

	wg.Wait()
}
