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
	conn *grpc.ClientConn, addresses []string, tid *txlogic.TokenID,
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
	conn *grpc.ClientConn, addr string, amount uint64, tid *txlogic.TokenID,
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
	acc *acc.Account, to string, tag *rpcpb.TokenTag, supply uint64,
	conn *grpc.ClientConn,
) (*types.Transaction, *txlogic.TokenID, *rpcpb.Utxo, error) {

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
	tx, tid, change, err := txlogic.NewTokenIssueTxWithUtxos(acc, to, tag,
		inputAmt-fee, utxos...)
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
	if resp.GetCode() != 0 {
		return "", errors.New(resp.GetMessage())
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
		err = fmt.Errorf("invalid arguments, addr %s utxo total=%d, amount=%d, "+
			"fee=%d, changeAmt=%d", fromAcc.Addr(), total, amount, fee, changeAmt)
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
			if change == nil {
				break
			}
		}
		txss = append(txss, txs)
	}
	return txss, transfer, totalFee, num, nil
}

func fetchUtxos(conn *grpc.ClientConn, addr string, amount uint64, tid *txlogic.TokenID) (
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
	acc *acc.Account, toAddrs []string, amounts []uint64, tid *txlogic.TokenID,
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
	return txlogic.NewTokenTransferTxWithUtxos(acc, toAddrs, amounts, tid, 0, utxos...)
}

// NewTokenTxs new a token tx
func NewTokenTxs(
	acc *acc.Account, toAddr string, amountT uint64, count int, tid *txlogic.TokenID,
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
	changeAmt := amount
	for i := 0; i < count; i++ {
		if i == count-1 {
			unitT = amountT - unitT*uint64(i)
		}
		changeAmt -= maxTokenFee
		tx, change, changeT, err := txlogic.NewTokenTransferTxWithUtxos(acc, []string{toAddr},
			[]uint64{unitT}, tid, changeAmt, utxos...)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
		utxos = []*rpcpb.Utxo{change, changeT}
		if change == nil {
			break
		}
	}
	return txs, nil
}

// NewSplitAddrTxWithFee new split address tx
func NewSplitAddrTxWithFee(
	acc *acc.Account, addrs []string, weights []uint64, fee uint64, conn *grpc.ClientConn,
) (tx *types.Transaction, splitAddr string, change *rpcpb.Utxo, err error) {
	// get utxos
	utxos, err := fetchUtxos(conn, acc.Addr(), fee, nil)
	if err != nil {
		return
	}
	return txlogic.NewSplitAddrTxWithUtxos(acc, addrs, weights, utxos, fee)
}

// MakeUnsignedTx make a tx without signature
func MakeUnsignedTx(
	wa service.WalletAgent, from string, to []string, amounts []uint64, fee uint64,
) (*types.Transaction, []*rpcpb.Utxo, error) {
	total := fee
	for _, a := range amounts {
		total += a
	}
	utxos, err := wa.Utxos(from, nil, total)
	if err != nil {
		return nil, nil, err
	}
	changeAmt, overflowed := calcChangeAmount(amounts, fee, utxos...)
	if overflowed {
		return nil, nil, txlogic.ErrInsufficientBalance
	}
	tx, err := txlogic.MakeUnsignedTx(from, to, amounts, changeAmt, utxos...)
	return tx, utxos, err
}

// MakeUnsignedSplitAddrTx news tx to make split addr without signature
// it returns a tx, split addr, a change
func MakeUnsignedSplitAddrTx(
	wa service.WalletAgent, from string, addrs []string, weights []uint64, fee uint64,
) (*types.Transaction, string, []*rpcpb.Utxo, error) {
	utxos, err := wa.Utxos(from, nil, fee)
	if err != nil {
		return nil, "", nil, err
	}
	changeAmt, overflowed := calcChangeAmount(nil, fee, utxos...)
	if overflowed {
		return nil, "", nil, txlogic.ErrInsufficientBalance
	}
	tx, splitAddr, err := txlogic.MakeUnsignedSplitAddrTx(from, addrs, weights,
		changeAmt, utxos...)
	return tx, splitAddr, utxos, err
}

// MakeUnsignedTokenIssueTx news tx to issue token without signature
func MakeUnsignedTokenIssueTx(
	wa service.WalletAgent, issuer, issuee string, tag *rpcpb.TokenTag, fee uint64,
) (*types.Transaction, uint32, []*rpcpb.Utxo, error) {
	utxos, err := wa.Utxos(issuer, nil, fee)
	if err != nil {
		return nil, 0, nil, err
	}
	changeAmt, overflowed := calcChangeAmount(nil, fee, utxos...)
	if overflowed {
		return nil, 0, nil, txlogic.ErrInsufficientBalance
	}
	tx, issueOutIndex, err := txlogic.MakeUnsignedTokenIssueTx(issuer, issuee, tag,
		changeAmt, utxos...)
	return tx, issueOutIndex, utxos, err
}

// MakeUnsignedTokenTransferTx news tx to transfer token without signature
func MakeUnsignedTokenTransferTx(
	wa service.WalletAgent, from string, to []string, amounts []uint64,
	tid *txlogic.TokenID, fee uint64,
) (*types.Transaction, []*rpcpb.Utxo, error) {
	utxos, err := wa.Utxos(from, nil, fee)
	if err != nil {
		return nil, nil, err
	}
	tAmt := uint64(0)
	for _, a := range amounts {
		tAmt += a
	}
	tokenUtxos, err := wa.Utxos(from, tid, tAmt)
	if err != nil {
		return nil, nil, err
	}
	changeAmt, overflowed := calcChangeAmount(nil, fee, utxos...)
	if overflowed {
		return nil, nil, txlogic.ErrInsufficientBalance
	}
	mixUtxos := append(utxos, tokenUtxos...)
	tx, _, err := txlogic.MakeUnsignedTokenTransferTx(from, to, amounts, tid,
		changeAmt, mixUtxos...)
	return tx, mixUtxos, err
}

func calcChangeAmount(amounts []uint64, fee uint64, utxos ...*rpcpb.Utxo) (uint64, bool) {
	total := fee
	for _, a := range amounts {
		total += a
	}
	uv := uint64(0)
	for _, u := range utxos {
		amount, tid, err := txlogic.ParseUtxoAmount(u)
		if err != nil || tid != nil {
			continue
		}
		uv += amount
	}
	changeAmt := uv - total
	return changeAmt, changeAmt > uv
}
