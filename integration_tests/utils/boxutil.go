// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/wallet"
	lru "github.com/hashicorp/golang-lru"
	"google.golang.org/grpc"
)

const (
	walletDir         = "./.devconfig/ws1/box_keystore/"
	dockerComposeFile = "../docker/docker-compose.yml"
	testPassphrase    = "1"

	// RPCTimeout defines rpc timeout
	RPCTimeout = 5 * time.Second
	// RPCInterval defines rpc query interval
	RPCInterval = 300 * time.Millisecond

	tokenBoxAmt = 1
	tokenTxFee  = 100
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
func WaitTokenBalanceEnough(addr string, amount uint64, tokenID *types.OutPoint,
	checkPeer string, timeout time.Duration) (uint64, error) {
	// return eagerly
	b := TokenBalanceFor(addr, tokenID, checkPeer)
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
			b = TokenBalanceFor(addr, tokenID, checkPeer)
			if b >= amount {
				return b, nil
			}
		}
	}
	return b, fmt.Errorf("Timeout for waiting for %s token balance enough %d, now %d",
		addr, amount, b)
}

// WaitTokenBalanceEqualTo wait token balance of addr equal to amount
func WaitTokenBalanceEqualTo(addr string, amount uint64, tokenID *types.OutPoint,
	checkPeer string, timeout time.Duration) error {
	// return eagerly
	b := TokenBalanceFor(addr, tokenID, checkPeer)
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
			b = TokenBalanceFor(addr, tokenID, checkPeer)
			if b == amount {
				return nil
			}
		}
	}
	return fmt.Errorf("Timeout for waiting for %s token balance enough %d, now %d",
		addr, amount, b)
}

// TokenBalanceFor get token balance of addr
func TokenBalanceFor(addr string, tokenID *types.OutPoint, peerAddr string) uint64 {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	// get balance
	b, err := client.GetTokenBalance(conn, addr, tokenID.Hash, tokenID.Index)
	if err != nil {
		logger.Panic(err)
	}
	return b
}

// NewTx new a tx and return change utxo
func NewTx(fromAcc *wallet.Account, toAddrs []string, amounts []uint64,
	peerAddr string) (tx *types.Transaction, change *rpcpb.Utxo, fee uint64,
	err error) {
	// calc fee
	amount := uint64(0)
	for _, a := range amounts {
		amount += a
	}
	if amount >= 10000 {
		fee = amount / 10000
	}
	tx, change, err = NewTxWithFee(fromAcc, toAddrs, amounts, fee, peerAddr)
	return
}

// NewTxWithFee new a tx and return change utxo
func NewTxWithFee(fromAcc *wallet.Account, toAddrs []string, amounts []uint64,
	fee uint64, peerAddr string) (tx *types.Transaction, change *rpcpb.Utxo, err error) {
	// calc amount
	amount := uint64(0)
	for _, a := range amounts {
		amount += a
	}
	// get utxos
	utxos, err := fetchUtxos(fromAcc.Addr(), amount+fee, peerAddr)
	if err != nil {
		return
	}
	// NOTE: for test only
	//checkDuplicateUtxos(utxos)
	// calc change amount
	total := uint64(0)
	for _, u := range utxos {
		total += u.GetTxOut().GetValue()
	}
	changeAmt := total - amount - fee
	if changeAmt >= total {
		err = fmt.Errorf("invalid arguments, utxo total=%d, amount=%d, fee=%d, "+
			"changeAmt=%d", total, amount, fee, changeAmt)
		return
	}
	//
	tx, change, err = txlogic.NewTxWithUtxos(fromAcc, utxos, toAddrs, amounts, changeAmt)
	return
}

