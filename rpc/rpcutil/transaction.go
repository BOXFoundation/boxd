// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpcutil

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	acc "github.com/BOXFoundation/boxd/wallet/account"
	"google.golang.org/grpc"
)

const (
	connTimeout = 30
	maxDecimal  = 8
)

// GetTokenBalance returns total amount of an address with specified token id
func GetTokenBalance(
	conn *grpc.ClientConn, addresses []string, tokenHash string, tokenIndex uint32,
) ([]uint64, error) {

	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	resp, err := c.GetTokenBalance(ctx, &rpcpb.GetTokenBalanceReq{
		Addrs: addresses, TokenHash: tokenHash, TokenIndex: tokenIndex})
	if err != nil {
		return nil, err
	} else if resp.Code != 0 {
		return nil, errors.New(resp.Message)
	}
	return resp.GetBalances(), nil
}

func newFetchUtxosReq(addr string, amount uint64) *rpcpb.FetchUtxosReq {
	return &rpcpb.FetchUtxosReq{Addr: addr, Amount: amount}
}

// FetchUtxos fetch utxos from chain
func FetchUtxos(
	conn *grpc.ClientConn, addr string, amount uint64, tHashStr string, tIdx uint32,
) ([]*rpcpb.Utxo, error) {

	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	req := newFetchUtxosReq(addr, amount)
	if tHashStr != "" {
		hash := new(crypto.HashType)
		if err := hash.SetString(tHashStr); err != nil {
			return nil, err
		}
		req.TokenHash = tHashStr
		req.TokenIndex = tIdx
	}
	resp, err := c.FetchUtxos(ctx, req)
	if err != nil {
		return nil, err
	} else if resp.Code != 0 {
		return nil, errors.New(resp.Message)
	}
	return resp.GetUtxos(), nil
}

// NewIssueTokenTx new a issue token transaction
func NewIssueTokenTx(
	acc *acc.Account, to *types.AddressHash, tag *rpcpb.TokenTag, conn *grpc.ClientConn,
) (*types.Transaction, *types.TokenID, *rpcpb.Utxo, error) {
	// fetch utxos for fee
	// multiply 2 to ensure utxos is enough to pay for tx fee
	utxos, err := fetchUtxos(conn, acc.Addr(), 2*core.TransferFee, "", 0)
	if err != nil {
		return nil, nil, nil, err
	}
	return txlogic.NewTokenIssueTxWithUtxos(acc, to, tag, utxos...)
}

// NewContractDeployTx new a deploy contract transaction
func NewContractDeployTx(
	acc *acc.Account, amount, gasLimit, nonce uint64, code []byte, conn *grpc.ClientConn,
) (*types.Transaction, string, error) {
	// fetch utxos for gas
	// add TransferFee to ensure utxos is enough to pay for tx fee
	utxos, err := fetchUtxos(conn, acc.Addr(), core.FixedGasPrice*gasLimit+core.TransferFee, "", 0)
	if err != nil {
		return nil, "", err
	}
	//
	tx, err := txlogic.MakeUnsignedContractDeployTx(acc.AddressHash(), amount,
		gasLimit, nonce, code, utxos...)
	if err != nil {
		return nil, "", err
	}
	// sign vin
	if err = txlogic.SignTxWithUtxos(tx, utxos, acc); err != nil {
		return nil, "", err
	}
	senderAddr, err := types.NewAddress(acc.Addr())
	if err != nil {
		return nil, "", err
	}
	contractAddr, _ := types.MakeContractAddress(senderAddr, nonce)
	return tx, contractAddr.String(), nil
}

// NewContractCallTx new a call contract transaction
func NewContractCallTx(
	acc *acc.Account, to *types.AddressHash, amount, gasLimit, nonce uint64,
	code []byte, conn *grpc.ClientConn,
) (*types.Transaction, error) {
	// fetch utxos for gas
	// add TransferFee to ensure utxos is enough to pay for tx fee
	utxos, err := fetchUtxos(conn, acc.Addr(), core.FixedGasPrice*gasLimit+core.TransferFee, "", 0)
	if err != nil {
		return nil, err
	}
	//
	tx, err := txlogic.MakeUnsignedContractCallTx(acc.AddressHash(), to, amount,
		gasLimit, nonce, code, utxos...)
	if err != nil {
		return nil, err
	}
	// sign vin
	if err = txlogic.SignTxWithUtxos(tx, utxos, acc); err != nil {
		return nil, err
	}
	return tx, nil
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
	} else if resp.GetCode() != 0 {
		return "", errors.New(resp.GetMessage())
	}
	return resp.Hash, nil
}

