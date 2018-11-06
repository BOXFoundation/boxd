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
	"github.com/spf13/viper"
)

// CreateTransaction retrieves all the utxo of a public key, and use some of them to send transaction
func CreateTransaction(v *viper.Viper, fromAddress types.Address, targets map[types.Address]uint64, pubKeyBytes []byte, signer crypto.Signer) (*types.Transaction, error) {
	var total uint64
	for _, amount := range targets {
		total += amount
	}
	utxoResponse, err := FundTransaction(v, fromAddress, total)

	if err != nil {
		return nil, err
	}

	txReq := &rpcpb.SendTransactionRequest{}
	scriptPubKey, err := getPayToPubKeyHashScript(toPubKeyHash)
	if err != nil {
		return nil, err
	}
	tx, err := wrapTransaction(fromPubkeyHash, toPubKeyHash, pubKeyBytes, scriptPubKey, utxoResponse, nil, 0, amount, false, signer)
	if err != nil {
		return nil, err
	}
	txReq.Tx = tx

	conn := mustConnect(v)
	defer conn.Close()
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

// GetRawTransaction get the transaction info of given hash
func GetRawTransaction(v *viper.Viper, hash []byte) (*types.Transaction, error) {
	conn := mustConnect(v)
	defer conn.Close()
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

//ListUtxos list all utxos
func ListUtxos(v *viper.Viper) (*rpcpb.ListUtxosResponse, error) {
	conn := mustConnect(v)
	defer conn.Close()
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := c.ListUtxos(ctx, &rpcpb.ListUtxosRequest{})
	if err != nil {
		return nil, err
	}
	return r, nil
}

// GetBalance returns total amount of an address
func GetBalance(v *viper.Viper, addresses []string) (map[string]uint64, error) {
	conn := mustConnect(v)
	defer conn.Close()
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := c.GetBalance(ctx, &rpcpb.GetBalanceRequest{Addrs: addresses})
	if err != nil {
		return map[string]uint64{}, err
	}
	return r.GetBalances(), err
}
