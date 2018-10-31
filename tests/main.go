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

	//// wait for some blocks to generate
	//time.Sleep(15 * time.Second)

	//// get balance of each miners

	//// wait some time
	//time.Sleep(15 * time.Second)

	//// check balance of each miners

	//// check miners' balances

	//// create a transaction
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
