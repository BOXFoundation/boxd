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
	utxos, err := selectUtxo(utxoResponse, total)
	if err != nil {
		return nil, err
	}
	tx, err := wrapTransaction(fromAddress.ScriptAddress(), targets, pubKeyBytes, utxos, signer)
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

func selectUtxo(resp *rpcpb.ListUtxosResponse, amount uint64) ([]*rpcpb.Utxo, error) {
	utxoList := resp.GetUtxos()
	sort.Slice(utxoList, func(i, j int) bool {
		return utxoList[i].GetTxOut().GetValue() < utxoList[j].GetTxOut().GetValue()
	})
	var current uint64
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

func wrapTransaction(fromPubKeyHash []byte, targets map[types.Address]uint64, fromPubKeyBytes []byte, utxos []*rpcpb.Utxo, signer crypto.Signer) (*corepb.Transaction, error) {
	tx := &corepb.Transaction{}
	var current, total uint64
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
	vout := make([]*corepb.TxOut, 0)
	fromScript, err := getScriptAddress(fromPubKeyHash)
	prevScriptPubKey := script.NewScriptFromBytes(fromScript)
	if err != nil {
		return nil, err
	}
	for addr, amount := range targets {
		toScript, err := getScriptAddress(addr.ScriptAddress())
		if err != nil {
			return nil, err
		}
		vout = append(vout, &corepb.TxOut{Value: amount, ScriptPubKey: toScript})
		total += amount
	}
	//toScript, err := getScriptAddress(toPubKeyHash)
	//if err != nil {
	//	return nil, err
	//}

	//fromScript, err := getScriptAddress(fromPubKeyHash)
	//prevScriptPubKey := script.NewScriptFromBytes(fromScript)
	//if err != nil {
	//	return nil, err
	//}
	//tx.Vout = []*corepb.TxOut{{
	//	Value:        amount,
	//	ScriptPubKey: toScript,
	//}}
	if current > total {
		vout = append(vout, &corepb.TxOut{
			Value:        current - total,
			ScriptPubKey: fromScript,
		})
	}

	tx.Vout = vout

	// Sign the tx inputs
	typedTx := &types.Transaction{}
	if err = typedTx.FromProtoMessage(tx); err != nil {
		return nil, err
	}
	for txInIdx, txIn := range typedTx.Vin {
		sigHash, err := script.CalcTxHashForSig(fromScript, typedTx, txInIdx)
		if err != nil {
			return nil, err
		}
		sig, err := signer.Sign(sigHash)
		if err != nil {
			return nil, err
		}
		scriptSig := script.SignatureScript(sig, fromPubKeyBytes)
		txIn.ScriptSig = *scriptSig
		tx.Vin[txInIdx].ScriptSig = *scriptSig

		// test to ensure
		if err = script.Validate(scriptSig, prevScriptPubKey, typedTx, txInIdx); err != nil {
			return nil, err
		}
	}

	return tx, nil
}

// FundTransaction gets the utxo of a public key
func FundTransaction(v *viper.Viper, addr types.Address, amount uint64) (*rpcpb.ListUtxosResponse, error) {
	conn := mustConnect(v)
	defer conn.Close()
	p2pkScript, err := getScriptAddress(addr.ScriptAddress())
	if err != nil {
		return nil, err
	}
	logger.Debugf("Script Value: %v", p2pkScript)
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := c.FundTransaction(ctx, &rpcpb.FundTransactionRequest{
		Addr:   addr.EncodeAddress(),
		Amount: amount,
	})
	if err != nil {
		return nil, err
	}
	logger.Debugf("Result: %+v", r)
	return r, nil
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
