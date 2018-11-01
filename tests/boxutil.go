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

func balanceFor(accAddr string, nodeAddr string) (uint64, error) {
	conn, err := grpc.Dial(nodeAddr, grpc.WithInsecure())
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