// NewTxs construct some transactions
func NewTxs(fromAcc *wallet.Account, toAddr string, count int, peerAddr string) (
	txss [][]*types.Transaction, transfer, totalFee uint64, num int, err error) {
	// get utxoes
	utxos, err := fetchUtxos(fromAcc.Addr(), 0, peerAddr)
	if err != nil {
		return
	}
	if len(utxos) == 0 {
		err = fmt.Errorf("no utxos")
		return
	}
	// NOTE: for test only
	//checkDuplicateUtxos(utxos)
	// gen txs
	txss = make([][]*types.Transaction, 0)
	n := (count + len(utxos) - 1) / len(utxos)

	transfer, totalFee, num = uint64(0), uint64(0), 0
	for _, u := range utxos {
		change := u
		value := change.GetTxOut().GetValue()
		aveAmt := value / uint64(n)
		if aveAmt == 0 {
			continue
		}
		changeAmt := value
		txs := make([]*types.Transaction, 0)
		fee := uint64(0)
		for j := n; num < count && j > 0; j-- {
			if aveAmt >= 10000 {
				fee = uint64(rand.Int63n(int64(aveAmt) / 10000))
			}
			amount := aveAmt - fee
			changeAmt = changeAmt - aveAmt
			tx := new(types.Transaction)
			tx, change, err = txlogic.NewTxWithUtxos(fromAcc, []*rpcpb.Utxo{change}, []string{toAddr},
				[]uint64{amount}, changeAmt)
			if err != nil {
				return
			}
			txs = append(txs, tx)
			transfer += amount
			totalFee += fee
			num++
		}
		txss = append(txss, txs)
	}
	return txss, transfer, totalFee, num, nil
}

var utxoCache *lru.Cache

func init() {
	utxoCache, _ = lru.New(1000000)
}

func checkDuplicateUtxos(utxos []*rpcpb.Utxo) {
	for _, u := range utxos {
		op := new(types.OutPoint)
		if err := op.FromProtoMessage(u.OutPoint); err != nil {
			logger.Panic(err)
		}
		if ok := utxoCache.Contains(*op); ok {
			logger.Errorf("duplicate outpoint for utxo: %v", u)
			os.Exit(1)
		}
	}
}

func addUtxoToCache(vins []*types.TxIn) {
	for _, in := range vins {
		if ok, _ := utxoCache.ContainsOrAdd(in.PrevOutPoint, struct{}{}); ok {
			logger.Errorf("duplicate outpoint for vin: %v", in)
			os.Exit(1)
		}
	}
}

func fetchUtxos(addr string, amount uint64, peerAddr string) ([]*rpcpb.Utxo, error) {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	utxos, err := client.FetchUtxos(conn, addr, amount)
	if err != nil {
		return nil, err
	}
	if len(utxos) == 0 {
		return nil, fmt.Errorf("FundTransaction fetch 0 utxos")
	}
	return utxos, err
}

// IssueTokenTx issues some token
func IssueTokenTx(acc *wallet.Account, toAddr string, tag *txlogic.TokenTag, totalSupply uint64,
	peerAddr string) *types.OutPoint {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()
	// create
	fromAddress, err1 := types.NewAddress(acc.Addr())
	toAddress, err2 := types.NewAddress(toAddr)
	if err1 != nil || err2 != nil {
		logger.Panicf("%v, %v", err1, err2)
	}
	tx, err := client.CreateTokenIssueTx(conn, fromAddress, toAddress, acc.PublicKey(),
		tag.Name, tag.Symbol, uint64(totalSupply), tag.Decimal, acc)
	if err != nil {
		logger.Panic(err)
	}
	txHash, _ := tx.CalcTxHash()
	return txlogic.NewOutPoint(txHash, 0)
}

// NewTokenTx new a token tx
func NewTokenTx(acc *wallet.Account, toAddrs []string, amounts []uint64,
	tokenID *types.OutPoint, peerAddr string) (*types.Transaction, *rpcpb.Utxo,
	*rpcpb.Utxo, error) {
	fee := uint64(1000)
	amount := fee + uint64(len(toAddrs)*tokenBoxAmt)
	amountT := uint64(0)
	for _, a := range amounts {
		amountT += a
	}
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return nil, nil, nil, err
	}
	defer conn.Close()
	fromAddress, _ := types.NewAddress(acc.Addr())
	resp, err := client.FundTokenTransaction(conn, fromAddress, tokenID, amount, amountT)
	if err != nil {
		return nil, nil, nil, err
	}
	return NewTokenTxWithUtxos(acc, toAddrs, amounts, tokenID, resp.GetUtxos())
}

