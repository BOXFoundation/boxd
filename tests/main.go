// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"math/rand"
	"time"

	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/wallet"
)

const (
	workDir   = "./.devconfig/"
	keyDir    = "./.devconfig/keyfile/"
	walletDir = "./.devconfig/ws1/box_keystore/"

	testPassphrase = "1"

	peerCount = 6

	blockTime = 5 * time.Second

	dockerComposeFile = "../docker-compose.yml"
)

var logger = log.NewLogger("tests") // logger

var (
	peersAddr = []string{
		// boxd001
		"127.0.0.1:19111", // n1
		"127.0.0.1:19121", // n2
		"127.0.0.1:19131", // n3
		//"192.168.0.227:19111", // n1
		//"192.168.0.227:19121", // n2
		//"192.168.0.227:19131", // n3
		//"39.105.210.220:19001", // s1
		//"39.105.210.220:19011", // s2
		// boxd002
		//"39.105.214.10:19311", // a1
		//"39.105.214.10:19321", // a2
		"127.0.0.1:19141", // n4
		"127.0.0.1:19151", // n5
		"127.0.0.1:19161", // n6
		//"192.168.0.226:19141", // n4
		//"192.168.0.226:19151", // n5
		//"192.168.0.226:19161", // n6
	}

	enableDocker = flag.Bool("docker", false, "test on docker?")

	newNodes = flag.Bool("nodes", false, "need to start nodes?")

	testsCnt = flag.Int("accounts", 10, "how many need to create test acconts?")
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	flag.Parse()
	if *newNodes {
		// prepare environment and clean history data
		if err := prepareEnv(peerCount); err != nil {
			logger.Panic(err)
		}
		//defer tearDown(peerCount)

		// start nodes
		if *enableDocker {
			if err := startNodes(); err != nil {
				logger.Panic(err)
			}
			defer stopNodes()
		} else {
			processes, err := startLocalNodes(peerCount)
			defer stopLocalNodes(processes...)
			if err != nil {
				logger.Panic(err)
			}
		}
	}

	// get addresses of three miners
	minersAddr := allMinersAddr()
	logger.Debugf("minersAddr: %v", minersAddr)

	// generate addresses of test accounts
	testsAccCnt := *testsCnt
	testsAddr, testsAcc := genTestAddr(testsAccCnt)
	logger.Debugf("testsAddr: %v\ntestsAcc: %v", testsAddr, testsAcc)
	defer removeKeystoreFiles(testsAcc...)

	// get accounts for testsAddr
	logger.Infof("start to unlock all %d tests accounts", len(testsAddr))
	accounts := make([]*wallet.Account, len(testsAddr))
	for i := 0; i < len(testsAddr); i++ {
		accounts[i] = unlockAccount(testsAddr[i])
	}

	// wait for nodes to be ready
	logger.Info("waiting for minersAddr has 1 utxo at least")
	addr, _, err := waitOneAddrUTXOEnough(minersAddr, 1, peersAddr[0], timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}
	txTest := newTxTest(addr, testsAddr, accounts, testsAccCnt*testsAccCnt)
	logger.Info("start to test tx")
	txTest.testTx()
}
