// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"time"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"google.golang.org/grpc"
)

const (
	// min output value
	dustLimit = 1
)

// CreateTokenIssueTx retrieves all the utxo of a public key, and use some of them to fund token issurance tx
func CreateTokenIssueTx(conn *grpc.ClientConn, fromAddress, toAddress types.Address, pubKeyBytes []byte, tokenName, tokenSymbol string,
	tokenTotalSupply uint64, tokenDecimals uint8, signer crypto.Signer) (*types.Transaction, error) {

	txReq := &rpcpb.SendTransactionRequest{}
	issueScript, err := getIssueTokenScript(toAddress.Hash(), tokenName, tokenSymbol, tokenTotalSupply, tokenDecimals)
	if err != nil {
		return nil, err
	}

	price, err := GetFeePrice(conn)
	if err != nil {
		return nil, err
	}

	var tx *corepb.Transaction
	amount := uint64(dustLimit)
	change := &corepb.TxOut{
		Value:        0,
		ScriptPubKey: getScriptAddress(fromAddress),
	}
	for {
		utxoResponse, err := FundTransaction(conn, fromAddress, amount)
		if err != nil {
			return nil, err
		}
		tx = generateTokenIssueTransaction(issueScript, utxoResponse.GetUtxos(), change)
		if err = signTransaction(tx, utxoResponse.GetUtxos(), pubKeyBytes, signer); err != nil {
			return nil, err
		}
		ok, adjustedAmount := tryBalance(tx, change, utxoResponse.Utxos, price)
		if ok {
			signTransaction(tx, utxoResponse.GetUtxos(), pubKeyBytes, signer)
			break
		}
		amount = adjustedAmount
	}
	txReq.Tx = tx

	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := c.SendTransaction(ctx, txReq)
	if err != nil {
		return nil, err
	}
	logger.Debugf("Result: %+v", r)
	transaction := &types.Transaction{}
	transaction.FromProtoMessage(tx)
	return transaction, nil
}

// CreateTokenTransferTx retrieves all the token utxo of a public key, and use some of them to fund token transfer tx
func CreateTokenTransferTx(conn *grpc.ClientConn, fromAddress types.Address, targets map[types.Address]uint64, pubKeyBytes []byte,
	tokenTxHash crypto.HashType, tokenTxOutIdx uint32, signer crypto.Signer) (*types.Transaction, error) {

	var totalToken uint64
	transferTargets := make([]*TransferParam, 0)
	token := &types.OutPoint{
		Hash:  tokenTxHash,
		Index: tokenTxOutIdx,
	}
	for addr, amount := range targets {
		transferTargets = append(transferTargets, &TransferParam{
			addr:    addr,
			isToken: true,
			amount:  amount,
			token:   token,
		})
		totalToken += amount
	}

	change := &corepb.TxOut{
		Value:        0,
		ScriptPubKey: getScriptAddress(fromAddress),
	}

	price, err := GetFeePrice(conn)
	if err != nil {
		return nil, err
	}

	var tx *corepb.Transaction
	boxAmount := uint64(dustLimit * (len(targets) + 1))
	for {
		utxoResponse, err := FundTokenTransaction(conn, fromAddress, token, boxAmount, totalToken)
		if err != nil {
			return nil, err
		}
		if tx, err = generateTx(fromAddress, utxoResponse.GetUtxos(), transferTargets, change); err != nil {
			return nil, err
		}
		if err = signTransaction(tx, utxoResponse.GetUtxos(), pubKeyBytes, signer); err != nil {
			return nil, err
		}
		ok, adjustedAmount := tryBalance(tx, change, utxoResponse.Utxos, price)
		if ok {
			signTransaction(tx, utxoResponse.GetUtxos(), pubKeyBytes, signer)
			break
		}
		boxAmount = adjustedAmount
	}

	//fmt.Print(util.PrettyPrint(tx))
	txReq := &rpcpb.SendTransactionRequest{Tx: tx}

	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := c.SendTransaction(ctx, txReq)
	if err != nil {
		return nil, err
	}
	logger.Debugf("Result: %+v", r)
	transaction := &types.Transaction{}
	transaction.FromProtoMessage(tx)
	return transaction, nil
}

// GetTokenBalance returns the token balance of a public key
func GetTokenBalance(conn *grpc.ClientConn, addr types.Address,
	tokenTxHash crypto.HashType, tokenTxOutIdx uint32) (uint64, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := c.GetTokenBalance(ctx, &rpcpb.GetTokenBalanceRequest{
		Addrs: []string{addr.String()},
		Token: &corepb.OutPoint{
			Hash:  tokenTxHash.GetBytes(),
			Index: tokenTxOutIdx,
		},
	})
	if err != nil {
		return 0, err
	}
	return r.GetBalances()[addr.String()], nil
}
