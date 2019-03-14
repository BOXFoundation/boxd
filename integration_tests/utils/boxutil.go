// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	"github.com/BOXFoundation/boxd/wallet"
	acc "github.com/BOXFoundation/boxd/wallet/account"
	"google.golang.org/grpc"
)

const (
	walletDir         = "./.devconfig/ws1/box_keystore/"
	dockerComposeFile = "../docker/docker-compose.yml"
	testPassphrase    = "1"

	// RPCTimeout defines rpc timeout
	RPCTimeout = 10 * time.Second
	// RPCInterval defines rpc query interval
	RPCInterval = 300 * time.Millisecond
)

var logger = log.NewLogger("integration_utils") // logger

func balanceNoPanicFor(accAddr string, peerAddr string) (uint64, error) {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), RPCTimeout)
	defer cancel()
	rpcClient := rpcpb.NewTransactionCommandClient(conn)
	start := time.Now()
	r, err := rpcClient.GetBalance(ctx, &rpcpb.GetBalanceReq{Addrs: []string{accAddr}})
	if time.Since(start) > 2*RPCInterval {
		logger.Warnf("cost %v for GetBalance on %s", time.Since(start), peerAddr)
	}
	if err != nil {
		return 0, err
	}
	return r.Balances[0], nil
}

// BalanceFor get balance of accAddr
func BalanceFor(accAddr string, peerAddr string) uint64 {
	b, err := balanceNoPanicFor(accAddr, peerAddr)
	if err != nil {
		logger.Panicf("balance for %s on peer %s error: %s", accAddr, peerAddr, err)
	}
	return b
}

func balancesFor(peerAddr string, addresses ...string) ([]uint64, error) {
	var bb []uint64
	for _, a := range addresses {
		amount := BalanceFor(a, peerAddr)
		bb = append(bb, amount)
	}
	return bb, nil
}

// UnlockAccount defines unlock account
func UnlockAccount(addr string) *acc.Account {
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		logger.Panic(err)
	}
	account, exists := wltMgr.GetAccount(addr)
	if !exists {
		logger.Panicf("Account %s not managed", addr)
	}
	if err := account.UnlockWithPassphrase(testPassphrase); err != nil {
		logger.Panicf("Fail to unlock account: %v, error: %s", account, err)
	}

	return account
}

// ChainHeightFor get chain height of peer's chain
func ChainHeightFor(peerAddr string) (int, error) {
	// create grpc conn
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// call rpc interface
	ctx, cancel := context.WithTimeout(context.Background(), RPCTimeout)
	defer cancel()
	rpcClient := rpcpb.NewContorlCommandClient(conn)

	start := time.Now()
	r, err := rpcClient.GetBlockHeight(ctx, &rpcpb.GetBlockHeightRequest{})
	if time.Since(start) > RPCInterval {
		logger.Warnf("cost %v for GetBlockHeight on %s", time.Since(start), peerAddr)
	}
	if err != nil {
		return 0, err
	}
	return int(r.Height), nil
}

