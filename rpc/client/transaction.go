// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
	"github.com/spf13/viper"
)

// CreateTransaction retrieves all the utxo of a public key, and use some of them to send transaction
func CreateTransaction(v *viper.Viper, fromPubkeyHash, toPubKeyHash, pubKeyBytes []byte, amount int64, signer crypto.Signer) (*types.Transaction, error) {
	utxoResponse, err := FundTransaction(v, fromPubkeyHash, amount)

	if err != nil {
		return nil, err
	}

	txReq := &rpcpb.SendTransactionRequest{}
	utxos, err := selectUtxo(utxoResponse, amount)
	if err != nil {
		return nil, err
	}
	tx, err := wrapTransaction(fromPubkeyHash, toPubKeyHash, pubKeyBytes, utxos, amount, signer)
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

func wrapTransaction(fromPubKeyHash, toPubKeyHash, fromPubKeyBytes []byte, utxos []*rpcpb.Utxo, amount int64, signer crypto.Signer) (*corepb.Transaction, error) {
	tx := &corepb.Transaction{}
	var current int64
	txIn := make([]*corepb.TxIn, len(utxos))
	logger.Debugf("wrap transaction, utxos:%+v\n", utxos)
	for i, utxo := range utxos {
		txIn[i] = &corepb.TxIn{
			PrevOutPoint: &corepb.OutPoint{
				Hash:  utxo.GetOutPoint().Hash,
				Index: utxo.GetOutPoint().GetIndex(),
			},
			ScriptSig: []byte{},
			Sequence:  uint32(i),
		}
		current += utxo.GetTxOut().GetValue()
	}
	tx.Vin = txIn
	toScript, err := getScriptAddress(toPubKeyHash)
	if err != nil {
		return nil, err
	}

	fromScript, err := getScriptAddress(fromPubKeyHash)
	if err != nil {
		return nil, err
	}
	tx.Vout = []*corepb.TxOut{{
		Value:        amount,
		ScriptPubKey: toScript,
	}}
	if current > amount {
		tx.Vout = append(tx.Vout, &corepb.TxOut{
			Value:        current - amount,
			ScriptPubKey: fromScript,
		})
	}

	// Sign the tx inputs
	typedTx := &types.Transaction{}
	if err = typedTx.FromProtoMessage(tx); err != nil {
		return nil, err
	}
	for i, txIn := range typedTx.Vin {
		sigHash, err := script.CalcTxHashForSig(fromScript, typedTx, i)
		if err != nil {
			return nil, err
		}
		sig, err := signer.Sign(sigHash)
		if err != nil {
			return nil, err
		}
		scriptSig := script.NewScript()
		scriptSig.AddOperand(sig.Serialize()).AddOperand(fromPubKeyBytes)
		txIn.ScriptSig = *scriptSig
		tx.Vin[i].ScriptSig = *scriptSig

		// test to ensure
		catScript := script.NewScript()
		// concatenate unlocking & locking scripts
		catScript.AddOperand(txIn.ScriptSig).AddOpCode(script.OPCODESEPARATOR).AddOperand(fromScript)
		if err = catScript.Evaluate(typedTx, i); err != nil {
			return nil, err
		}
	}

	return tx, nil
}

// FundTransaction gets the utxo of a public key
func FundTransaction(v *viper.Viper, pubKeyHash []byte, amount int64) (*rpcpb.ListUtxosResponse, error) {
	conn := mustConnect(v)
	defer conn.Close()
	p2pkScript, err := getScriptAddress(pubKeyHash)
	if err != nil {
		return nil, err
	}
	logger.Debugf("Script Value: %v", p2pkScript)
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Debugf("Fund transaction from: %v, amount : %v", pubKeyHash, amount)

	r, err := c.FundTransaction(ctx, &rpcpb.FundTransactionRequest{
		ScriptPubKey: p2pkScript,
		Amount:       amount,
	})
	if err != nil {
		return nil, err
	}
	logger.Debugf("Result: %+v", r)
	return r, nil
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