// NewTokenTxs new a token tx
func NewTokenTxs(acc *wallet.Account, toAddr string, amountT uint64, count int,
	tokenID *types.OutPoint, peerAddr string) ([]*types.Transaction, error) {
	// get utxo for some amount box and token
	amount := (tokenTxFee + tokenBoxAmt) * uint64(count)
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	fromAddress, _ := types.NewAddress(acc.Addr())
	resp, err := client.FundTokenTransaction(conn, fromAddress, tokenID, amount,
		amountT)
	if err != nil {
		return nil, err
	}
	//
	var (
		txs   []*types.Transaction
		utxos []*rpcpb.Utxo
	)
	unitT := amountT / uint64(count)
	utxos = resp.GetUtxos()
	for i := 0; i < count; i++ {
		if i == count-1 {
			unitT = amountT - unitT*uint64(i)
		}
		tx, change, changeT, err := NewTokenTxWithUtxos(acc, []string{toAddr},
			[]uint64{unitT}, tokenID, utxos)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
		utxos = []*rpcpb.Utxo{change, changeT}
	}
	return txs, nil
}

// NewTokenTxWithUtxos new a token tx
func NewTokenTxWithUtxos(acc *wallet.Account, toAddrs []string, amounts []uint64,
	tokenID *types.OutPoint, utxos []*rpcpb.Utxo) (*types.Transaction,
	*rpcpb.Utxo, *rpcpb.Utxo, error) {
	// check amount
	val, valT := uint64(0), uint64(0)
	for _, u := range utxos {
		outpoint, value, ok := txlogic.ExtractTokenInfo(u)
		if !ok {
			val += u.GetTxOut().GetValue()
		} else {
			if *tokenID != *outpoint {
				return nil, nil, nil, errors.New("token outpoint mismatch")
			}
			valT += value
		}
	}
	remain := val - uint64(len(toAddrs)) - tokenTxFee
	if remain > val {
		return nil, nil, nil, errors.New("insuffience box")
	}
	total := uint64(0)
	for _, a := range amounts {
		total += a
	}
	remainT := valT - total
	if remainT > valT {
		return nil, nil, nil, errors.New("insuffience token")
	}

	// construct transaction
	tx := new(types.Transaction)
	// vin
	for _, utxo := range utxos {
		tx.Vin = append(tx.Vin, txlogic.MakeVin(utxo, 0))
	}
	// vout for toAddrs
	for i, toAddr := range toAddrs {
		tx.Vout = append(tx.Vout, txlogic.MakeTokenVout(toAddr, tokenID, amounts[i]))
	}
	// vout for change of fromAddress
	changeIdx, changeTIdx := uint32(0), uint32(0)
	if remain > 0 {
		tx.Vout = append(tx.Vout, txlogic.MakeVout(acc.Addr(), remain))
		changeIdx = uint32(len(tx.Vout)) - 1
	}
	// vout for token change of fromAddress
	if remainT > 0 {
		tx.Vout = append(tx.Vout, txlogic.MakeTokenVout(acc.Addr(), tokenID, remainT))
		changeTIdx = uint32(len(tx.Vout)) - 1
	}

	// sign vin
	if err := txlogic.SignTxWithUtxos(tx, utxos, acc); err != nil {
		return nil, nil, nil, err
	}

	// construct change and token change utxo
	var change, changeT *rpcpb.Utxo
	txHash, _ := tx.TxHash()
	if remain > 0 {
		change = &rpcpb.Utxo{
			OutPoint:    txlogic.NewPbOutPoint(txHash, changeIdx),
			TxOut:       txlogic.MakeVout(acc.Addr(), remain),
			BlockHeight: 0,
			IsCoinbase:  false,
			IsSpent:     false,
		}
	}
	if remainT > 0 {
		changeT = &rpcpb.Utxo{
			OutPoint:    txlogic.NewPbOutPoint(txHash, changeTIdx),
			TxOut:       txlogic.MakeTokenVout(acc.Addr(), tokenID, remainT),
			BlockHeight: 0,
			IsCoinbase:  false,
			IsSpent:     false,
		}
	}

	return tx, change, changeT, nil
}

// NewSplitAddrTxWithFee new split address tx
func NewSplitAddrTxWithFee(acc *wallet.Account, addrs []string, weights []uint64,
	fee uint64, peerAddr string) (tx *types.Transaction, change *rpcpb.Utxo,
	splitAddr string, err error) {
	// get utxos
	utxos, err := fetchUtxos(acc.Addr(), fee, peerAddr)
	if err != nil {
		return
	}
	return txlogic.NewSplitAddrTxWithUtxos(acc, addrs, weights, utxos, fee, peerAddr)
}
