// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	acc "github.com/BOXFoundation/boxd/wallet/account"
)

// SplitAddrTest manage circulation of token
type SplitAddrTest struct {
	*BaseFmw
}

func splitAddrTest() {
	//t := NewSplitAddrTest(utils.TokenAccounts(), utils.TokenUnitAccounts())
	t := NewSplitAddrTest(utils.SplitAddrAccounts(), utils.SplitAddrUnitAccounts())
	defer t.TearDown()
	t.Run(t.HandleFunc)
	logger.Info("done split address test")
}

// NewSplitAddrTest construct a SplitAddrTest instance
func NewSplitAddrTest(accCnt int, partLen int) *SplitAddrTest {
	t := &SplitAddrTest{}
	t.BaseFmw = NewBaseFmw(accCnt, partLen)
	return t
}

// HandleFunc hooks test func
func (t *SplitAddrTest) HandleFunc(addrs []string, index *int) (exit bool) {
	defer func() {
		if x := recover(); x != nil {
			utils.TryRecordError(fmt.Errorf("%v", x))
			logger.Error(x)
		}
	}()
	if len(addrs) != 5 {
		logger.Errorf("split addr test require 5 accounts at leat, now %d", len(addrs))
		return true
	}
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
	testAmount, splitFee, testFee := uint64(100000), uint64(1000), uint64(1000)
	logger.Infof("waiting for minersAddr %s has %d at least on %s for split address test",
		miner, testAmount+testFee, peerAddr)
	_, err := utils.WaitBalanceEnough(miner, testAmount+splitFee+testFee, peerAddr,
		timeoutToChain)
	if err != nil {
		logger.Error(err)
		return true
	}

	sender, receivers := addrs[0], addrs[1:]
	weights := []uint64{1, 2, 3, 4}

	// send box to sender
	conn, err := rpcutil.GetGRPCConn(peerAddr)
	if err != nil {
		logger.Error(err)
		return
	}
	defer conn.Close()
	logger.Infof("miner %s send %d box to sender %s", miner, testAmount+splitFee, sender)
	minerAcc, _ := AddrToAcc.Load(miner)
	senderTx, _, _, err := rpcutil.NewTx(minerAcc.(*acc.Account), []string{sender},
		[]uint64{testAmount + splitFee}, conn)
	if err != nil {
		logger.Error(err)
		return
	}
	if _, err := rpcutil.SendTransaction(conn, senderTx); err != nil {
		logger.Error(err)
		return
	}

	//bytes, _ := json.MarshalIndent(senderTx, "", "  ")
	//hash, _ := senderTx.CalcTxHash()
	//logger.Infof("senderTx hash: %v\nbody: %s", hash[:], string(bytes))

	time.Sleep(time.Second)
	// create split addr
	logger.Infof("sender %s create split address with addrs %v and weights %v",
		sender, receivers, weights)
	senderAcc, _ := AddrToAcc.Load(sender)
	splitTx, _, _, err := rpcutil.NewSplitAddrTxWithFee(senderAcc.(*acc.Account),
		receivers, weights, splitFee, conn)
	if err != nil {
		logger.Error(err)
		return
	}
	if _, err := rpcutil.SendTransaction(conn, splitTx); err != nil {
		logger.Error(err)
		return
	}

	//bytes, _ = json.MarshalIndent(splitTx, "", "  ")
	//hash, _ = splitTx.CalcTxHash()
	//logger.Infof("splitTx hash: %v\nbody: %s", hash[:], string(bytes))

	logger.Infof("wait for balance of sender %s equals to %d, timeout %v", sender,
		testAmount, timeoutToChain)
	_, err = utils.WaitBalanceEqual(sender, testAmount, peerAddr, timeoutToChain)
	if err != nil {
		utils.TryRecordError(err)
		logger.Error(err)
		return
	}
	UnpickMiner(miner)
	atomic.AddUint64(&t.txCnt, 1)
	curTimes := utils.SplitAddrRepeatTxTimes()
	if utils.SplitAddrRepeatRandom() {
		curTimes = rand.Intn(utils.SplitAddrRepeatTxTimes())
	}
	splitAddrRepeatTest(sender, receivers, weights, curTimes, &t.txCnt, peerAddr)
	//
	return
}

func splitAddrRepeatTest(sender string, receivers []string, weights []uint64,
	times int, txCnt *uint64, peerAddr string) {
	logger.Info("=== RUN   splitAddrRepeatTest")
	defer logger.Infof("--- DONE: splitAddrRepeatTest")

	if times <= 0 {
		logger.Warn("times is 0, exit")
		return
	}

	if len(receivers) != 4 {
		logger.Errorf("split addr test require 4 accounts at leat, now %d", len(receivers))
		return
	}

	// fetch pre-balance sender and receivers
	senderBalancePre := utils.BalanceFor(sender, peerAddr)
	receiversBalancePre := make([]uint64, len(receivers))
	for i, addr := range receivers {
		receiversBalancePre[i] = utils.BalanceFor(addr, peerAddr)
	}
	logger.Infof("before split addrs txs, sender[%s] balance: %d, receivers balance: %v",
		sender, senderBalancePre, receiversBalancePre)
	// make split address
	//addrSub, err := MakeSplitAddr(receivers[:2], weights[:2])
	addr, err := txlogic.MakeSplitAddr(receivers, weights)
	if err != nil {
		logger.Panic(err)
	}
	//addr, err := utils.makeSplitAddr(append(receivers[2:4], addrSub),
	//	append(weights[2:4], weights[0]+weights[1]))
	//if err != nil {
	//	logger.Panic(err)
	//}

	// transfer
	conn, err := rpcutil.GetGRPCConn(peerAddr)
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	senderAcc, _ := AddrToAcc.Load(sender)
	txss, transfer, fee, count, err := rpcutil.NewTxs(senderAcc.(*acc.Account), addr, 1, conn)
	if err != nil {
		logger.Error(err)
		return
	}
	for _, txs := range txss {
		for _, tx := range txs {
			//bytes, _ := json.MarshalIndent(tx, "", "  ")
			//hash, _ := tx.CalcTxHash()
			//logger.Infof("send split tx hash: %v\nbody: %s", hash[:], string(bytes))
			if _, err := rpcutil.SendTransaction(conn, tx); err != nil {
				logger.Panic(err)
			}
			atomic.AddUint64(txCnt, 1)
		}
	}
	logger.Infof("%s sent %d transactions total %d to split address %s on peer %s",
		sender, count, transfer, addr, peerAddr)

	logger.Infof("wait for balance of sender %s equal to %d, timeout %v",
		sender, senderBalancePre-transfer-fee, timeoutToChain)
	_, err = utils.WaitBalanceEqual(sender, senderBalancePre-transfer-fee,
		peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}

	totalWeight := uint64(0)
	for _, w := range weights {
		totalWeight += w
	}
	for i, addr := range receivers {
		amount := receiversBalancePre[i] + transfer/totalWeight*weights[i]
		logger.Infof("wait for balance of receivers[%d] receive %d(%d/%d), timeout: %v",
			i, amount, weights[i], totalWeight, timeoutToChain)
		_, err := utils.WaitBalanceEnough(addr, amount, peerAddr, timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}
	}
}
