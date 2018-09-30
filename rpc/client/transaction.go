// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/BOXFoundation/Quicksilver/rpc/pb"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

// CreateTransaction retrieves all the utxo of a public key, and use some of them to send transaction
func CreateTransaction(v *viper.Viper, fromPubkey []byte, toPubKey []byte, amount int64) error {
	utxoResponse, err := FundTransaction(v, fromPubkey, amount)
	if err != nil {
		return err
	}

	txReq := &rpcpb.SendTransactionRequest{}
	utxos, err := selectUtxo(utxoResponse, amount)
	if err != nil {
		return err
	}
	msgTx, err := wrapTransaction(fromPubkey, toPubKey, utxos, amount)
	if err != nil {
		return err
	}
	txReq.Tx = msgTx

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
	r, err := c.SendTransaction(ctx, txReq)
	if err != nil {
		return err
	}
	log.Printf("Result: %+v", r)
	return nil
}

func selectUtxo(resp *rpcpb.ListUtxosResponse, amount int64) ([]*rpcpb.Utxo, error) {
	utxoList := resp.GetUtxos()
	sort.Slice(utxoList, func(i, j int) bool {
		return utxoList[i].GetTxOut().GetValue() < utxoList[j].GetTxOut().GetValue()
	})
	var current int64
	resultList := []*rpcpb.Utxo{}
	for _, utxo := range utxoList {
		if utxo.IsSpent {
			continue
		}
		current += utxo.GetTxOut().GetValue()
		resultList = append(resultList, utxo)
		if current >= amount {
			return resultList, nil
		}
	}
	return nil, fmt.Errorf("Not enough balance")
}

func wrapTransaction(fromPubKey, toPubKey []byte, utxos []*rpcpb.Utxo, amount int64) (*rpcpb.MsgTx, error) {
	msgTx := &rpcpb.MsgTx{}
	var current int64
	txIn := make([]*rpcpb.TxIn, len(utxos))
	fmt.Printf("wrap transaction, utxos:%+v\n", utxos)
	for i, utxo := range utxos {
		txIn[i] = &rpcpb.TxIn{
			PrevOutPoint: &rpcpb.OutPoint{
				Hash:  utxo.GetOutPoint().Hash,
				Index: utxo.GetOutPoint().GetIndex(),
			},
			ScriptSig: []byte{},
			Sequence:  uint32(i),
		}
		current += utxo.GetTxOut().GetValue()
	}
	msgTx.Vin = txIn
	fmt.Println("wrap vout")
	toScript, err := getScriptAddress(toPubKey)
	if err != nil {
		return nil, err
	}
	fromScript, err := getScriptAddress(fromPubKey)
	if err != nil {
		return nil, err
	}
	msgTx.Vout = []*rpcpb.TxOut{{
		Value:        amount,
		ScriptPubKey: toScript,
	}}
	fmt.Println("wrap change")
	if current > amount {
		msgTx.Vout = append(msgTx.Vout, &rpcpb.TxOut{
			Value:        current - amount,
			ScriptPubKey: fromScript,
		})
	}
	return msgTx, nil
}

// FundTransaction gets the utxo of a public key
func FundTransaction(v *viper.Viper, pubKey []byte, amount int64) (*rpcpb.ListUtxosResponse, error) {
	var cfg = unmarshalConfig(v)
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	p2pkScript, err := getScriptAddress(pubKey)
	if err != nil {
		return nil, err
	}
	log.Printf("Script Value: %v", p2pkScript)
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Printf("Fund transaction from: %v, amount : %v", pubKey, amount)

	r, err := c.FundTransaction(ctx, &rpcpb.FundTransactionRequest{
		PubKey: p2pkScript,
		Amount: amount,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("Result: %+v", r)
	return r, nil
}

//ListUtxos list all utxos
func ListUtxos(v *viper.Viper) error {
	var cfg = unmarshalConfig(v)
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port), grpc.WithInsecure())
	if err != nil {
		return err
	}
	defer conn.Close()
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Printf("List Utxos")
	r, err := c.ListUtxos(ctx, &rpcpb.ListUtxosRequest{})
	if err != nil {
		return err
	}
	log.Printf("Result: %+v", r)
	return nil
}
