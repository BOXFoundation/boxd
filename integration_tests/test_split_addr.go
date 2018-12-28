// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/client"
	"google.golang.org/grpc"
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
	conn, _ := grpc.Dial(peerAddr, grpc.WithInsecure())
	defer conn.Close()
	logger.Infof("miner %s send %d box to sender %s", miner, testAmount+splitFee, sender)
	senderTx, _, _, err := utils.NewTx(AddrToAcc[miner], []string{sender},
		[]uint64{testAmount + splitFee}, peerAddr)
	if err != nil {
		logger.Error(err)
		return
	}
	if err := client.SendTransaction(conn, senderTx); err != nil {
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
	splitTx, _, _, err := utils.NewSplitAddrTxWithFee(AddrToAcc[sender], receivers,
		weights, splitFee, peerAddr)
	if err != nil {
		logger.Error(err)
		return
	}
	if err := client.SendTransaction(conn, splitTx); err != nil {
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
	times := utils.SplitAddrRepeatTxTimes()
	splitAddrRepeatTest(sender, receivers, weights, times, &t.txCnt, peerAddr)
	//

	return
}

func splitAddrRepeatTest(sender string, receivers []string, weights []uint64,
	times int, txCnt *uint64, peerAddr string) {
	logger.Info("=== RUN   splitAddrRepeatTest")

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
	addr, err := utils.MakeSplitAddr(receivers, weights)
	if err != nil {
		logger.Panic(err)
	}
	//addr, err := utils.makeSplitAddr(append(receivers[2:4], addrSub),
	//	append(weights[2:4], weights[0]+weights[1]))
	//if err != nil {
	//	logger.Panic(err)
	//}

	// transfer
	txss, transfer, fee, count, err := utils.NewTxs(AddrToAcc[sender], addr, 1,
		peerAddr)
	if err != nil {
		logger.Error(err)
		return
	}
	conn, _ := grpc.Dial(peerAddr, grpc.WithInsecure())
	defer conn.Close()
	for _, txs := range txss {
		for _, tx := range txs {
			//bytes, _ := json.MarshalIndent(tx, "", "  ")
			//hash, _ := tx.CalcTxHash()
			//logger.Infof("send split tx hash: %v\nbody: %s", hash[:], string(bytes))
			if err := client.SendTransaction(conn, tx); err != nil {
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
	logger.Infof("--- DONE: splitAddrRepeatTest")
}
