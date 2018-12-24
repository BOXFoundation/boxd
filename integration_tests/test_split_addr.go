// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/script"
	"google.golang.org/grpc"
)

// SplitAddrTest manage circulation of token
type SplitAddrTest struct {
	*BaseFmw
}

func splitAddrTest() {
	//t := NewSplitAddrTest(utils.TokenAccounts(), utils.TokenUnitAccounts())
	t := NewSplitAddrTest(5, 5)
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
func (t *SplitAddrTest) HandleFunc(addrs []string, index *int) {
	defer func() {
		if x := recover(); x != nil {
			utils.TryRecordError(fmt.Errorf("%v", x))
			logger.Error(x)
		}
	}()
	if len(addrs) != 5 {
		logger.Errorf("split addr test require 5 accounts at leat, now %d", len(addrs))
		return
	}
	peerAddr := peersAddr[(*index)%peerCnt]
	(*index)++
	//
	miner, ok := PickOneMiner()
	if !ok {
		logger.Warnf("have no miner address to pick")
		return
	}
	defer UnpickMiner(miner)
	//
	testAmount, testFee := uint64(1000000), uint64(10000)
	logger.Infof("waiting for minersAddr %s has %d at least on %s for split address test",
		miner, testAmount+testFee, peerAddr)
	_, err := utils.WaitBalanceEnough(miner, testAmount+testFee, peerAddr, timeoutToChain)
	if err != nil {
		logger.Error(err)
		return
	}
	sender, receivers := addrs[0], addrs[1:]
	weights := []uint64{1, 2, 3, 4}
	tx, _, _, err := utils.NewTx(AddrToAcc[miner], []string{sender},
		[]uint64{testAmount}, peerAddr)
	if err != nil {
		logger.Error(err)
		return
	}
	conn, _ := grpc.Dial(peerAddr, grpc.WithInsecure())
	defer conn.Close()
	if err := client.SendTransaction(conn, tx); err != nil {
		logger.Error(err)
		return
	}

	logger.Infof("wait for balance of sender %s reach %d, timeout %v", sender,
		testAmount, timeoutToChain)
	_, err = utils.WaitBalanceEnough(sender, testAmount, peerAddr, timeoutToChain)
	if err != nil {
		utils.TryRecordError(err)
		logger.Error(err)
		return
	}
	UnpickMiner(miner)
	atomic.AddUint64(&t.txCnt, 1)
	times := 100
	splitAddrRepeatTest(sender, receivers, weights, times, &t.txCnt, peerAddr)
	//
}

func splitAddrRepeatTest(sender string, receivers []string, weights []uint64,
	times int, txCnt *uint64, peerAddr string) {
	logger.Info("=== RUN   splitAddrRepeatTest")

	if len(receivers) != 4 {
		logger.Errorf("split addr test require 4 accounts at leat, now %d", len(receivers))
		return
	}

	senderBalancePre := utils.BalanceFor(sender, peerAddr)
	receiversBalancePre := make([]uint64, len(receivers))
	for i, addr := range receivers {
		receiversBalancePre[i] = utils.BalanceFor(addr, peerAddr)
	}
	logger.Infof("before split addrs txs, sender[%s] balance: %d, receivers balance: %v",
		sender, senderBalancePre, receiversBalancePre)
	// make split address
	addrSub, err := makeSplitAddr(receivers[:2], weights[:2])
	if err != nil {
		logger.Panic(err)
	}
	addr, err := makeSplitAddr(append(receivers[2:4], addrSub),
		append(weights[2:4], weights[0]+weights[1]))
	if err != nil {
		logger.Panic(err)
	}
	// transfer
	txss, transfer, fee, count, err := utils.NewTxs(AddrToAcc[sender], addr,
		times, peerAddr)
	if err != nil {
		logger.Error(err)
		return
	}
	conn, _ := grpc.Dial(peerAddr, grpc.WithInsecure())
	defer conn.Close()
	for _, txs := range txss {
		for _, tx := range txs {
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

func makeSplitAddr(addrs []string, weights []uint64) (string, error) {
	if len(addrs) != len(weights) {
		return "", errors.New("invalid params")
	}
	addresses := make([]types.Address, len(addrs))
	for i, addr := range addrs {
		addresses[i], _ = types.NewAddress(addr)
	}
	splitAddress, err := script.SplitAddrScript(addresses, weights).ExtractAddress()
	if err != nil {
		return "", err
	}
	return splitAddress.String(), nil
}
