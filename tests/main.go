// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"math/rand"
	"time"

	"github.com/BOXFoundation/boxd/log"
)

const (
	workDir   = "./.devconfig/"
	keyDir    = "./.devconfig/keyfile/"
	walletDir = "./.devconfig/ws1/box_keystore/"

	testPassphrase = "zaq12wsx"

	peerCount = 3

	blockTime = 5 * time.Second

	dockerComposeFile = "../docker-compose.yml"
)

var logger = log.NewLogger("tests") // logger

var (
	peersAddr = []string{
		"127.0.0.1:19191",
		"127.0.0.1:19181",
		"127.0.0.1:19171",
	}

	enableDocker = flag.Bool("docker", false, "test on docker?")
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	flag.Parse()
	// prepare environment and clean history data
	if err := prepareEnv(peerCount); err != nil {
		logger.Fatal(err)
	}
	defer tearDown(peerCount)

	// start nodes
	localPeers := 3
	if *enableDocker {
		if err := startNodes(); err != nil {
			logger.Fatal(err)
		}
		defer stopNodes()
	} else {
		processes, err := startLocalNodes(localPeers)
		defer stopLocalNodes(processes...)
		if err != nil {
			logger.Fatal(err)
		}
	}

	// get addresses of three miners
	minersAddr := allMinersAddr()
	logger.Debugf("minersAddr: %v", minersAddr)

	// generate addresses of test accounts
	testsAccCnt := 10
	testsAddr, testsAcc := genTestAddr(testsAccCnt)
	logger.Debugf("testsAddr: %v\ntestsAcc: %v", testsAddr, testsAcc)
	defer removeKeystoreFiles(testsAcc...)

	// wait for nodes to be ready
	logger.Infof("waiting for %v: nodes running", blockTime)
	time.Sleep(blockTime)

	txTest := newTxTest(minersAddr, testsAddr, 10000)
	txTest.testTx()
}
