// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/BOXFoundation/boxd/log"
)

const (
	workDir   = "../.devconfig/"
	keyDir    = "../keyfile/"
	walletDir = "../.devconfig/ws1/box_keystore"

	testPassphrase = "zaq12wsx"

	peerCount = 3
	testCount = 2

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
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	txTest()
}

func txTest() {
	// prepare environment and clean history data
	//if err := prepareEnv(peerCount); err != nil {
	//	logger.Fatal(err)
	//}

	// start nodes
	if err := startNodes(); err != nil {
		logger.Fatal(err)
	}
	defer stopNodes()

	// wait for nodes to be ready
	logger.Info("waiting for 12s: nodes running")
	time.Sleep(12 * time.Second)

	// get addresses of three miners
	minersAddr := allMinersAddr()
	logger.Infof("minersAddr: %v", minersAddr)

	//  generate addresses of test accounts
	testsAddr := genTestAddr(testCount)
	logger.Infof("testsAddr: %v", testsAddr)
	defer removeNewKeystoreFiles()

	// wait for some blocks to generate
	//logger.Info("wait mining for 5 seconds")
	//time.Sleep(5 * time.Second)

	// wait all peers' heights are same
	logger.Info("waiting for all the peers' heights are the same")
	height, err := waitHeightSame()
	if err != nil {
		logger.Fatal(err)
	}
	logger.Infof("now the height of all peers is %d", height)

	singleTxTest(minersAddr[0], minersAddr[1], peersAddr[0], peersAddr[1])

	//// transfer some boxes from T1 to T2
	//someBoxes = 1000 + rand.Intn(1000)

	//// wait some time
	//time.Sleep(15 * time.Second)

	// check the balance of M1 and T1 on all nodes

	// clear databases and logs
	//if err := tearDown(peerCount); err != nil {
	//	logger.Fatal(err)
	//}
}

func singleTxTest(fromAddr, toAddr string, execPeer, checkPeer string) {
	// get balance of miners
	logger.Infof("start to get balance of fromAddr[%s], toAddr[%s] from %s",
		fromAddr, toAddr, execPeer)
	fromBalance, err1 := balanceFor(fromAddr, execPeer)
	toBalance, err2 := balanceFor(toAddr, execPeer)
	if err1 != nil || err2 != nil {
		err := fmt.Errorf("%s, %s", err1, err2)
		panic(err)
	}
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalance, toAddr, toBalance)

	// create a transaction from miner 1 to miner 2 and execute it
	amount := uint64(10000 + rand.Intn(10000))
	if err := execTx(fromAddr, toAddr, amount, execPeer); err != nil {
		panic(err)
	}
	logger.Infof("have sent %d from %s to %s on peer %s", amount, fromAddr,
		toAddr, execPeer)

	logger.Infof("after %v, the transaction would be on the chain", 6*blockTime)
	time.Sleep(6 * blockTime)

	// check the balance of miners
	logger.Infof("start to get balance of miners from %s", checkPeer)
	fromBalance, err1 = balanceFor(fromAddr, checkPeer)
	toBalance, err2 = balanceFor(toAddr, checkPeer)
	if err1 != nil || err2 != nil {
		err := fmt.Errorf("%s, %s", err1, err2)
		panic(err)
	}
	logger.Infof("fromAddr[%s] balance: %d toAddr[%s] balance: %d",
		fromAddr, fromBalance, toAddr, toBalance)
}
