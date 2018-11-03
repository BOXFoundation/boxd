// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
	"github.com/spf13/viper"
)

const (
	// min output value
	dustLimit = 1
)

// CreateTokenIssueTx retrieves all the utxo of a public key, and use some of them to fund token issurance tx
func CreateTokenIssueTx(v *viper.Viper, fromPubkeyHash, toPubKeyHash, pubKeyBytes []byte, tokenName string,
	tokenTotalSupply uint64, signer crypto.Signer) (*types.Transaction, error) {
	utxoResponse, err := FundTransaction(v, fromPubkeyHash, dustLimit)

	if err != nil {
		return nil, err
	}

	txReq := &rpcpb.SendTransactionRequest{}
	scriptPubKey, err := getIssueTokenScript(toPubKeyHash, tokenName, tokenTotalSupply)
	if err != nil {
		return nil, err
	}
	tx, err := wrapTransaction(fromPubkeyHash, toPubKeyHash, pubKeyBytes, scriptPubKey, utxoResponse, dustLimit, false, signer)
	if err != nil {
		return nil, err
	}
	txReq.Tx = tx

	conn := mustConnect(v)
	defer conn.Close()
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Debugf("Create transaction from: %v, to : %v", fromPubkeyHash, toPubKeyHash)
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
func CreateTokenTransferTx(v *viper.Viper, fromPubkeyHash, toPubKeyHash, pubKeyBytes []byte,
	amount uint64, signer crypto.Signer) (*types.Transaction, error) {
	utxoResponse, err := FundTransaction(v, fromPubkeyHash, amount)

	if err != nil {
		return nil, err
	}

	txReq := &rpcpb.SendTransactionRequest{}
	scriptPubKey, err := getTransferTokenScript(toPubKeyHash, amount)
	if err != nil {
		return nil, err
	}
	tx, err := wrapTransaction(fromPubkeyHash, toPubKeyHash, pubKeyBytes, scriptPubKey, utxoResponse, amount, true, signer)
	if err != nil {
		return nil, err
	}
	txReq.Tx = tx

	conn := mustConnect(v)
	defer conn.Close()
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Debugf("Create transaction from: %v, to : %v", fromPubkeyHash, toPubKeyHash)
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
func GetTokenBalance(v *viper.Viper, pubkeyHash []byte) uint64 {
	utxoResponse, err := FundTransaction(v, pubkeyHash, 0)
	if err != nil {
		return 0
	}
	utxos := utxoResponse.GetUtxos()
	var currentAmount uint64
	for _, utxo := range utxos {
		if utxo.IsSpent {
			continue
		}
		scriptPubKey := script.NewScriptFromBytes(utxo.GetTxOut().GetScriptPubKey())
		if scriptPubKey.IsPayToPubKeyHash() {
			// not token
			continue
		}
		currentAmount += scriptPubKey.GetTokenAmount()
	}
	return currentAmount
}
