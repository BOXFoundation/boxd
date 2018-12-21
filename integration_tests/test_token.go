// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"

	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/client"
	"google.golang.org/grpc"
)

// TokenTest manage circulation of token
type TokenTest struct {
	accCnt   int
	partLen  int
	txCnt    uint64
	addrs    []string
	accAddrs []string
	quitCh   []chan os.Signal
}

// NewTokenTest construct a TokenTest instance
func NewTokenTest(accCnt int, partLen int) *TokenTest {
	t := &TokenTest{}
	// get account address
	t.accCnt = accCnt
	t.partLen = partLen
	logger.Infof("start to gen %d address for token test", accCnt)
	t.addrs, t.accAddrs = utils.GenTestAddr(t.accCnt)
	logger.Debugf("addrs: %v\ntestsAcc: %v", t.addrs, t.accAddrs)
	// get accounts for addrs
	for _, addr := range t.addrs {
		acc := utils.UnlockAccount(addr)
		AddrToAcc[addr] = acc
	}
	for i := 0; i < (accCnt+partLen-1)/partLen; i++ {
		t.quitCh = append(t.quitCh, make(chan os.Signal, 1))
		signal.Notify(t.quitCh[i], os.Interrupt, os.Kill)
	}
	return t
}

// TearDown clean test accounts files
func (t *TokenTest) TearDown() {
	utils.RemoveKeystoreFiles(t.accAddrs...)
}

// Run runs toke test
func (t *TokenTest) Run() {
	var wg sync.WaitGroup
	for i := 0; i*t.partLen < len(t.addrs); i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			t.doTx(index)
		}(i)
	}
	wg.Wait()
}

// Run runs toke test
func (t *TokenTest) doTx(index int) {
	defer func() {
		logger.Infof("done token test doTx[%d]", index)
		if x := recover(); x != nil {
			utils.TryRecordError(fmt.Errorf("%v", x))
		}
	}()
	if len(t.addrs) < 3 {
		logger.Errorf("token test require 3 accounts at leat, now %d", len(t.addrs))
		return
	}
	peerIdx := peerCnt / 2
	logger.Infof("start token test doTx[%d]", index)
	times := utils.TokenRepeatTxTimes()
	for {
		select {
		case s := <-t.quitCh[index]:
			logger.Infof("receive quit signal %v, quiting token test!", s)
			return
		default:
		}
		peerIdx = peerIdx % peerCnt
		peerAddr := peersAddr[peerIdx]
		peerIdx++
		logger.Infof("waiting for minersAddr has %d at least on %s for token test",
			totalAmount, peerAddr)
		miner := minerAddrs[rand.Intn(len(minerAddrs)-1)]
		_, err := utils.WaitBalanceEnough(miner, 22000000, peerAddr, timeoutToChain)
		if err != nil {
			logger.Error(err)
			continue
		}
		begin := index * t.partLen
		issuer, sender, receivers := t.addrs[begin], t.addrs[begin+1], t.addrs[begin+2:]
		tx, _, _, err := utils.NewTx(AddrToAcc[miner], []string{issuer, sender},
			[]uint64{10000000, 10000000}, peerAddr)
		if err != nil {
			logger.Error(err)
			continue
		}
		conn, _ := grpc.Dial(peerAddr, grpc.WithInsecure())
		defer conn.Close()
		if err := client.SendTransaction(conn, tx); err != nil {
			logger.Error(err)
			continue
		}
		atomic.AddUint64(&t.txCnt, 1)
		tag := utils.NewTokenTag("box token", "BOX", 8)
		tokenRepeatTest(issuer, sender, receivers, tag, times, &t.txCnt, peerAddr)
		//
		if scopeValue(*scope) == basicScope {
			break
		}
	}
}

func tokenRepeatTest(issuer, sender string, receivers []string, tag *utils.TokenTag,
	times int, txCnt *uint64, peerAddr string) {
	// issue some token
	totalSupply := uint64(100000000)
	txTotalAmount := totalSupply/2 + uint64(rand.Int63n(int64(totalSupply)/2))
	logger.Infof("%s issue %d token to %s", issuer, totalSupply, sender)
	tokenID := utils.IssueTokenTx(AddrToAcc[issuer], sender, tag, totalSupply, peerAddr)
	atomic.AddUint64(txCnt, 1)

	// check issue result
	logger.Infof("wait for token balance of sender %s equal to %d, timeout %v",
		sender, totalSupply, timeoutToChain)
	blcSenderPre, err := utils.WaitTokenBalanceEnough(sender, totalSupply, tokenID,
		peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}

	// check status before transfer
	receiver := receivers[0]
	blcRcvPre := utils.TokenBalanceFor(receiver, tokenID, peerAddr)
	logger.Infof("before token transfer, sender %s has %d token, receiver %s"+
		" has %d token", sender, blcSenderPre, receiver, blcRcvPre)

	// construct some token txs
	logger.Infof("start to create %d token txs from %s to %s on %s",
		times, sender, receiver, peerAddr)
	txs, err := utils.NewTokenTxs(AddrToAcc[sender], receiver, txTotalAmount, times,
		tokenID, peerAddr)
	if err != nil {
		logger.Panic(err)
	}

	// send token txs
	logger.Infof("start to send %d token txs from %s to %s on %s",
		times, sender, receiver, peerAddr)
	conn, _ := grpc.Dial(peerAddr, grpc.WithInsecure())
	defer conn.Close()
	for _, tx := range txs {
		if err := client.SendTransaction(conn, tx); err != nil {
			logger.Panic(err)
		}
		atomic.AddUint64(txCnt, 1)
	}
	logger.Infof("%s sent %d times total %d token tx to %s", sender, times,
		txTotalAmount, receiver)

	// query and check token balance
	logger.Infof("wait for token balance of %s equal to %d, timeout %v",
		sender, blcSenderPre-txTotalAmount, timeoutToChain)
	err = utils.WaitTokenBalanceEqualTo(sender, blcSenderPre-txTotalAmount, tokenID,
		peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}
	logger.Infof("wait for token balance of receiver %s equal to %d, timeout %v",
		receiver, blcRcvPre+txTotalAmount, timeoutToChain)
	err = utils.WaitTokenBalanceEqualTo(receiver, blcRcvPre+txTotalAmount, tokenID,
		peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}
}
