// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"google.golang.org/grpc"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
)

const (
	// min output value
	dustLimit = 1
)

// CreateTokenIssueTx retrieves all the utxo of a public key, and use some of them to fund token issurance tx
func CreateTokenIssueTx(conn *grpc.ClientConn, fromAddress, toAddress types.Address, pubKeyBytes []byte, tokenName string,
	tokenTotalSupply uint64, signer crypto.Signer) (*types.Transaction, error) {

	utxoResponse, err := FundTransaction(conn, fromAddress, dustLimit)
	if err != nil {
		return nil, err
	}

	txReq := &rpcpb.SendTransactionRequest{}
	scriptPubKey, err := getIssueTokenScript(toAddress.Hash(), tokenName, tokenTotalSupply)
	if err != nil {
		return nil, err
	}
	targets := make(map[types.Address]uint64, 0)
	targets[toAddress] = dustLimit
	tx, err := wrapTransaction(fromAddress, targets, pubKeyBytes, utxoResponse, false, true, nil, 0, scriptPubKey, signer)
	if err != nil {
		return nil, err
	}
	txReq.Tx = tx

	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := c.SendTransaction(ctx, txReq)
	if err != nil {
		return nil, err
	}
	logger.Infof("Result: %+v", r)
	transaction := &types.Transaction{}
	transaction.FromProtoMessage(tx)
	return transaction, nil
}

// CreateTokenTransferTx retrieves all the token utxo of a public key, and use some of them to fund token transfer tx
func CreateTokenTransferTx(conn *grpc.ClientConn, fromAddress types.Address, targets map[types.Address]uint64, pubKeyBytes []byte,
	tokenTxHash *crypto.HashType, tokenTxOutIdx uint32, signer crypto.Signer) (*types.Transaction, error) {

	var total, anyToAmount uint64
	var anyToAddress types.Address
	for addr, amount := range targets {
		total += amount
		anyToAddress = addr
		anyToAmount = amount
	}
	utxoResponse, err := FundTransaction(conn, fromAddress, total)
	if err != nil {
		return nil, err
	}

	txReq := &rpcpb.SendTransactionRequest{}
	// TODO: can only handle transfer to 1 address
	scriptPubKey, err := getTransferTokenScript(anyToAddress.Hash(), tokenTxHash, tokenTxOutIdx, anyToAmount)
	if err != nil {
		return nil, err
	}
	tx, err := wrapTransaction(fromAddress, targets, pubKeyBytes, utxoResponse,
		true, true, tokenTxHash, tokenTxOutIdx, scriptPubKey, signer)
	if err != nil {
		return nil, err
	}
	txReq.Tx = tx

	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := c.SendTransaction(ctx, txReq)
	if err != nil {
		return nil, err
	}
	logger.Infof("Result: %+v", r)
	transaction := &types.Transaction{}
	transaction.FromProtoMessage(tx)
	return transaction, nil
}

// GetTokenBalance returns the token balance of a public key
func GetTokenBalance(conn *grpc.ClientConn, addr types.Address, tokenTxHash *crypto.HashType, tokenTxOutIdx uint32) uint64 {
	utxoResponse, err := FundTransaction(conn, addr, 0)
	if err != nil {
		return 0
	}
	utxos := utxoResponse.GetUtxos()
	var currentAmount uint64
	for _, utxo := range utxos {
		txHashBytes := utxo.GetOutPoint().GetHash()
		hash := &crypto.HashType{}
		hash.SetBytes(txHashBytes)
		if utxo.IsSpent {
			continue
		}
		currentAmount += getUtxoTokenAmount(utxo, tokenTxHash, tokenTxOutIdx)
	}
	return currentAmount
}