// NewTx new a tx and return change utxo
func NewTx(
	fromAcc *acc.Account, toAddrs []*types.AddressHash, amounts []uint64,
	conn *grpc.ClientConn,
) (*types.Transaction, *rpcpb.Utxo, error) {
	// calc amount
	amount := uint64(0)
	for _, a := range amounts {
		amount += a
	}
	// get utxos
	initialGasUsed := uint64((len(toAddrs)+1)/core.InOutNumPerExtraFee+1) * core.TransferFee
	utxos, err := fetchUtxos(conn, fromAcc.Addr(), amount+initialGasUsed, "", 0)
	if err != nil {
		return nil, nil, fmt.Errorf("fetchUtxos error for %s amount %d: %s",
			fromAcc.Addr(), amount+initialGasUsed, err)
	}
	return txlogic.NewTxWithUtxos(fromAcc, toAddrs, utxos, amounts)
}

// NewTxs construct some transactions
func NewTxs(
	fromAcc *acc.Account, toAddr *types.AddressHash, count int, conn *grpc.ClientConn,
) (txss [][]*types.Transaction, transfer, totalFee uint64, num int, err error) {
	// get utxoes
	utxos, err := fetchUtxos(conn, fromAcc.Addr(), 0, "", 0)
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
	//
	for _, u := range utxos {
		change := u
		value := change.GetTxOut().GetValue()
		aveAmt := value / uint64(n)
		if aveAmt <= core.TransferFee {
			err = fmt.Errorf("newtxs average amount is %d, continue with next utxo", aveAmt)
			continue
		}
		txs := make([]*types.Transaction, 0)
		for j := n; num < count && j > 0; j-- {
			amount := aveAmt - core.TransferFee
			tx := new(types.Transaction)
			tx, change, err = txlogic.NewTxWithUtxos(fromAcc, []*types.AddressHash{toAddr},
				[]*rpcpb.Utxo{change}, []uint64{amount})
			if err != nil {
				logger.Warn(err)
				continue
			}
			txs = append(txs, tx)
			transfer += amount
			totalFee += core.TransferFee
			num++
			if change == nil {
				break
			}
		}
		txss = append(txss, txs)
	}
	return txss, transfer, totalFee, num, nil
}

