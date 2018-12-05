// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"time"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"google.golang.org/grpc"
)

const connTimeout = 30

// GetFeePrice gets the recommended mining fee price according to recent packed transactions
func GetFeePrice(conn *grpc.ClientConn) (uint64, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	r, err := c.GetFeePrice(ctx, &rpcpb.GetFeePriceRequest{})
	return r.BoxPerByte, err
}

// FundTransaction gets the utxo of a public key
func FundTransaction(conn *grpc.ClientConn, addr types.Address, amount uint64) (*rpcpb.ListUtxosResponse, error) {
	p2pkScript, err := getScriptAddressFromPubKeyHash(addr.Hash())
	if err != nil {
		return nil, err
	}
	logger.Debugf("Script Value: %v", p2pkScript)
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()

	r, err := c.FundTransaction(ctx, &rpcpb.FundTransactionRequest{
		Addr:   addr.String(),
		Amount: amount,
	})
	if err != nil {
		return nil, err
	}
	logger.Debugf("Result: %+v", r)
	return r, nil
}

// FundTokenTransaction gets the utxo of a public key containing a certain amount of box and token
func FundTokenTransaction(conn *grpc.ClientConn, addr types.Address, token *types.OutPoint, boxAmount, tokenAmount uint64) (*rpcpb.ListUtxosResponse, error) {
	p2pkScript, err := getScriptAddressFromPubKeyHash(addr.Hash())
	if err != nil {
		return nil, err
	}
	logger.Debugf("Script Value: %v", p2pkScript)
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()

	tokenBudges := make([]*rpcpb.TokenAmount, 0)
	if token != nil && tokenAmount > 0 {
		outPointMsg, err := token.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		outPointPb, ok := outPointMsg.(*corepb.OutPoint)
		if !ok {
			return nil, fmt.Errorf("Invalid token outpoint")
		}
		tokenBudges = append(tokenBudges, &rpcpb.TokenAmount{
			Token:  outPointPb,
			Amount: tokenAmount,
		})
	}
	r, err := c.FundTransaction(ctx, &rpcpb.FundTransactionRequest{
		Addr:         addr.String(),
		Amount:       boxAmount,
		TokenBudgets: tokenBudges,
	})
	if err != nil {
		return nil, err
	}
	logger.Debugf("Result: %+v", r)
	return r, nil
}

// CreateTransaction retrieves all the utxo of a public key, and use some of them to send transaction
func CreateTransaction(conn *grpc.ClientConn, fromAddress types.Address, targets map[types.Address]uint64, pubKeyBytes []byte,
	signer crypto.Signer, addrs []types.Address, weights []uint64) (*types.Transaction, error) {
	var totalAmount uint64
	transferTargets := make([]*TransferParam, 0)
	for addr, amount := range targets {
		totalAmount += amount
		transferTargets = append(transferTargets, &TransferParam{
			addr:    addr,
			isToken: false,
			amount:  amount,
			token:   nil,
			addrs:   addrs,
			weights: weights,
		})
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
	for {
		utxoResponse, err := FundTransaction(conn, fromAddress, totalAmount)
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
		totalAmount = adjustedAmount
	}

	txReq := &rpcpb.SendTransactionRequest{Tx: tx}

	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()

	_, err = c.SendTransaction(ctx, txReq)
	if err != nil {
		return nil, err
	}
	transaction := &types.Transaction{}
	transaction.FromProtoMessage(tx)
	return transaction, nil
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

//ListUtxos list all utxos
func ListUtxos(conn *grpc.ClientConn) (*rpcpb.ListUtxosResponse, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	r, err := c.ListUtxos(ctx, &rpcpb.ListUtxosRequest{})
	if err != nil {
		return nil, err
	}
	return r, nil
}

// GetBalance returns total amount of an address
func GetBalance(conn *grpc.ClientConn, addresses []string) (map[string]uint64, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), connTimeout*time.Second)
	defer cancel()
	r, err := c.GetBalance(ctx, &rpcpb.GetBalanceRequest{Addrs: addresses})
	if err != nil {
		return map[string]uint64{}, err
	}
	return r.GetBalances(), err
}
