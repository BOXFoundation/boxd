// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"time"

	"github.com/BOXFoundation/boxd/log"
)

const (
	workDir   = "../.devconfig/"
	keyDir    = "../keyfile/"
	walletDir = "./wallet/"
	rpcAddr   = "127.0.0.1:19191"

	dockerComposeFile = "../docker-compose.yml"
)

var logger = log.NewLogger("tests") // logger

func main() {
	txTest()
}

func txTest() {
	nodeCount := 3
	testCount := 2

	// prepare environment and clean history data
	//if err := prepareEnv(nodeCount); err != nil {
	//	logger.Fatal(err)
	//}

	// start nodes
	if err := startNodes(); err != nil {
		logger.Fatal(err)
	}

	// wait for nodes to be ready
	logger.Info("wait node running for 3 seconds")
	time.Sleep(3 * time.Second)

	// get pub keys, priv keys and addresses of miners and two test account(T1, T2)
	var minersAddr []string
	for i := 0; i < nodeCount; i++ {
		addr, err := minerAddress(i)
		if err != nil {
			logger.Fatal(err)
		}
		minersAddr = append(minersAddr, addr)
	}
	logger.Infof("minersAddr: %v", minersAddr)
	var testsAddr []string
	for i := 0; i < testCount; i++ {
		addr, err := newAccount()
		if err != nil {
			logger.Fatal(err)
		}
		testsAddr = append(testsAddr, addr)
	}
	logger.Infof("testsAddr: %v", testsAddr)

	// wait for some blocks to generate
	logger.Info("wait mining for 5 seconds")
	time.Sleep(5 * time.Second)

	// get balance of miners
	logger.Info("start getting balance of miners")
	var minersBalance []uint64
	for i := 0; i < nodeCount; i++ {
		amount, err := balanceFor(minersAddr[i], rpcAddr)
		if err != nil {
			logger.Fatal(err)
		}
		minersBalance = append(minersBalance, amount)
	}
	logger.Infof("minersBalance: %v", minersBalance)

	//// wait some time
	//time.Sleep(15 * time.Second)

	// check balance of each miners

	// check miners' balances

	// get initial balance of test accounts
	logger.Info("start getting balance of test accounts")
	var testsBalance []uint64
	for i := 0; i < testCount; i++ {
		amount, err := balanceFor(testsAddr[i], rpcAddr)
		if err != nil {
			logger.Fatal(err)
		}
		testsBalance = append(testsBalance, amount)
	}
	logger.Infof("testsBalance: %v", testsBalance)

	// create a transaction
	//createTx()

	//// transfer some boxes from a miner(M1) to a test count(T1)
	//someBoxes := 2000 + rand.Intn(1000)

	//// wait some time
	//time.Sleep(15 * time.Second)

	//// check the balance of M1 and T1 on all nodes

	//// transfer 2424 from T1 to T2
	//someBoxes = 1000 + rand.Intn(1000)

	//// wait some time
	//time.Sleep(15 * time.Second)

	// check the balance of M1 and T1 on all nodes

	// test finished, stopNodes
	if err := stopNodes(); err != nil {
		logger.Fatal(err)
	}

	// clear databases and logs
	//if err := tearDown(nodeCount); err != nil {
	//	logger.Fatal(err)
	//}
}
