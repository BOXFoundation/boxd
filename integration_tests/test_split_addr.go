// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"sync/atomic"

	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	acc "github.com/BOXFoundation/boxd/wallet/account"
	"google.golang.org/grpc"
)

// SplitAddrTest manage circulation of token
type SplitAddrTest struct {
	*BaseFmw
}

var (
	testAmount, splitFee = uint64(100000), uint64(1000)
)

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
	conn, err := rpcutil.GetGRPCConn(peerAddr)
	if err != nil {
		logger.Panic(err)
		return true
	}
	defer conn.Close()
	//
	miner, ok := PickOneMiner()
	if !ok {
		logger.Warnf("have no miner address to pick")
		return true
	}
	defer UnpickMiner(miner)
	//
	testFee := uint64(1000)
	logger.Infof("waiting for minersAddr %s has %d at least for split address test",
		miner, testAmount+testFee)
	_, err = utils.WaitBalanceEnough(miner, testAmount+splitFee+testFee, conn,
		timeoutToChain)
	if err != nil {
		logger.Error(err)
		return true
	}

	sender, receivers := addrs[0], addrs[1:]
	weights := []uint64{1, 2, 3, 4}

	// send box to sender
	prevSenderBalance := utils.BalanceFor(sender, conn)
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

	_, err = utils.WaitBalanceEqual(sender, prevSenderBalance+testAmount+splitFee,
		conn, timeoutToChain)
	if err != nil {
		logger.Warn(err)
		return
	}
	UnpickMiner(miner)
	curTimes := utils.SplitAddrRepeatTxTimes()
	if utils.SplitAddrRepeatRandom() {
		curTimes = rand.Intn(utils.SplitAddrRepeatTxTimes())
	}
	splitAddrRepeatTest(sender, receivers, weights, curTimes, &t.txCnt, conn)
	//
	return
}

func splitAddrRepeatTest(
	sender string, receivers []string, weights []uint64, times int, txCnt *uint64,
	conn *grpc.ClientConn,
) {
	logger.Info("=== RUN   splitAddrRepeatTest")
	defer logger.Infof("--- DONE: splitAddrRepeatTest")

	if times <= 0 {
		logger.Warn("times is 0, exit")
		return
	}
	if len(receivers) != 4 {
		logger.Panic("split addr test require 4 accounts at leat, now %d", len(receivers))
	}
	// create split addr
	prevSenderBalance := utils.BalanceFor(sender, conn)
	logger.Infof("sender %s prev balance %d", prevSenderBalance)
	logger.Infof("sender %s create split address with addrs %v and weights %v",
		sender, receivers, weights)
	senderAcc, _ := AddrToAcc.Load(sender)
	splitTx, _, err := rpcutil.NewSplitAddrTxWithFee(senderAcc.(*acc.Account),
		receivers, weights, splitFee, conn)
	if err != nil {
		logger.Panic(err)
	}
	hashStr, err := rpcutil.SendTransaction(conn, splitTx)
	if err != nil {
		logger.Panic(err)
	}

	logger.Infof("wait for balance of sender %s equals to %d, timeout %v", sender,
		prevSenderBalance-splitFee, timeoutToChain)
	_, err = utils.WaitBalanceEqual(sender, prevSenderBalance-splitFee, conn, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}
	atomic.AddUint64(txCnt, 1)

	// fetch pre-balance sender and receivers
	senderBalancePre := utils.BalanceFor(sender, conn)
	receiversBalancePre := make([]uint64, len(receivers))
	for i, addr := range receivers {
		receiversBalancePre[i] = utils.BalanceFor(addr, conn)
	}
	logger.Infof("before split addrs txs, sender[%s] balance: %d, receivers balance: %v",
		sender, senderBalancePre, receiversBalancePre)
	// make split address
	//addrSub, err := MakeSplitAddr(receivers[:2], weights[:2])
	splitTxHash := new(crypto.HashType)
	splitTxHash.SetString(hashStr)
	receiverAddrs := make([]types.Address, len(receivers))
	for i, addr := range receivers {
		receiverAddrs[i], _ = types.ParseAddress(addr)
	}
	addr := txlogic.MakeSplitAddress(splitTxHash, 0, receiverAddrs, weights)

	// transfer
	txss, transfer, fee, count, err := rpcutil.NewTxs(senderAcc.(*acc.Account), addr.String(), 1, conn)
	if err != nil {
		logger.Panic(err)
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
	logger.Infof("%s sent %d transactions total %d to split address %s",
		sender, count, transfer, addr)

	logger.Infof("wait for balance of sender %s equal to %d, timeout %v",
		sender, senderBalancePre-transfer-fee, timeoutToChain)
	_, err = utils.WaitBalanceEqual(sender, senderBalancePre-transfer-fee,
		conn, timeoutToChain)
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
		_, err := utils.WaitBalanceEnough(addr, amount, conn, timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}
	}
}
