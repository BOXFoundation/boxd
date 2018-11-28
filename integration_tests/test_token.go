// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/client"
	"google.golang.org/grpc"
)

// TokenTest manage circulation of token
type TokenTest struct {
	accCnt   int
	addrs    []string
	accAddrs []string
	quitCh   chan os.Signal
}

// NewTokenTest construct a TokenTest instance
func NewTokenTest(accCnt int) *TokenTest {
	t := &TokenTest{}
	// get account address
	t.accCnt = accCnt
	logger.Infof("start to gen %d address for token test", accCnt)
	t.addrs, t.accAddrs = genTestAddr(t.accCnt)
	logger.Debugf("addrs: %v\ntestsAcc: %v", t.addrs, t.accAddrs)
	// get accounts for addrs
	for _, addr := range t.addrs {
		acc := unlockAccount(addr)
		AddrToAcc[addr] = acc
	}
	t.quitCh = make(chan os.Signal, 1)
	signal.Notify(t.quitCh, os.Interrupt, os.Kill)
	return t
}

// TearDown clean test accounts files
func (t *TokenTest) TearDown() {
	removeKeystoreFiles(t.accAddrs...)
}

// Run runs toke test
func (t *TokenTest) Run() {
	defer func() {
		logger.Info("done TokenTest doTx")
		if x := recover(); x != nil {
			TryRecordError(fmt.Errorf("%v", x))
		}
	}()
	if len(t.addrs) < 3 {
		return
	}
	peerIdx := 0
	logger.Infof("start TokenTest doTx")
	for {
		select {
		case s := <-t.quitCh:
			logger.Infof("receive quit signal %v, quiting token test!", s)
			return
		default:
		}
		peerIdx = peerIdx % peerCnt
		peerAddr := peersAddr[peerIdx]
		peerIdx++
		logger.Infof("waiting for minersAddr has %d at least on %s", totalAmount, peerAddr)
		time.Sleep(blockTime)
		addr, _, err := waitOneAddrBalanceEnough(minerAddrs, totalAmount, peerAddr,
			timeoutToChain)
		if err != nil {
			logger.Error(err)
			time.Sleep(blockTime)
			continue
		}

		// transfer some box from a miner to a test account
		blcPre := balanceFor(t.addrs[0], peerAddr)
		feeAmount := uint64(totalAmount / 2)
		logger.Infof("send %d from %s to %s", feeAmount, addr, t.addrs[0])
		execTx(AddrToAcc[addr], []string{t.addrs[0]}, []uint64{feeAmount}, peerAddr)
		logger.Infof("wait for balance of %s equal to %d, timeout %v", t.addrs[0],
			blcPre+feeAmount, timeoutToChain)
		_, err = waitBalanceEnough(t.addrs[0], blcPre+feeAmount, peerAddr, timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}

		// define roles
		issuer, issuee, sender, receiver := t.addrs[0], t.addrs[1], t.addrs[0], t.addrs[2]
		// issue some token
		totalSupply, tokenName := uint64(100000), "box"
		txAmount := totalSupply/2 + uint64(rand.Intn(int(totalSupply)/2))
		logger.Infof("%s issue %d token to %s", issuer, totalSupply, issuee)
		issueTx0 := issueTokenTx(issuer, issuee, tokenName, totalSupply, peerAddr)
		logger.Infof("%s issue %d token to %s", issuer, totalSupply, sender)
		issueTx := issueTokenTx(issuer, sender, tokenName, totalSupply, peerAddr)

		// check issue result
		logger.Infof("wait for token balance of issuee %s equal to %d, timeout %v",
			issuee, totalSupply, timeoutToChain)
		_, err = waitTokenBalanceEnough(issuee, totalSupply, issueTx0, peerAddr,
			timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}
		logger.Infof("wait for token balance of sender %s equal to %d, timeout %v",
			sender, totalSupply, timeoutToChain)
		blcSenderPre, err := waitTokenBalanceEnough(sender, totalSupply, issueTx,
			peerAddr, timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}
		blcRcvPre := tokenBalanceFor(receiver, issueTx, peerAddr)
		logger.Infof("before token transfer, sender %s has %d token, receiver %s"+
			" has %d token", sender, blcSenderPre, receiver, blcRcvPre)

		// transfer some token
		logger.Infof("sender %s transfer %d token to receiver %s", sender, txAmount, receiver)
		transferToken(sender, receiver, txAmount, issueTx, peerAddr)

		// query and check token balance
		logger.Infof("wait for token balance of %s equal to %d, timeout %v",
			sender, blcSenderPre-txAmount, timeoutToChain)
		err = waitTokenBalanceEqualTo(sender, blcSenderPre-txAmount, issueTx,
			peerAddr, timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}
		logger.Infof("wait for token balance of receiver %s equal to %d, timeout %v",
			receiver, blcRcvPre+txAmount, timeoutToChain)
		err = waitTokenBalanceEqualTo(receiver, blcRcvPre+txAmount, issueTx, peerAddr,
			timeoutToChain)
		if err != nil {
			logger.Panic(err)
		}

		//
		if scopeValue(*scope) == basicScope {
			break
		}
	}
}

func issueTokenTx(fromAddr, toAddr, tokenName string, totalSupply uint64,
	peerAddr string) *types.Transaction {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	// create
	fromAddress, err1 := types.NewAddress(fromAddr)
	toAddress, err2 := types.NewAddress(toAddr)
	if err1 != nil || err2 != nil {
		logger.Panicf("%v, %v", err1, err2)
	}
	tx, err := client.CreateTokenIssueTx(conn, fromAddress, toAddress,
		AddrToAcc[fromAddr].PublicKey(), tokenName, uint64(totalSupply),
		AddrToAcc[fromAddr])
	if err != nil {
		logger.Panic(err)
	}
	return tx
}

func transferToken(fromAddr, toAddr string, amount uint64, tx *types.Transaction,
	peerAddr string) *types.Transaction {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	// transfer
	toAddress, err := types.NewAddress(toAddr)
	if err != nil {
		logger.Panic(err)
	}
	targets := map[types.Address]uint64{
		toAddress: amount,
	}
	fromAddress, err := types.NewAddress(fromAddr)
	if err != nil {
		logger.Panic(err)
	}
	txHash, err := tx.TxHash()
	if err != nil {
		logger.Panic(err)
	}
	newTx, err := client.CreateTokenTransferTx(conn, fromAddress, targets,
		AddrToAcc[fromAddr].PublicKey(), txHash, 0, AddrToAcc[fromAddr])
	if err != nil {
		logger.Panic(err)
	}
	return newTx
}

func tokenBalanceFor(addr string, tx *types.Transaction, peerAddr string) uint64 {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	// get balance
	address, err := types.NewAddress(addr)
	if err != nil {
		logger.Panic(err)
	}
	txHash, err := tx.TxHash()
	if err != nil {
		logger.Panic(err)
	}
	return client.GetTokenBalance(conn, address, txHash, 0)
}
