package main

import (
	"fmt"
	"math/rand"
	"time"
)

type txTest struct {
	minersAddr []string
	testsAddr  []string
}

func newTxTest(minersAddr, testsAddr []string) *txTest {
	return &txTest{
		minersAddr: minersAddr,
		testsAddr:  testsAddr,
	}
}

func (tt *txTest) testTx() {
	// check
	if len(tt.testsAddr) < 10 {
		logger.Fatal("test accounts count is less 10")
	}

	// wait all peers' heights are same
	logger.Info("waiting for all the peers' heights are the same")
	height, err := waitHeightSame()
	if err != nil {
		logger.Fatal(err)
	}
	logger.Infof("now the height of all peers is %d", height)

	// test single tx
	singleTxTest(tt.minersAddr[0], tt.testsAddr[0], peersAddr[0], peersAddr[1])

	// test repeast tx between account A to account B
	repeatTxTest(tt.minersAddr[0], tt.testsAddr[1], peersAddr[0], peersAddr[1], 1000)

	// test tx between account A to many other accounts
	multiTxVoutTest(tt.testsAddr[1], tt.testsAddr[2:], peersAddr[1], peersAddr[0])

	// test tx between accounts to a single account
	multiTxVinTest(tt.testsAddr[2:], tt.testsAddr[1], peersAddr[1], peersAddr[0])

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

	logger.Infof("after %v, the transaction would be on the chain", 2*blockTime)
	time.Sleep(2 * blockTime)

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

func repeatTxTest(fromAddr, toAddr string, execPeer, checkPeer string, times int) {
}

func multiTxVoutTest(fromAddr string, toAddr []string, execPeer, checkPeer string) {
}

func multiTxVinTest(fromAddr []string, toAddr string, execPeer, checkPeer string) {
}