// WaitAllNodesHeightHigher wait all nodes' height is higher than h
func WaitAllNodesHeightHigher(addrs []string, h int, timeout time.Duration) error {
	d := RPCInterval
	t := time.NewTicker(d)
	defer t.Stop()
	idx := 0
	for i := 0; i < int(timeout/d); i++ {
		select {
		case <-t.C:
			hh, err := ChainHeightFor(addrs[idx])
			if err != nil {
				return err
			}
			if hh >= h {
				idx++
				if idx == len(addrs) {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("timeout for waiting for node %s's block height reach %d",
		addrs[idx], h)
}

// MinerAccounts get miners' accounts
func MinerAccounts(keyFiles ...string) ([]string, []*acc.Account) {
	var (
		addrs    []string
		accounts []*acc.Account
	)

	for _, f := range keyFiles {
		var (
			account *acc.Account
			err     error
		)
		account, err = acc.NewAccountFromFile(f)
		if err != nil {
			logger.Panic(err)
		}
		account.UnlockWithPassphrase(testPassphrase)
		accounts = append(accounts, account)
		pk := account.PrivateKey()
		addrPubHash, err := types.NewAddressFromPubKey(pk.PubKey())
		if err != nil {
			logger.Panic(err)
		}
		addrs = append(addrs, addrPubHash.String())
	}
	return addrs, accounts
}

// GenTestAddr defines generate test address
func GenTestAddr(count int) []string {
	var addresses []string
	for i := 0; i < count; i++ {
		var (
			addr string
			err  error
		)
		addr, err = newAccountFromWallet()
		if err != nil {
			logger.Panic(err)
		}
		addresses = append(addresses, addr)
	}
	return addresses
}

func newAccountFromWallet() (string, error) {
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		return "", err
	}
	return wltMgr.NewAccount(testPassphrase)
}

// WaitOneAddrBalanceEnough wait one addr's balance more than amount
func WaitOneAddrBalanceEnough(addrs []string, amount uint64, checkPeer string,
	timeout time.Duration) (string, uint64, error) {
	d := RPCInterval
	t := time.NewTicker(d)
	defer t.Stop()
	b := uint64(0)
	var err error
	for i := 0; i < int(timeout/d); i++ {
		select {
		case <-t.C:
			for _, addr := range addrs {
				b, err = balanceNoPanicFor(addr, checkPeer)
				if err != nil {
					continue
				}
				if b >= amount {
					return addr, b, nil
				}
			}
		}
	}
	return addrs[0], b, fmt.Errorf("timeout for waiting for balance reach "+
		"%d for %v, now %d", amount, addrs, b)
}

// WaitBalanceEnough wait balance of addr is more than amount
func WaitBalanceEnough(addr string, amount uint64, checkPeer string,
	timeout time.Duration) (uint64, error) {
	// return eagerly
	b := BalanceFor(addr, checkPeer)
	if b >= amount {
		return b, nil
	}
	// check repeatedly
	d := RPCInterval
	t := time.NewTicker(d)
	defer t.Stop()
	for i := 0; i < int(timeout/d); i++ {
		select {
		case <-t.C:
			b = BalanceFor(addr, checkPeer)
			if b >= amount {
				return b, nil
			}
		}
	}
	return b, fmt.Errorf("Timeout for waiting for %s balance enough %d, now %d",
		addr, amount, b)
}

// WaitBalanceEqual wait balance of addr is more than amount
func WaitBalanceEqual(addr string, amount uint64, checkPeer string,
	timeout time.Duration) (uint64, error) {
	// return eagerly
	b := BalanceFor(addr, checkPeer)
	if b == amount {
		return b, nil
	}
	// check repeatedly
	d := RPCInterval
	t := time.NewTicker(d)
	defer t.Stop()
	for i := 0; i < int(timeout/d); i++ {
		select {
		case <-t.C:
			b = BalanceFor(addr, checkPeer)
			if b == amount {
				return b, nil
			}
		}
	}
	return b, fmt.Errorf("Timeout for waiting for %s balance equal to %d, now %d",
		addr, amount, b)
}

// WaitTokenBalanceEnough wait tokken balance of addr is more than amount
func WaitTokenBalanceEnough(
	addr string, amount uint64, thash string, idx uint32, checkPeer string,
	timeout time.Duration,
) (uint64, error) {
	// return eagerly
	b := TokenBalanceFor(addr, thash, idx, checkPeer)
	if b >= amount {
		return b, nil
	}
	// check repeatedly
	d := RPCInterval
	t := time.NewTicker(d)
	defer t.Stop()
	for i := 0; i < int(timeout/d); i++ {
		select {
		case <-t.C:
			b = TokenBalanceFor(addr, thash, idx, checkPeer)
			if b >= amount {
				return b, nil
			}
		}
	}
	return b, fmt.Errorf("Timeout for waiting for %s token balance enough %d, now %d",
		addr, amount, b)
}

// WaitTokenBalanceEqualTo wait token balance of addr equal to amount
func WaitTokenBalanceEqualTo(
	addr string, amount uint64, thash string, idx uint32, checkPeer string, timeout time.Duration,
) error {
	// return eagerly
	b := TokenBalanceFor(addr, thash, idx, checkPeer)
	if b == amount {
		return nil
	}
	// check repeatedly
	d := RPCInterval
	t := time.NewTicker(d)
	defer t.Stop()
	for i := 0; i < int(timeout/d); i++ {
		select {
		case <-t.C:
			b = TokenBalanceFor(addr, thash, idx, checkPeer)
			if b == amount {
				return nil
			}
		}
	}
	return fmt.Errorf("Timeout for waiting for %s token balance equal to %d, now %d",
		addr, amount, b)
}

// TokenBalanceFor get token balance of addr
func TokenBalanceFor(
	addr string, tokenHash string, tokenIndex uint32, peerAddr string,
) uint64 {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	// get balance
	b, err := rpcutil.GetTokenBalance(conn, []string{addr}, tokenHash, tokenIndex)
	if err != nil {
		logger.Panic(err)
	}
	return b[0]
}
