// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpcutil

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/pb"
	acc "github.com/BOXFoundation/boxd/wallet/account"
	"google.golang.org/grpc"
)

const (
	connTimeout = 30
	maxTokenFee = 100
)

// GetBalance returns total amount of an address
func GetBalance(conn *grpc.ClientConn, addresses []string) ([]uint64, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	req, err := c.GetBalance(ctx, &rpcpb.GetBalanceReq{Addrs: addresses})
	if err != nil {
		return nil, err
	}
	return req.GetBalances(), nil
}

// GetTokenBalance returns total amount of an address with specified token id
func GetTokenBalance(
	conn *grpc.ClientConn, addresses []string, tid *types.TokenID,
) ([]uint64, error) {

	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	if tid == nil {
		return nil, errors.New("token id cannot be empty")
	}
	pbTid := txlogic.ConvOutPoint((*types.OutPoint)(tid))
	req, err := c.GetTokenBalance(ctx, &rpcpb.GetTokenBalanceReq{
		Addrs: addresses, TokenID: pbTid})
	if err != nil {
		return nil, err
	}
	return req.GetBalances(), nil
}

func newFetchUtxosReq(addr string, amount uint64) *rpcpb.FetchUtxosReq {
	return &rpcpb.FetchUtxosReq{Addr: addr, Amount: amount}
}

