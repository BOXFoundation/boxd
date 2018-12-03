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
	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/wallet"
	"google.golang.org/grpc"
)

const (
	walletDir         = "./.devconfig/ws1/box_keystore/"
	dockerComposeFile = "../docker/docker-compose.yml"
	testPassphrase    = "1"

	// RPCTimeout defines rpc timeout
	RPCTimeout = 3 * time.Second
	// RPCInterval defines rpc query interval
	RPCInterval = 300 * time.Millisecond
)

var logger = log.NewLogger("integration_utils") // logger

// KeyStore defines key structure
type KeyStore struct {
	Address string `json:"address"`
}

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
	r, err := rpcClient.GetBalance(ctx, &rpcpb.GetBalanceRequest{Addrs: []string{accAddr}})
	if time.Since(start) > RPCInterval {
		logger.Warnf("cost %v for GetBalance on %s", time.Since(start), peerAddr)
	}
	if err != nil {
		return 0, err
	}
	return r.Balances[accAddr], nil
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
func UnlockAccount(addr string) *wallet.Account {
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

// ExecTx execute a transaction
func ExecTx(account *wallet.Account, toAddrs []string, amounts []uint64,
	peerAddr string) *types.Transaction {
	//
	if len(toAddrs) != len(amounts) {
		logger.Panicf("toAddrs count %d is mismatch with amounts count: %d",
			len(toAddrs), len(amounts))
	}
	//
	fromAddress, err := types.NewAddress(account.Addr())
	if err != nil {
		logger.Panicf("NewAddress fromAddr: %s error: %s", account.Addr(), err)
	}

	// initialize rpc
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	// make toAddr map
	addrAmountMap := make(map[types.Address]uint64, len(toAddrs))
	for i := 0; i < len(toAddrs); i++ {
		toAddress, err := types.NewAddress(toAddrs[i])
		if err != nil {
			logger.Panicf("NewAddress toAddrs: %s error: %s", toAddrs, err)
		}
		addrAmountMap[toAddress] = amounts[i]
	}

	start := time.Now()
	tx, err := client.CreateTransaction(conn, fromAddress, addrAmountMap,
		account.PublicKey(), account, nil, nil)
	if time.Since(start) > 2*RPCInterval {
		logger.Warnf("cost %v for CreateTransaction on %s", time.Since(start), peerAddr)
	}
	if err != nil {
		logger.Panicf("create transaction from %s, addr amont map %v, error: %s",
			fromAddress, addrAmountMap, err)
	}
	return tx
}

func utxosNoPanicFor(accAddr string, peerAddr string) ([]*rpcpb.Utxo, error) {
	b := BalanceFor(accAddr, peerAddr)
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	addr, err := types.NewAddress(accAddr)
	if err != nil {
		return nil, fmt.Errorf("NewAddress addrs: %s error: %s", addr, err)
	}
	logger.Debugf("fund transaction for %s balance %d", addr, b)
	start := time.Now()
	r, err := client.FundTransaction(conn, addr, b)
	if time.Since(start) > RPCInterval {
		logger.Warnf("cost %v for FundTransaction on %s", time.Since(start), peerAddr)
	}
	if err != nil {
		return nil, err
	}
	return r.Utxos, nil
}

func utxosFor(accAddr string, peerAddr string) []*rpcpb.Utxo {
	utxos, err := utxosNoPanicFor(accAddr, peerAddr)
	if err != nil {
		logger.Panic(err)
	}
	return utxos
}

func utxosWithBalanceFor(accAddr string, balance uint64, peerAddr string) []*rpcpb.Utxo {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	addr, err := types.NewAddress(accAddr)
	if err != nil {
		logger.Panic(err)
	}
	start := time.Now()
	r, err := client.FundTransaction(conn, addr, balance)
	if time.Since(start) > RPCInterval {
		logger.Warnf("cost %v for FundTransaction on %s", time.Since(start), peerAddr)
	}
	if err != nil {
		logger.Panic(err)
	}
	return r.Utxos
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

func waitHeightSame() (int, error) {
	timeout := 30
	for i := 0; i < timeout; i++ {
		var hh []int
		for j := 0; j < len(peersAddr); j++ {
			h, err := ChainHeightFor(peersAddr[j])
			if err != nil {
				return 0, err
			}
			hh = append(hh, h)
		}
		if isAllSame(hh) {
			return hh[0], nil
		}
		time.Sleep(time.Second)
	}
	return 0, fmt.Errorf("wait timeout for %ds", timeout)
}

func isAllSame(array []int) bool {
	if len(array) == 0 || len(array) == 1 {
		return true
	}
	for i := 1; i < len(array); i++ {
		if array[i] != array[i-1] {
			return false
		}
	}
	return true
}

// MinerAccounts get miners' accounts
func MinerAccounts(keyFiles ...string) ([]string, []*wallet.Account) {
	var (
		addrs    []string
		accounts []*wallet.Account
	)

	for _, f := range keyFiles {
		var (
			account *wallet.Account
			err     error
		)
		account, err = wallet.NewAccountFromFile(f)
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
func GenTestAddr(count int) ([]string, []string) {
	var addresses, accounts []string
	for i := 0; i < count; i++ {
		var (
			acc, addr string
			err       error
		)
		acc, addr, err = newAccountFromWallet()
		if err != nil {
			logger.Panic(err)
		}
		addresses = append(addresses, addr)
		accounts = append(accounts, acc)
	}
	return addresses, accounts
}

func newAccountFromWallet() (string, string, error) {
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		return "", "", err
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

// WaitTokenBalanceEnough wait tokken balance of addr is more than amount
func WaitTokenBalanceEnough(addr string, amount uint64, tx *types.Transaction,
	checkPeer string, timeout time.Duration) (uint64, error) {
	// return eagerly
	b := TokenBalanceFor(addr, tx, checkPeer)
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
			b = TokenBalanceFor(addr, tx, checkPeer)
			if b >= amount {
				return b, nil
			}
		}
	}
	return b, fmt.Errorf("Timeout for waiting for %s token balance enough %d, now %d",
		addr, amount, b)
}

// WaitTokenBalanceEqualTo wait token balance of addr equal to amount
func WaitTokenBalanceEqualTo(addr string, amount uint64, tx *types.Transaction,
	checkPeer string, timeout time.Duration) error {
	// return eagerly
	b := TokenBalanceFor(addr, tx, checkPeer)
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
			b = TokenBalanceFor(addr, tx, checkPeer)
			if b == amount {
				return nil
			}
		}
	}
	return fmt.Errorf("Timeout for waiting for %s token balance enough %d, now %d",
		addr, amount, b)
}

func waitOneAddrUTXOEnough(addrs []string, n int, checkPeer string,
	timeout time.Duration) (string, int, error) {
	d := RPCInterval
	t := time.NewTicker(d)
	defer t.Stop()
	var utxos []*rpcpb.Utxo
	for i := 0; i < int(timeout/d); i++ {
		select {
		case <-t.C:
			for _, addr := range addrs {
				utxos = utxosFor(addr, checkPeer)
				if len(utxos) >= n {
					return addr, len(utxos), nil
				}
			}
		}
	}
	return addrs[0], len(utxos), fmt.Errorf("timeout for waiting for UTXO reach "+
		"%d for %v, now %d", n, addrs, len(utxos))
}

func waitUTXOsEnough(addr string, n int, checkPeer string, timeout time.Duration) (
	int, error) {
	d := RPCInterval
	t := time.NewTicker(d)
	defer t.Stop()
	var utxos []*rpcpb.Utxo
	var err error
	//out:
	for i := 0; i < int(timeout/d); i++ {
		select {
		case <-t.C:
			utxos, err = utxosNoPanicFor(addr, checkPeer)
			if err != nil {
				logger.Warnf("fetch utxo count for %s error: %s", err)
				//break out
			}
			if len(utxos) >= n {
				return len(utxos), nil
			}
		}
	}
	return len(utxos), fmt.Errorf("timeout for waiting for UTXO reach %d for %s, "+
		"now %d", n, addr, len(utxos))
}

// TokenBalanceFor get token balance of addr
func TokenBalanceFor(addr string, tx *types.Transaction, peerAddr string) uint64 {
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

type sortByUTXOValue []*rpcpb.Utxo

func (x sortByUTXOValue) Len() int           { return len(x) }
func (x sortByUTXOValue) Less(i, j int) bool { return x[i].TxOut.Value < x[j].TxOut.Value }
func (x sortByUTXOValue) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
