// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	acc "github.com/BOXFoundation/boxd/wallet/account"
)

// TokenTest manage circulation of token
type TokenTest struct {
	*BaseFmw
}

func tokenTest() {
	t := NewTokenTest(utils.TokenAccounts(), utils.TokenUnitAccounts())
	defer t.TearDown()

	// print tx count per TickerDurationTxs
	if scopeValue(*scope) == continueScope {
		go CountTxs(&tokenTestTxCnt, &t.txCnt)
	}

	t.Run(t.HandleFunc)
	logger.Info("done token test")
}

// NewTokenTest construct a TokenTest instance
func NewTokenTest(accCnt int, partLen int) *TokenTest {
	t := &TokenTest{}
	t.BaseFmw = NewBaseFmw(accCnt, partLen)
	return t
}

// HandleFunc hooks test func
func (t *TokenTest) HandleFunc(addrs []string, index *int) (exit bool) {
	defer func() {
		if x := recover(); x != nil {
			utils.TryRecordError(fmt.Errorf("%v", x))
			logger.Error(x)
		}
	}()
	peerAddr := peersAddr[(*index)%len(peersAddr)]
	(*index)++
	//
	miner, ok := PickOneMiner()
	if !ok {
		logger.Warnf("have no miner address to pick")
		return true
	}
	defer UnpickMiner(miner)
	//
	testFee, subsidy := uint64(100000), uint64(1000)
	logger.Infof("waiting for minersAddr %s has %d at least on %s for token test",
		miner, testFee+2*subsidy, peerAddr)
	_, err := utils.WaitBalanceEnough(miner, testFee+2*subsidy, peerAddr, timeoutToChain)
	if err != nil {
		logger.Error(err)
		return true
	}
	if len(addrs) < 3 {
		logger.Errorf("token test require 3 accounts at leat, now %d", len(addrs))
		return true
	}
	issuer, sender, receivers := addrs[0], addrs[1], addrs[2:]
	conn, err := rpcutil.GetGRPCConn(peerAddr)
	if err != nil {
		logger.Warn(err)
		return false
	}
	defer conn.Close()
	minerAcc, _ := AddrToAcc.Load(miner)
	tx, _, _, err := rpcutil.NewTx(minerAcc.(*acc.Account), []string{issuer, sender},
		[]uint64{subsidy, testFee}, conn)
	if err != nil {
		logger.Error(err)
		return
	}
	if _, err := rpcutil.SendTransaction(conn, tx); err != nil &&
		!strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		logger.Error(err)
		return
	}
	UnpickMiner(miner)
	atomic.AddUint64(&t.txCnt, 1)
	totalSupply := uint64(10000)
	tag := txlogic.NewTokenTag("box token", "BOX", 6, totalSupply)
	curTimes := utils.TokenRepeatTxTimes()
	if utils.TokenRepeatRandom() {
		curTimes = rand.Intn(utils.TokenRepeatTxTimes())
	}
	tokenRepeatTest(issuer, sender, receivers, tag, curTimes, &t.txCnt, peerAddr)
	//
	return
}

func tokenRepeatTest(issuer, sender string, receivers []string,
	tag *rpcpb.TokenTag, times int, txCnt *uint64, peerAddr string) {
	logger.Info("=== RUN   tokenRepeatTest")
	defer logger.Info("=== DONE   tokenRepeatTest")

	if times <= 0 {
		logger.Warn("times is 0, exit")
		return
	}

	conn, err := rpcutil.GetGRPCConn(peerAddr)
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()

	// issue some token
	logger.Infof("%s issue %d token to %s", issuer, tag.Supply, sender)

	issuerAcc, _ := AddrToAcc.Load(issuer)
	tx, tid, _, err := rpcutil.NewIssueTokenTx(issuerAcc.(*acc.Account), sender, tag,
		tag.Supply, conn)
	if err != nil {
		logger.Panic(err)
	}
	if _, err := rpcutil.SendTransaction(conn, tx); err != nil &&
		!strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		logger.Panic(err)
	}
	atomic.AddUint64(txCnt, 1)

	// check issue result
	totalAmount := tag.Supply * uint64(tag.Decimal)
	logger.Infof("wait for token balance of sender %s equal to %d, timeout %v",
		sender, totalAmount, timeoutToChain)
	blcSenderPre, err := utils.WaitTokenBalanceEnough(sender, totalAmount, tid,
		peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}

	// check status before transfer
	receiver := receivers[0]
	blcRcvPre := utils.TokenBalanceFor(receiver, tid, peerAddr)
	logger.Infof("before token transfer, sender %s has %d token, receiver %s"+
		" has %d token", sender, blcSenderPre, receiver, blcRcvPre)

	// construct some token txs
	logger.Infof("start to create %d token txs from %s to %s on %s",
		times, sender, receiver, peerAddr)
	txTotalAmount := totalAmount/2 + uint64(rand.Int63n(int64(totalAmount)/2))
	senderAcc, _ := AddrToAcc.Load(sender)
	txs, err := rpcutil.NewTokenTxs(senderAcc.(*acc.Account), receiver, txTotalAmount, times,
		tid, conn)
	if err != nil {
		logger.Panic(err)
	}

	// send token txs
	logger.Infof("start to send %d token txs from %s to %s on %s",
		times, sender, receiver, peerAddr)
	for _, tx := range txs {
		if _, err := rpcutil.SendTransaction(conn, tx); err != nil &&
			!strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
			logger.Panic(err)
		}
		atomic.AddUint64(txCnt, 1)
		time.Sleep(2 * time.Millisecond)
	}
	logger.Infof("%s sent %d times total %d token tx to %s", sender, times,
		txTotalAmount, receiver)

	// query and check token balance
	logger.Infof("wait for token balance of %s equal to %d, timeout %v",
		sender, blcSenderPre-txTotalAmount, timeoutToChain)
	err = utils.WaitTokenBalanceEqualTo(sender, blcSenderPre-txTotalAmount, tid,
		peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}
	logger.Infof("wait for token balance of receiver %s equal to %d, timeout %v",
		receiver, blcRcvPre+txTotalAmount, timeoutToChain)
	err = utils.WaitTokenBalanceEqualTo(receiver, blcRcvPre+txTotalAmount, tid,
		peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}
}
