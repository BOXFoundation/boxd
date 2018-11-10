// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/wallet"
	"google.golang.org/grpc"
)

const (
	rpcTimeout = 3 * time.Second
)

// KeyStore defines key structure
type KeyStore struct {
	Address string `json:"address"`
}

func minerAddress(index int) (string, error) {
	file := keyDir + fmt.Sprintf("key%d.keystore", index+1)
	account, err := wallet.NewAccountFromFile(file)
	if err != nil {
		return "", err
	}
	return account.Addr(), nil
}

//func newAccount() (string, string, error) {
//	nodeName := "boxd_p1_1"
//	args := []string{"exec", nodeName, "./newaccount.sh"}
//	cmd := exec.Command("docker", args...)
//	stdout, err := cmd.StdoutPipe()
//	if err != nil {
//		return "", "", err
//	}
//	if err := cmd.Start(); err != nil {
//		return "", "", err
//	}
//	var buf bytes.Buffer
//	_, err = buf.ReadFrom(stdout)
//	if err != nil {
//		return "", "", err
//	}
//	if err := cmd.Wait(); err != nil {
//		return "", "", err
//	}
//	addr := GetIniKV(buf.String(), "Address")
//	if addr == "" {
//		return "", "", errors.New("newAccount failed, address is empty")
//	}
//	acc := GetIniKV(buf.String(), "Created new account")
//	if addr == "" {
//		return "", "", errors.New("newAccount failed, account is empty")
//	}
//	return acc, addr, nil
//}

func balanceFor(accAddr string, peerAddr string) uint64 {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	rpcClient := rpcpb.NewTransactionCommandClient(conn)
	r, err := rpcClient.GetBalance(ctx, &rpcpb.GetBalanceRequest{
		Addrs: []string{accAddr}})
	if err != nil {
		logger.Panicf("balance for %s on peer %s error: %s", accAddr, peerAddr, err)
	}
	return r.Balances[accAddr]
}

func balancesFor(peerAddr string, addresses ...string) ([]uint64, error) {
	var bb []uint64
	for _, a := range addresses {
		amount := balanceFor(a, peerAddr)
		bb = append(bb, amount)
	}
	return bb, nil
}

func unlockAccount(addr string) *wallet.Account {
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

func execTx(account *wallet.Account, fromAddr string, toAddrs []string,
	amounts []uint64, peerAddr string) {
	//
	if len(toAddrs) != len(amounts) {
		logger.Panicf("toAddrs count %d is mismatch with amounts count: %d",
			len(toAddrs), len(amounts))
	}
	//
	fromAddress, err := types.NewAddress(fromAddr)
	if err != nil {
		logger.Panicf("NewAddress fromAddr: %s error: %s", fromAddr, err)
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

	if _, err := client.CreateTransaction(conn, fromAddress, addrAmountMap,
		account.PublicKey(), account); err != nil {
		logger.Panicf("create transaction from %s, addr amont map %v, error: %s",
			fromAddress, addrAmountMap, err)
	}
}

func txCountFor(accAddr string, peerAddr string) int {
	b := balanceFor(accAddr, peerAddr)
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	addr, err := types.NewAddress(accAddr)
	if err != nil {
		logger.Panicf("NewAddress addrs: %s error: %s", addr, err)
	}
	logger.Infof("fund transaction for %s balance %d", addr, b)
	r, err := client.FundTransaction(conn, addr, b)
	if err != nil {
		logger.Panic(err)
	}
	return int(r.Count)
	//return len(r.Utxos)
}

func chainHeightFor(peerAddr string) (int, error) {
	// create grpc conn
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// call rpc interface
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	rpcClient := rpcpb.NewContorlCommandClient(conn)

	r, err := rpcClient.GetBlockHeight(ctx, &rpcpb.GetBlockHeightRequest{})
	if err != nil {
		return 0, err
	}
	return int(r.Height), nil
}

func waitHeightSame() (int, error) {
	timeout := 30
	for i := 0; i < timeout; i++ {
		var hh []int
		for j := 0; j < peerCount; j++ {
			h, err := chainHeightFor(peersAddr[j])
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

func allMinersAddr() []string {
	var addresses []string
	for i := 0; i < peerCount; i++ {
		addr, err := minerAddress(i)
		if err != nil {
			logger.Panic(err)
		}
		addresses = append(addresses, addr)
	}
	return addresses
}

func genTestAddr(count int) ([]string, []string) {
	logger.Infof("start to create %d accounts", count)
	var addresses, accounts []string
	for i := 0; i < count; i++ {
		var (
			acc, addr string
			err       error
		)
		//if *enableDocker {
		//	acc, addr, err = newAccount()
		//} else {
		//	acc, addr, err = newAccountFromWallet()
		//}
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
