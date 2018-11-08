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

// GetFeePrice gets the recommended mining fee price according to recent packed transactions
func GetFeePrice(conn *grpc.ClientConn) (uint64, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

// CreateTransaction retrieves all the utxo of a public key, and use some of them to send transaction
func CreateTransaction(conn *grpc.ClientConn, fromAddress types.Address, targets map[types.Address]uint64, pubKeyBytes []byte, signer crypto.Signer) (*types.Transaction, error) {
	//var total, adjustedAmount uint64
	//for _, amount := range targets {
	//	total += amount
	//}
	//change := &corepb.TxOut{ScriptPubKey: getScriptAddress(fromAddress)}
	//gasPrice, err := GetFeePrice(conn)
	//if err != nil {
	//	return nil, err
	//}
	//var tx *corepb.Transaction
	//for {
	//	utxoResponse, err := FundTransaction(conn, fromAddress, total)
	//	if err != nil {
	//		return nil, err
	//	}
	//	tx, adjustedAmount = tryGenerateTx(utxoResponse.Utxos, targets, change, gasPrice)
	//	if tx != nil {
	//		break
	//	}
	//	if adjustedAmount > total {
	//		total = adjustedAmount
	//	} else {
	//		return nil, fmt.Errorf("unable to fund transaction with proper gas fee")
	//	}
	//}
	//if err := signTransaction(tx, fromAddress.Hash(), pubKeyBytes, signer); err != nil {
	//=======
	//func CreateTransaction(v *viper.Viper, fromAddress types.Address, targets map[types.Address]uint64,
	//	pubKeyBytes []byte, signer crypto.Signer) (*types.Transaction, error) {

	var total uint64
	for _, amount := range targets {
		total += amount
	}
	utxoResponse, err := FundTransaction(conn, fromAddress, total)
	if err != nil {
		return nil, err
	}

	tx, err := wrapTransaction(fromAddress, targets, pubKeyBytes, utxoResponse, false, false, nil, 0, nil, signer)
	if err != nil {
		//>>>>>>> develop
		return nil, err
	}

	txReq := &rpcpb.SendTransactionRequest{Tx: tx}

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
func GetRawTransaction(conn *grpc.ClientConn, hash []byte) (*types.Transaction, error) {
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
func ListUtxos(conn *grpc.ClientConn) (*rpcpb.ListUtxosResponse, error) {
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
func GetBalance(conn *grpc.ClientConn, addresses []string) (map[string]uint64, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := c.GetBalance(ctx, &rpcpb.GetBalanceRequest{Addrs: addresses})
	if err != nil {
		return map[string]uint64{}, err
	}
	return r.GetBalances(), err
}
