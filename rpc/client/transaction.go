// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"google.golang.org/genproto/googleapis/devtools/clouderrorreporting/v1beta1"
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/BOXFoundation/Quicksilver/rpc/pb"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

func CreateTransaction(v *viper.Viper, fromPubkey []byte, toPubKey []byte, amount int64) error {
	utxoResponse, err := FundTransaction(v, fromPubkey, amount)
	if err != nil {
		return err
	}
	txReq := &rpcpb.SendTransactionRequest{}
	txOut := []*rpcpb.TxOut{&rpcpb.TxOut{
		Value:        amount,
		ScriptPubKey: toPubKey,
	}}
	txMsg := &rpcpb.MsgTx{Vout: txOut}
	utxos := utxoResponse.GetUtxos()
	for _, utxo := range utxos {

	}
	var cfg = unmarshalConfig(v)
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port), grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Printf("Create transaction from: %v, to : %v", fromPubkey, toPubKey)

	r, err := c.FundTransaction(ctx, &rpcpb.FundTransactionRequest{
		PubKey: fromPubkey,
		Amount: amount,
	})
	if err != nil {
		return err
	}
	log.Printf("Result: %+v", r)
	return nil
}

func selectUtxo(resp *rpcpb.ListUtxosResponse, amount int64) ([]*rpcpb.UtxoWrap, error) {
	utxoList := resp.GetUtxos()
	sort.Slice(utxoList, func(i, j int) bool {
		return utxoList[i].Value < utxoList[j].Value
	})
	var current
	for i, utxo := range utxoList {
		current += utxo.GetValue()
		if current >= amount {
			return utxoList[:i+1], nil
		}
	}
	return nil, fmt.Errorf("Not enough balance")
}

func wrapTransaction(utxos []*rpcpb.UtxoWrap, amount int64) (*rpcpb.MsgTx, error) {
	msgTx := &rpcpb.MsgTx{}
	var current int64 = 0
	txIn := make([]*rpcpb.TxIn, len(utxos))
	for i, utxo := range utxos {

	}
}

func FundTransaction(v *viper.Viper, pubKey []byte, amount int64) (*rpcpb.ListUtxosResponse, error) {
	var cfg = unmarshalConfig(v)
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Printf("Fund transaction from: %v, amount : %v", pubKey, amount)

	r, err := c.FundTransaction(ctx, &rpcpb.FundTransactionRequest{
		PubKey: pubKey,
		Amount: amount,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("Result: %+v", r)
	return r, nil
}