func fetchUtxos(
	conn *grpc.ClientConn, addr string, amount uint64, tHashStr string, tIdx uint32,
) (utxos []*rpcpb.Utxo, err error) {
	for t := 0; t < 30; t++ {
		utxos, err = FetchUtxos(conn, addr, amount, tHashStr, tIdx)
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
	acc *acc.Account, toAddrs []*types.AddressHash, amounts []uint64, tHashStr string,
	tIdx uint32, conn *grpc.ClientConn,
) (*types.Transaction, *rpcpb.Utxo, *rpcpb.Utxo, error) {
	amount := uint64((len(toAddrs)+1)/core.InOutNumPerExtraFee+1) * core.TransferFee
	amountT := uint64(0)
	for _, a := range amounts {
		amountT += a
	}
	boxUtxos, err := fetchUtxos(conn, acc.Addr(), amount, "", 0)
	if err != nil {
		return nil, nil, nil, err
	}
	tokenUtxos, err := fetchUtxos(conn, acc.Addr(), amountT, tHashStr, tIdx)
	if err != nil {
		return nil, nil, nil, err
	}
	utxos := append(boxUtxos, tokenUtxos...)
	tHash := new(crypto.HashType)
	if err := tHash.SetString(tHashStr); err != nil {
		return nil, nil, nil, err
	}
	tid := (*types.TokenID)(types.NewOutPoint(tHash, tIdx))
	return txlogic.NewTokenTransferTxWithUtxos(acc, toAddrs, amounts, tid, utxos...)
}

// NewTokenTxs new a token tx
func NewTokenTxs(
	acc *acc.Account, toAddr *types.AddressHash, amountT uint64, count int,
	tHashStr string, tIdx uint32, conn *grpc.ClientConn,
) ([]*types.Transaction, error) {
	// get utxos
	amount := core.TransferFee * uint64(count)
	boxUtxos, err := fetchUtxos(conn, acc.Addr(), amount, "", 0)
	if err != nil {
		return nil, err
	}
	tokenUtxos, err := fetchUtxos(conn, acc.Addr(), amountT, tHashStr, tIdx)
	if err != nil {
		return nil, err
	}
	utxos := append(boxUtxos, tokenUtxos...)
	// tid
	tHash := new(crypto.HashType)
	if err := tHash.SetString(tHashStr); err != nil {
		return nil, err
	}
	tid := (*types.TokenID)(types.NewOutPoint(tHash, tIdx))
	//
	var txs []*types.Transaction
	unitT := amountT / uint64(count)
	for i := 0; i < count; i++ {
		if i == count-1 {
			unitT = amountT - unitT*uint64(i)
		}
		tx, change, changeT, err := txlogic.NewTokenTransferTxWithUtxos(acc,
			[]*types.AddressHash{toAddr}, []uint64{unitT}, tid, utxos...)
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

// NewERC20TransferFromContractTxs new a contract transferFrom tx
func NewERC20TransferFromContractTxs(
	acc *acc.Account, contractAddr *types.AddressHash, count int, gasLimit, startNonce uint64,
	code []byte, conn *grpc.ClientConn,
) ([]*types.Transaction, error) {
	// get utxos
	amount := core.FixedGasPrice * gasLimit * uint64(count)
	utxos, err := fetchUtxos(conn, acc.Addr(), amount, "", 0)
	if err != nil {
		return nil, err
	}
	var boxAmt uint64
	for _, u := range utxos {
		boxAmt += u.TxOut.Value
	}
	//
	var txs []*types.Transaction
	nonce := startNonce
	for i := 0; i < count; i++ {
		tx, change, err := txlogic.NewContractTxWithUtxos(acc, contractAddr, 0,
			gasLimit, nonce, code, utxos...)
		if err != nil {
			return nil, err
		}
		nonce++
		txs = append(txs, tx)
		utxos = []*rpcpb.Utxo{change}
		if change == nil {
			break
		}
	}
	return txs, nil
}

// NewSplitAddrTx new split address tx
func NewSplitAddrTx(
	acc *acc.Account, addrs []*types.AddressHash, weights []uint32, conn *grpc.ClientConn,
) (tx *types.Transaction, change *rpcpb.Utxo, err error) {
	// get utxos
	// add TransferFee to ensure utxos is enough to pay for tx fee
	utxos, err := fetchUtxos(conn, acc.Addr(), 2*core.TransferFee, "", 0)
	if err != nil {
		return
	}
	return txlogic.NewSplitAddrTxWithUtxos(acc, addrs, weights, utxos)
}

// MakeUnsignedTx make a tx without signature
func MakeUnsignedTx(
	wa service.WalletAgent, from *types.AddressHash, to []*types.AddressHash,
	amounts []uint64,
) (*types.Transaction, []*rpcpb.Utxo, error) {
	// add an extra TransferFee to avoid change amount is zero
	total := uint64((len(to)+15)/core.InOutNumPerExtraFee+1) * core.TransferFee
	for _, a := range amounts {
		total += a
	}
	utxos, err := wa.Utxos(from, nil, total)
	if err != nil {
		return nil, nil, err
	}
	tx, err := txlogic.MakeUnsignedTx(from, to, amounts, utxos...)
	return tx, utxos, err
}

// MakeUnsignedCombineTx make a combine tx without signature
func MakeUnsignedCombineTx(
	wa service.WalletAgent, from *types.AddressHash,
) (*types.Transaction, []*rpcpb.Utxo, error) {
	// add TransferFee to ensure utxos is enough to pay for tx fee
	utxos, err := wa.Utxos(from, nil, 2*core.TransferFee)
	if err != nil {
		return nil, nil, err
	}
	utxosAmt := uint64(0)
	for _, u := range utxos {
		utxosAmt += u.GetTxOut().GetValue()
	}
	gasUsed := uint64((1+len(utxos))/core.InOutNumPerExtraFee+1) * core.TransferFee
	if utxosAmt <= gasUsed {
		return nil, nil, errors.New("insufficient input amount")
	}
	tx, err := txlogic.MakeUnsignedTx(from, []*types.AddressHash{from},
		[]uint64{utxosAmt - gasUsed}, utxos...)
	return tx, utxos, err
}

// MakeUnsignedCombineTokenTx make a combine tx without signature
func MakeUnsignedCombineTokenTx(
	wa service.WalletAgent, from *types.AddressHash, tid *types.TokenID,
) (*types.Transaction, []*rpcpb.Utxo, error) {
	// fetch box utxos
	// add an extra TransferFee to avoid change amount is zero
	utxos, err := wa.Utxos(from, nil, 2*core.TransferFee)
	if err != nil {
		return nil, nil, err
	}
	utxosAmt := uint64(0)
	for _, u := range utxos {
		utxosAmt += u.GetTxOut().GetValue()
	}
	totalUtxos := utxos
	// fetch token utxos
	utxos, err = wa.Utxos(from, tid, 0)
	if err != nil {
		return nil, nil, err
	}
	utxosAmtT := uint64(0)
	for _, u := range utxos {
		amount, id, _ := txlogic.ParseUtxoAmount(u)
		if id == nil || *id != *tid {
			logger.Warnf("get other token utxos %+v, want %+v", id, tid)
			continue
		}
		utxosAmtT += amount
	}
	totalUtxos = append(totalUtxos, utxos...)
	// make tx
	tx, err := txlogic.MakeUnsignedTokenTransferTx(from, []*types.AddressHash{from},
		[]uint64{utxosAmtT}, tid, totalUtxos...)
	return tx, utxos, err
}

//MakeUnsignedContractDeployTx make a tx without a signature
func MakeUnsignedContractDeployTx(
	wa service.WalletAgent, from *types.AddressHash, amount, gasLimit, nonce uint64,
	byteCode []byte,
) (*types.Transaction, []*rpcpb.Utxo, error) {
	// add TransferFee to ensure utxos is enough to pay for tx fee
	total := amount + gasLimit*core.FixedGasPrice + core.TransferFee
	utxos, err := wa.Utxos(from, nil, total)
	if err != nil {
		return nil, nil, err
	}
	tx, err := txlogic.MakeUnsignedContractDeployTx(from, amount, gasLimit, nonce,
		byteCode, utxos...)
	return tx, utxos, err
}

//MakeUnsignedContractCallTx call a contract tx without a signature
func MakeUnsignedContractCallTx(
	wa service.WalletAgent, from *types.AddressHash, amount, gasLimit, nonce uint64,
	contractAddr *types.AddressHash, byteCode []byte,
) (*types.Transaction, []*rpcpb.Utxo, error) {
	// add TransferFee to ensure utxos is enough to pay for tx fee
	total := amount + gasLimit*core.FixedGasPrice + core.TransferFee
	utxos, err := wa.Utxos(from, nil, total)
	if err != nil {
		return nil, nil, err
	}
	tx, err := txlogic.MakeUnsignedContractCallTx(from, contractAddr, amount,
		gasLimit, nonce, byteCode, utxos...)
	return tx, utxos, err
}

// MakeUnsignedSplitAddrTx news tx to make split addr without signature
// it returns a tx, split addr, a change
func MakeUnsignedSplitAddrTx(
	wa service.WalletAgent, from *types.AddressHash, addrs []*types.AddressHash,
	weights []uint32,
) (*types.Transaction, []*rpcpb.Utxo, error) {
	// add TransferFee to ensure utxos is enough to pay for tx fee
	utxos, err := wa.Utxos(from, nil, 2*core.TransferFee)
	if err != nil {
		return nil, nil, err
	}
	tx, err := txlogic.MakeUnsignedSplitAddrTx(from, addrs, weights, utxos...)
	return tx, utxos, err
}

// MakeUnsignedTokenIssueTx news tx to issue token without signature
func MakeUnsignedTokenIssueTx(
	wa service.WalletAgent, issuer, owner *types.AddressHash, tag *rpcpb.TokenTag,
) (*types.Transaction, uint32, []*rpcpb.Utxo, error) {
	if tag.GetDecimal() > maxDecimal {
		return nil, 0, nil, fmt.Errorf("Decimal[%d] is bigger than max Decimal[%d]", tag.GetDecimal(), maxDecimal)
	}
	if tag.GetSupply() > math.MaxUint64/uint64(math.Pow10(int(tag.GetDecimal()))) {
		return nil, 0, nil, fmt.Errorf("supply is too bigger")
	}
	// add TransferFee to ensure utxos is enough to pay for tx fee
	utxos, err := wa.Utxos(issuer, nil, 2*core.TransferFee)
	if err != nil {
		return nil, 0, nil, err
	}
	tx, issueOutIndex, err := txlogic.MakeUnsignedTokenIssueTx(issuer, owner, tag,
		utxos...)
	return tx, issueOutIndex, utxos, err
}

// MakeUnsignedTokenTransferTx news tx to transfer token without signature
func MakeUnsignedTokenTransferTx(
	wa service.WalletAgent, from *types.AddressHash, to []*types.AddressHash,
	amounts []uint64, tid *types.TokenID,
) (*types.Transaction, []*rpcpb.Utxo, error) {
	// add an extra TransferFee to avoid change amount is zero
	utxos, err := wa.Utxos(from, nil, 2*core.TransferFee)
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
	mixUtxos := append(utxos, tokenUtxos...)
	tx, err := txlogic.MakeUnsignedTokenTransferTx(from, to, amounts, tid, mixUtxos...)
	return tx, mixUtxos, err
}
