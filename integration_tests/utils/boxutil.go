// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"runtime/debug"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/rpc/client"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
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

	tokenBoxAmt = 1
	tokenTxFee  = 100
)

var logger = log.NewLogger("integration_utils") // logger

type sortByUTXOValue []*rpcpb.Utxo

func (x sortByUTXOValue) Len() int           { return len(x) }
func (x sortByUTXOValue) Less(i, j int) bool { return x[i].TxOut.Value < x[j].TxOut.Value }
func (x sortByUTXOValue) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// KeyStore defines key structure
type KeyStore struct {
	Address string `json:"address"`
}

// TokenTag defines token tag
type TokenTag struct {
	Name    string
	Symbol  string
	Decimal uint8
}

// NewTokenTag news a TokenTag
func NewTokenTag(name, sym string, decimal uint8) *TokenTag {
	return &TokenTag{
		Name:    name,
		Symbol:  sym,
		Decimal: decimal,
	}
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
	if time.Since(start) > 2*RPCInterval {
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

// ExecTxNoPanic execute a transaction
func ExecTxNoPanic(account *wallet.Account, toAddrs []string, amounts []uint64,
	peerAddr string) (*types.Transaction, error) {
	//
	if len(toAddrs) != len(amounts) {
		logger.Panicf("toAddrs count %d is mismatch with amounts count: %d",
			len(toAddrs), len(amounts))
	}
	//
	fromAddress, err := types.NewAddress(account.Addr())
	if err != nil {
		return nil, fmt.Errorf("NewAddress fromAddr: %s error: %s", account.Addr(), err)
	}

	// initialize rpc
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	// make toAddr map
	addrAmountMap := make(map[types.Address]uint64, len(toAddrs))
	for i := 0; i < len(toAddrs); i++ {
		toAddress, err := types.NewAddress(toAddrs[i])
		if err != nil {
			return nil, fmt.Errorf("NewAddress toAddrs: %s error: %s", toAddrs, err)
		}
		addrAmountMap[toAddress] = amounts[i]
	}

	start := time.Now()
	tx, err := client.CreateTransaction(conn, fromAddress, addrAmountMap,
		account.PublicKey(), account, nil, nil)
	if time.Since(start) > 3*RPCInterval {
		logger.Warnf("cost %v for CreateTransaction on %s", time.Since(start), peerAddr)
	}
	if err != nil {
		return nil, fmt.Errorf("create transaction from %s, addr amont map %v, error: %s",
			fromAddress, addrAmountMap, err)
	}
	return tx, nil
}

// ExecTx execute a transaction
func ExecTx(account *wallet.Account, toAddrs []string, amounts []uint64,
	peerAddr string) *types.Transaction {

	tx, err := ExecTxNoPanic(account, toAddrs, amounts, peerAddr)
	if err != nil {
		debug.PrintStack()
		logger.Panic(err)
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
func TokenBalanceFor(addr string, tokenID *types.OutPoint, peerAddr string) uint64 {
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
	b, err := client.GetTokenBalance(conn, address, tokenID.Hash, tokenID.Index)
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
	// calc change amount
	total := uint64(0)
	for _, u := range utxos {
		total += u.GetTxOut().GetValue()
	}
	changeAmt := total - amount - fee
	//
	tx, change, err = NewTxWithUtxo(fromAcc, utxos, toAddrs, amounts, changeAmt)
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
			tx, change, err = NewTxWithUtxo(fromAcc, []*rpcpb.Utxo{change}, []string{toAddr},
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

// NewTxWithUtxo new a transaction
func NewTxWithUtxo(fromAcc *wallet.Account, utxos []*rpcpb.Utxo, toAddrs []string,
	amounts []uint64, changeAmt uint64) (*types.Transaction, *rpcpb.Utxo, error) {

	utxoValue := uint64(0)
	for _, u := range utxos {
		utxoValue += u.GetTxOut().GetValue()
	}
	amount := uint64(0)
	for _, a := range amounts {
		amount += a
	}
	if utxoValue < amount+changeAmt {
		return nil, nil, fmt.Errorf("input %d is less than output %d",
			utxoValue, amount+changeAmt)
	}

	// vin
	vins := make([]*types.TxIn, 0)
	for _, utxo := range utxos {
		vins = append(vins, makeVin(utxo, 0))
	}

	// vout for toAddrs
	vouts := make([]*corepb.TxOut, 0, len(toAddrs))
	for i, toAddr := range toAddrs {
		vouts = append(vouts, makeVout(toAddr, amounts[i]))
	}

	// vout for change of fromAddress
	fromAddrOut := makeVout(fromAcc.Addr(), changeAmt)

	// construct transaction
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, vouts...)
	tx.Vout = append(tx.Vout, fromAddrOut)

	// sign vin
	if err := signTx(tx, utxos, fromAcc); err != nil {
		return nil, nil, err
	}

	// create change utxo
	txHash, _ := tx.TxHash()
	change := &rpcpb.Utxo{
		OutPoint:    NewPbOutPoint(txHash, uint32(len(tx.Vout))-1),
		TxOut:       fromAddrOut,
		BlockHeight: 0,
		IsCoinbase:  false,
		IsSpent:     false,
	}

	return tx, change, nil
}

// NewPbOutPoint constructs a OutPoint
func NewPbOutPoint(hash *crypto.HashType, index uint32) *corepb.OutPoint {
	return &corepb.OutPoint{
		Hash:  (*hash)[:],
		Index: index,
	}
}

// NewOutPoint constructs a OutPoint
func NewOutPoint(hash *crypto.HashType, index uint32) *types.OutPoint {
	return &types.OutPoint{
		Hash:  *hash,
		Index: index,
	}
}

func fetchUtxos(addr string, amount uint64, peerAddr string) (utxos []*rpcpb.Utxo, err error) {
	// get utxoes
	fromAddress, _ := types.NewAddress(addr)
	//totalAmount, err := balanceNoPanicFor(addr, peerAddr)
	//if err != nil {
	//	return
	//}
	//if amount < totalAmount && amount != 0 {
	//	totalAmount = amount
	//}
	if amount == 0 {
		amount = 100000000
	}
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return
	}
	defer conn.Close()
	var utxoResponse *rpcpb.ListUtxosResponse
	for amount > 0 {
		utxoResponse, err = client.FundTransaction(conn, fromAddress, amount)
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "Not enough balance") {
			amount -= amount / 16
			continue
		}
		return
	}
	if utxoResponse == nil || amount == 0 {
		err = fmt.Errorf("FundTransaction fetch 0 utxos")
		return
	}
	//bytes, _ := json.MarshalIndent(utxoResponse.GetUtxos(), "", "  ")
	//logger.Infof("utxos: %s", string(bytes))
	return utxoResponse.GetUtxos(), nil
}

// IssueTokenTx issues some token
func IssueTokenTx(acc *wallet.Account, toAddr string, tag *TokenTag, totalSupply uint64,
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
	return NewOutPoint(txHash, 0)
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
		outpoint, value, ok := ExtractTokenInfo(u)
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
		tx.Vin = append(tx.Vin, makeVin(utxo, 0))
	}
	// vout for toAddrs
	for i, toAddr := range toAddrs {
		tx.Vout = append(tx.Vout, makeTokenVout(toAddr, tokenID, amounts[i]))
	}
	// vout for change of fromAddress
	changeIdx, changeTIdx := uint32(0), uint32(0)
	if remain > 0 {
		tx.Vout = append(tx.Vout, makeVout(acc.Addr(), remain))
		changeIdx = uint32(len(tx.Vout)) - 1
	}
	// vout for token change of fromAddress
	if remainT > 0 {
		tx.Vout = append(tx.Vout, makeTokenVout(acc.Addr(), tokenID, remainT))
		changeTIdx = uint32(len(tx.Vout)) - 1
	}

	// sign vin
	if err := signTx(tx, utxos, acc); err != nil {
		return nil, nil, nil, err
	}

	// construct change and token change utxo
	var change, changeT *rpcpb.Utxo
	txHash, _ := tx.TxHash()
	if remain > 0 {
		change = &rpcpb.Utxo{
			OutPoint:    NewPbOutPoint(txHash, changeIdx),
			TxOut:       makeVout(acc.Addr(), remain),
			BlockHeight: 0,
			IsCoinbase:  false,
			IsSpent:     false,
		}
	}
	if remainT > 0 {
		changeT = &rpcpb.Utxo{
			OutPoint:    NewPbOutPoint(txHash, changeTIdx),
			TxOut:       makeTokenVout(acc.Addr(), tokenID, remainT),
			BlockHeight: 0,
			IsCoinbase:  false,
			IsSpent:     false,
		}
	}

	return tx, change, changeT, nil
}

// ExtractTokenInfo extract token info from a utxo
func ExtractTokenInfo(utxo *rpcpb.Utxo) (*types.OutPoint, uint64, bool) {
	script := script.NewScriptFromBytes(utxo.TxOut.ScriptPubKey)
	if script.IsTokenIssue() {
		issueParam, err := script.GetIssueParams()
		if err == nil {
			outHash := crypto.HashType{}
			outHash.SetBytes(utxo.OutPoint.Hash)
			return &types.OutPoint{Hash: outHash, Index: utxo.OutPoint.Index},
				issueParam.TotalSupply * uint64(math.Pow10(int(issueParam.Decimals))),
				true
		}
	}
	if script.IsTokenTransfer() {
		transferParam, err := script.GetTransferParams()
		if err == nil {
			return &transferParam.OutPoint, transferParam.Amount, true
		}
	}
	return nil, 0, false
}

func makeVout(addr string, amount uint64) *corepb.TxOut {
	address, _ := types.NewAddress(addr)
	addrPkh, _ := types.NewAddressPubKeyHash(address.Hash())
	addrScript := *script.PayToPubKeyHashScript(addrPkh.Hash())
	return &corepb.TxOut{
		Value:        amount,
		ScriptPubKey: addrScript,
	}
}

func makeTokenVout(addr string, tokenID *types.OutPoint, amount uint64) *corepb.TxOut {
	address, _ := types.NewAddress(addr)
	addrPkh, _ := types.NewAddressPubKeyHash(address.Hash())
	transferParams := &script.TransferParams{}
	transferParams.Hash = tokenID.Hash
	transferParams.Index = tokenID.Index
	transferParams.Amount = amount
	addrScript := *script.TransferTokenScript(addrPkh.Hash(), transferParams)
	return &corepb.TxOut{Value: tokenBoxAmt, ScriptPubKey: addrScript}
}

func makeSplitAddrVout(addrs []string, weights []uint64) *corepb.TxOut {
	return &corepb.TxOut{
		Value:        0,
		ScriptPubKey: MakeSplitAddrPubkey(addrs, weights),
	}
}

func makeVin(utxo *rpcpb.Utxo, seq uint32) *types.TxIn {
	var hash crypto.HashType
	copy(hash[:], utxo.GetOutPoint().Hash)
	return &types.TxIn{
		PrevOutPoint: types.OutPoint{
			Hash:  hash,
			Index: utxo.GetOutPoint().GetIndex(),
		},
		ScriptSig: []byte{},
		Sequence:  seq,
	}
}

func signTx(tx *types.Transaction, utxos []*rpcpb.Utxo, acc *wallet.Account) error {
	for i, utxo := range utxos {
		scriptPkBytes := utxo.GetTxOut().GetScriptPubKey()
		sigHash, err := script.CalcTxHashForSig(scriptPkBytes, tx, i)
		if err != nil {
			return err
		}
		sig, err := acc.Sign(sigHash)
		if err != nil {
			return err
		}
		scriptSig := script.SignatureScript(sig, acc.PublicKey())
		tx.Vin[i].ScriptSig = *scriptSig
	}
	return nil
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
	return NewSplitAddrTxWithUtxos(acc, addrs, weights, utxos, fee, peerAddr)
}

// NewSplitAddrTxWithUtxos new split address tx
func NewSplitAddrTxWithUtxos(acc *wallet.Account, addrs []string, weights []uint64,
	utxos []*rpcpb.Utxo, fee uint64, peerAddr string) (tx *types.Transaction,
	change *rpcpb.Utxo, splitAddr string, err error) {

	utxoValue := uint64(0)
	for _, u := range utxos {
		utxoValue += u.GetTxOut().GetValue()
	}
	changeAmt := utxoValue - fee

	// vin
	vins := make([]*types.TxIn, 0)
	for _, utxo := range utxos {
		vins = append(vins, makeVin(utxo, 0))
	}

	// vout for toAddrs
	splitAddrOut := makeSplitAddrVout(addrs, weights)
	changeOut := makeVout(acc.Addr(), changeAmt)

	// construct transaction
	tx = new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, splitAddrOut, changeOut)

	// sign vin
	if err = signTx(tx, utxos, acc); err != nil {
		return
	}

	// create change utxo
	txHash, _ := tx.TxHash()
	change = &rpcpb.Utxo{
		OutPoint:    NewPbOutPoint(txHash, uint32(len(tx.Vout))-1),
		TxOut:       changeOut,
		BlockHeight: 0,
		IsCoinbase:  false,
		IsSpent:     false,
	}

	splitAddr, err = MakeSplitAddr(addrs, weights)

	return
}

// MakeSplitAddrPubkey make split addr
func MakeSplitAddrPubkey(addrs []string, weights []uint64) []byte {
	addresses := make([]types.Address, len(addrs))
	for i, addr := range addrs {
		addresses[i], _ = types.NewAddress(addr)
	}
	return *script.SplitAddrScript(addresses, weights)
}

// MakeSplitAddr make split addr
func MakeSplitAddr(addrs []string, weights []uint64) (string, error) {
	pk := MakeSplitAddrPubkey(addrs, weights)
	splitAddrScriptStr := script.NewScriptFromBytes(pk).Disasm()
	s := strings.Split(splitAddrScriptStr, " ")
	pubKeyHash, err := hex.DecodeString(s[1])
	if err != nil {
		return "", err
	}
	addr, err := types.NewAddressPubKeyHash(pubKeyHash)
	if err != nil {
		return "", err
	}
	return addr.String(), nil
}
