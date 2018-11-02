// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
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

func newAccount() (string, error) {
	nodeName := "boxd_p1_1"
	args := []string{"exec", nodeName, "./newaccount.sh"}
	cmd := exec.Command("docker", args...)
	logger.Infof("exec: docker %v", args)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(stdout)
	if err != nil {
		return "", err
	}
	//logger.Infof("newAccount from container %s return: %s", nodeName, buf.String())
	if err := cmd.Wait(); err != nil {
		return "", err
	}
	addr := GetIniKV(buf.String(), "Address")
	if addr == "" {
		return "", errors.New("newAccount failed, address is empty")
	}
	return addr, nil
}

func balanceFor(accAddr string, peerAddr string) (uint64, error) {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	rpcClient := rpcpb.NewTransactionCommandClient(conn)
	r, err := rpcClient.GetBalance(ctx, &rpcpb.GetBalanceRequest{
		Addrs: []string{accAddr}})
	if err != nil {
		return 0, err
	}
	return r.Balances[accAddr], nil
}

func balancesFor(peerAddr string, addresses ...string) ([]uint64, error) {
	var bb []uint64
	for _, a := range addresses {
		amount, err := balanceFor(a, peerAddr)
		if err != nil {
			return nil, err
		}
		bb = append(bb, amount)
	}
	return bb, nil
}

func execTx(fromAddr, toAddr string, amount uint64, peerAddr string) error {
	fromAddress, err := types.NewAddress(fromAddr)
	if err != nil {
		return fmt.Errorf("NewAddress fromAddr: %s error: %s", fromAddr, err)
	}

	// get account of sender
	wltMgr, err := wallet.NewWalletManager(walletDir)
	if err != nil {
		return err
	}
	account, exists := wltMgr.GetAccount(fromAddr)
	if !exists {
		return fmt.Errorf("Account %s not managed", fromAddr)
	}
	if err := account.UnlockWithPassphrase(testPassphrase); err != nil {
		return fmt.Errorf("Fail to unlock account: %v, error: %s", account, err)
	}

	// initialize rpc
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()
	toAddress, err := types.NewAddress(toAddr)
	if err != nil {
		return fmt.Errorf("NewAddress toAddr: %s error: %s", toAddr, err)
	}
	addrAmountMap := map[types.Address]uint64{toAddress: amount}
	if _, err := client.CreateTransaction(conn, fromAddress, addrAmountMap,
		account.PublicKey(), account); err != nil {
		panic(err)
	}
	return nil
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
			logger.Fatal(err)
		}
		addresses = append(addresses, addr)
	}
	return addresses
}

func genTestAddr(count int) []string {
	var addresses []string
	for i := 0; i < count; i++ {
		addr, err := newAccount()
		if err != nil {
			logger.Fatal(err)
		}
		addresses = append(addresses, addr)
	}
	return addresses
}
