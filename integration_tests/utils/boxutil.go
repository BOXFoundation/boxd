// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
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

// BalanceNoPanicFor returns accAddr's balance without panic
func BalanceNoPanicFor(accAddr string, conn *grpc.ClientConn) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), RPCTimeout)
	defer cancel()
	rpcClient := rpcpb.NewTransactionCommandClient(conn)
	start := time.Now()
	r, err := rpcClient.GetBalance(ctx, &rpcpb.GetBalanceReq{Addrs: []string{accAddr}})
	if time.Since(start) > 2*RPCInterval {
		logger.Warnf("cost %v for GetBalance", time.Since(start))
	}
	if err != nil {
		return 0, err
	} else if r.Code != 0 {
		return 0, errors.New(r.Message)
	}
	return r.Balances[0], nil
}

// BalanceFor get balance of accAddr
func BalanceFor(accAddr string, conn *grpc.ClientConn) uint64 {
	b, err := BalanceNoPanicFor(accAddr, conn)
	if err != nil {
		logger.Panicf("balance for %s error: %s", accAddr, err)
	}
	return b
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

// WaitBalanceEnough wait balance of addr is more than amount
func WaitBalanceEnough(addr string, amount uint64, conn *grpc.ClientConn,
	timeout time.Duration) (uint64, error) {
	// return eagerly
	b, err := BalanceNoPanicFor(addr, conn)
	if err != nil {
		return 0, err
	}
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
			b, err = BalanceNoPanicFor(addr, conn)
			if err != nil {
				return 0, err
			}
			if b >= amount {
				return b, nil
			}
		}
	}
	return b, fmt.Errorf("Timeout for waiting for %s balance enough %d, now %d",
		addr, amount, b)
}

// WaitBalanceEqual wait balance of addr is more than amount
func WaitBalanceEqual(addr string, amount uint64, conn *grpc.ClientConn,
	timeout time.Duration) (uint64, error) {
	// return eagerly
	b := BalanceFor(addr, conn)
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
			b = BalanceFor(addr, conn)
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
	addr string, amount uint64, thash string, idx uint32, conn *grpc.ClientConn,
	timeout time.Duration,
) (uint64, error) {
	// return eagerly
	b := TokenBalanceFor(addr, thash, idx, conn)
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
			b = TokenBalanceFor(addr, thash, idx, conn)
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
	addr string, amount uint64, thash string, idx uint32, conn *grpc.ClientConn,
	timeout time.Duration,
) error {
	// return eagerly
	b := TokenBalanceFor(addr, thash, idx, conn)
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
			b = TokenBalanceFor(addr, thash, idx, conn)
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
	addr string, tokenHash string, tokenIndex uint32, conn *grpc.ClientConn,
) uint64 {
	// get balance
	b, err := rpcutil.GetTokenBalance(conn, []string{addr}, tokenHash, tokenIndex)
	if err != nil {
		logger.Panic(err)
	}
	return b[0]
}

// CallContract calls contract with code
func CallContract(addr, contractAddr string, code []byte, conn *grpc.ClientConn) []byte {
	client := rpcpb.NewWebApiClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req := &rpcpb.CallReq{From: addr, To: contractAddr, Data: hex.EncodeToString(code)}
	resp, err := client.DoCall(ctx, req)
	if err != nil {
		logger.Panic(err)
	} else if resp.Code != 0 {
		if strings.Contains(resp.Message, core.ErrContractNotFound.Error()) {
			return nil
		}
		logger.Panic(errors.New(resp.Message))
	}
	//logger.Infof("CallContract for %s contract addr %s output %s", addr, contractAddr, resp.Output)
	output, _ := hex.DecodeString(resp.Output)
	return output
}

func parseContractNumberResult(ret []byte) (uint64, error) {
	r, err := strconv.ParseUint(hex.EncodeToString(ret), 16, 64)
	if err != nil {
		return 0, err
	}
	return r, nil
}

// WaitERC20BalanceEqualTo waits ERC20 balance of addr equal to amount
func WaitERC20BalanceEqualTo(
	addr, contractAddr string, amount uint64, code []byte, conn *grpc.ClientConn,
	timeout time.Duration,
) error {
	ret := CallContract(addr, contractAddr, code, conn)
	b, _ := parseContractNumberResult(ret)
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
			ret = CallContract(addr, contractAddr, code, conn)
			b, _ = parseContractNumberResult(ret)
			if b == amount {
				return nil
			}
		}
	}
	return fmt.Errorf("Timeout for waiting for %s contract token balance equal to %d, now %d",
		addr, amount, b)
}

// WaitERC20AllowanceEqualTo waits ERC20 allowance of addr equal to amount
func WaitERC20AllowanceEqualTo(
	addr, contractAddr string, amount uint64, code []byte, conn *grpc.ClientConn,
	timeout time.Duration,
) error {
	return WaitERC20BalanceEqualTo(addr, contractAddr, amount, code, conn, timeout)
}

// NonceFor returns address' nonce in blockchain state
func NonceFor(addr string, conn *grpc.ClientConn) uint64 {
	client := rpcpb.NewWebApiClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.Nonce(ctx, &rpcpb.NonceReq{Addr: addr})
	if err != nil {
		logger.Panic(err)
	} else if resp.Code != 0 {
		logger.Panic(errors.New(resp.Message))
	}
	return resp.Nonce
}