// FetchUtxos fetch utxos from chain
func FetchUtxos(
	conn *grpc.ClientConn, addr string, amount uint64, tid *types.TokenID,
) ([]*rpcpb.Utxo, error) {

	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	req := newFetchUtxosReq(addr, amount)
	if tid != nil {
		req.TokenID = txlogic.ConvOutPoint((*types.OutPoint)(tid))
	}
	resp, err := c.FetchUtxos(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetUtxos(), nil
}

// GetFeePrice gets the recommended mining fee price according to recent packed transactions
func GetFeePrice(conn *grpc.ClientConn) (uint64, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	r, err := c.GetFeePrice(ctx, &rpcpb.GetFeePriceRequest{})
	return r.BoxPerByte, err
}

// NewIssueTokenTx new a issue token transaction
func NewIssueTokenTx(
	acc *acc.Account, to string, tag *types.TokenTag, supply uint64,
	conn *grpc.ClientConn,
) (*types.Transaction, *types.TokenID, *rpcpb.Utxo, error) {

	// fetch utxos for fee
	utxos, err := fetchUtxos(conn, acc.Addr(), maxTokenFee, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	inputAmt := uint64(0)
	for _, u := range utxos {
		inputAmt += u.GetTxOut().GetValue()
	}
	// fee
	fee := calcFee()
	//
	tx, tid, change, err := txlogic.NewIssueTokenTxWithUtxos(acc, utxos, to, tag,
		supply, inputAmt-fee)
	if err != nil {
		logger.Warnf("new issue token tx with utxos from %s to %s tag %+v "+
			"supply %d change %d with utxos: %+v error: %s", acc.Addr(), to, tag,
			supply, inputAmt-fee, utxos, err)
		return nil, nil, nil, err
	}
	return tx, tid, change, nil
}

// SendTransaction sends an signed transaction to node server through grpc connection
func SendTransaction(conn *grpc.ClientConn, tx *types.Transaction) (string, error) {
	txProtoMsg, err := tx.ToProtoMessage()
	if err != nil {
		return "", err
	}
	txPb, ok := txProtoMsg.(*corepb.Transaction)
	if !ok {
		return "", fmt.Errorf("can't convert transaction into protobuf")
	}
	txReq := &rpcpb.SendTransactionReq{Tx: txPb}

	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()

	resp, err := c.SendTransaction(ctx, txReq)
	if err != nil {
		return "", err
	}
	return resp.Hash, nil
}

// GetRawTransaction get the transaction info of given hash
func GetRawTransaction(conn *grpc.ClientConn, hash []byte) (*types.Transaction, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	logger.Debugf("Get transaction of hash: %x", hash)

	r, err := c.GetRawTransaction(ctx, &rpcpb.GetRawTransactionRequest{Hash: hash})
	if err != nil {
		return nil, err
	}
	tx := &types.Transaction{}
	err = tx.FromProtoMessage(r.Tx)
	return tx, err
}

// GetTransactionsInPool gets all transactions in memory pool
func GetTransactionsInPool(conn *grpc.ClientConn) ([]*types.Transaction, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	r, err := c.GetTransactionPool(ctx, &rpcpb.GetTransactionPoolRequest{})
	if err != nil {
		return nil, err
	}
	var txs []*types.Transaction
	for _, txMsg := range r.Txs {
		tx := &types.Transaction{}
		err := tx.FromProtoMessage(txMsg)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

// NewTx new a tx and return change utxo
func NewTx(fromAcc *acc.Account, toAddrs []string, amounts []uint64,
	conn *grpc.ClientConn) (tx *types.Transaction, change *rpcpb.Utxo, fee uint64,
	err error) {
	// calc fee
	amount := uint64(0)
	for _, a := range amounts {
		amount += a
	}
	if amount >= 10000 {
		fee = amount / 10000
	}
	tx, change, err = NewTxWithFee(fromAcc, toAddrs, amounts, fee, conn)
	return
}

// NewTxWithFee new a tx and return change utxo
func NewTxWithFee(fromAcc *acc.Account, toAddrs []string, amounts []uint64,
	fee uint64, conn *grpc.ClientConn) (tx *types.Transaction, change *rpcpb.Utxo, err error) {
	// calc amount
	amount := uint64(0)
	for _, a := range amounts {
		amount += a
	}
	// get utxos
	utxos, err := fetchUtxos(conn, fromAcc.Addr(), amount+fee, nil)
	if err != nil {
		err = fmt.Errorf("fetchUtxos error for %s amount %d: %s",
			fromAcc.Addr(), amount+fee, err)
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
func NewTxs(
	fromAcc *acc.Account, toAddr string, count int, conn *grpc.ClientConn,
) (txss [][]*types.Transaction, transfer, totalFee uint64, num int, err error) {

	// get utxoes
	utxos, err := fetchUtxos(conn, fromAcc.Addr(), 0, nil)
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
			tx, change, err = txlogic.NewTxWithUtxos(fromAcc, []*rpcpb.Utxo{change},
				[]string{toAddr}, []uint64{amount}, changeAmt)
			if err != nil {
				logger.Warn(err)
				continue
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

// MakeTxWithoutSign make a tx without signature
func MakeTxWithoutSign(
	wa service.WalletAgent, from string, to []string, amounts []uint64, fee uint64,
) (*types.Transaction, []*rpcpb.Utxo, error) {
	total := fee
	for _, a := range amounts {
		total += a
	}
	utxos, err := wa.Utxos(from, nil, total)
	uv := uint64(0)
	for _, u := range utxos {
		amount, _, err := txlogic.ParseUtxoAmount(u)
		if err != nil {
			return nil, nil, err
		}
		uv += amount
	}
	changeAmt := uv - total
	tx, err := txlogic.MakeTxWithoutSign(from, to, amounts, changeAmt, utxos...)
	return tx, utxos, err
}

func fetchUtxos(conn *grpc.ClientConn, addr string, amount uint64, tid *types.TokenID) (
	utxos []*rpcpb.Utxo, err error) {
	for t := 0; t < 30; t++ {
		utxos, err = FetchUtxos(conn, addr, amount, tid)
		if len(utxos) == 0 {
			err = fmt.Errorf("fetch no utxo for %s amount %d", addr, amount)
		}
		if err == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if err != nil {
		logger.Warn(err)
	}
	return utxos, err
}

// GetGRPCConn returns a conn with peer addr
func GetGRPCConn(peerAddr string) (*grpc.ClientConn, error) {
	return grpc.Dial(peerAddr, grpc.WithInsecure())
}

// NewTokenTx new a token tx
func NewTokenTx(
	acc *acc.Account, toAddrs []string, amounts []uint64, tid *types.TokenID,
	conn *grpc.ClientConn,
) (*types.Transaction, *rpcpb.Utxo, *rpcpb.Utxo, error) {

	fee := uint64(100)
	amount := fee
	amountT := uint64(0)
	for _, a := range amounts {
		amountT += a
	}
	boxUtxos, err := fetchUtxos(conn, acc.Addr(), amount, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	tokenUtxos, err := fetchUtxos(conn, acc.Addr(), amountT, tid)
	if err != nil {
		return nil, nil, nil, err
	}
	utxos := append(boxUtxos, tokenUtxos...)
	return NewTokenTxWithUtxos(acc, toAddrs, amounts, tid, utxos)
}

// NewTokenTxs new a token tx
func NewTokenTxs(
	acc *acc.Account, toAddr string, amountT uint64, count int, tid *types.TokenID,
	conn *grpc.ClientConn,
) ([]*types.Transaction, error) {
	// get utxos
	amount := maxTokenFee * uint64(count)
	boxUtxos, err := fetchUtxos(conn, acc.Addr(), amount, nil)
	if err != nil {
		return nil, err
	}
	tokenUtxos, err := fetchUtxos(conn, acc.Addr(), amountT, tid)
	if err != nil {
		return nil, err
	}
	utxos := append(boxUtxos, tokenUtxos...)
	//
	var txs []*types.Transaction
	unitT := amountT / uint64(count)
	for i := 0; i < count; i++ {
		if i == count-1 {
			unitT = amountT - unitT*uint64(i)
		}
		tx, change, changeT, err := NewTokenTxWithUtxos(acc, []string{toAddr},
			[]uint64{unitT}, tid, utxos)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
		utxos = []*rpcpb.Utxo{change, changeT}
	}
	return txs, nil
}

// NewTokenTxWithUtxos new a token tx
func NewTokenTxWithUtxos(acc *acc.Account, toAddrs []string, amounts []uint64,
	tid *types.TokenID, utxos []*rpcpb.Utxo) (*types.Transaction,
	*rpcpb.Utxo, *rpcpb.Utxo, error) {
	// check amount
	val, valT := uint64(0), uint64(0)

	for _, u := range utxos {
		amount, tidR, err := txlogic.ParseUtxoAmount(u)
		if err != nil {
			return nil, nil, nil, err
		}
		if tidR == nil {
			val += amount
		} else if tid != nil && *tidR == *tid {
			valT += amount
		} else {
			logger.Errorf("utxo: %+v have wrong token id", u)
		}
	}
	remain := val - uint64(len(toAddrs)) - maxTokenFee
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
		o, err := txlogic.MakeTokenVout(toAddr, tid, amounts[i])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("make token vout error %s", err)
		}
		tx.Vout = append(tx.Vout, o)
	}
	// vout for change of fromAddress
	changeIdx, changeTIdx := uint32(0), uint32(0)
	if remain > 0 {
		tx.Vout = append(tx.Vout, txlogic.MakeVout(acc.Addr(), remain))
		changeIdx = uint32(len(tx.Vout)) - 1
	}
	// vout for token change of fromAddress
	if remainT > 0 {
		o, err := txlogic.MakeTokenVout(acc.Addr(), tid, remainT)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("make token vout error %s", err)
		}
		tx.Vout = append(tx.Vout, o)
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
		o, err := txlogic.MakeTokenVout(acc.Addr(), tid, remainT)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("make token vout error %s", err)
		}
		changeT = &rpcpb.Utxo{
			OutPoint:    txlogic.NewPbOutPoint(txHash, changeTIdx),
			TxOut:       o,
			BlockHeight: 0,
			IsCoinbase:  false,
			IsSpent:     false,
		}
	}

	return tx, change, changeT, nil
}

// NewSplitAddrTxWithFee new split address tx
func NewSplitAddrTxWithFee(
	acc *acc.Account, addrs []string, weights []uint64, fee uint64, conn *grpc.ClientConn,
) (tx *types.Transaction, change *rpcpb.Utxo, splitAddr string, err error) {
	// get utxos
	utxos, err := fetchUtxos(conn, acc.Addr(), fee, nil)
	if err != nil {
		return
	}
	return txlogic.NewSplitAddrTxWithUtxos(acc, addrs, weights, utxos, fee)
}
