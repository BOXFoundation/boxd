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

func balanceFor(accAddr string, rpcAddr string) (uint64, error) {
	conn, err := grpc.Dial(rpcAddr, grpc.WithInsecure())
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rpcClient := rpcpb.NewTransactionCommandClient(conn)
	r, err := rpcClient.GetBalance(ctx, &rpcpb.GetBalanceRequest{Addr: accAddr})
	if err != nil {
		return 0, err
	}
	return r.Amount, nil
}

func execTx(fromAddr, toAddr string, amount uint64, rpcAddr string) error {
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
	conn, err := grpc.Dial(rpcAddr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()
	rpcClient := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// get the utxo of sender
	utxoResponse, err := rpcClient.FundTransaction(ctx,
		&rpcpb.FundTransactionRequest{
			Addr:   fromAddress.EncodeAddress(),
			Amount: amount,
		})
	if err != nil {
		return err
	}

	// wrap transaction
	utxos, err := client.SelectUtxo(utxoResponse, amount)
	if err != nil {
		return err
	}
	toAddress, err := types.NewAddress(toAddr)
	if err != nil {
		return fmt.Errorf("NewAddress toAddr: %s error: %s", toAddr, err)
	}
	tx, err := client.WrapTransaction(fromAddress.ScriptAddress(),
		toAddress.ScriptAddress(), account.PublicKey(), utxos,
		amount, account)
	if err != nil {
		return err
	}

	// send the transaction
	txReq := &rpcpb.SendTransactionRequest{Tx: tx}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	r, err := rpcClient.SendTransaction(ctx2, txReq)
	if err != nil {
		return fmt.Errorf("rpc send transaction error: %s", err)
	}
	logger.Infof("rpc send transaction result: %+v", r)
	return nil
}
