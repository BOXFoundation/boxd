// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"time"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
)

// GetGasPrice gets the recommended gas price according to recent packed transactions

func GetGasPrice(conn *grpc.ClientConn) (uint64, error) {
	c := rpcpb.NewTransactionCommandClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := c.GetGasPrice(ctx, &rpcpb.GetGasPriceRequest{})
	return r.BoxPerByte, err
}

// CreateTransaction retrieves all the utxo of a public key, and use some of them to send transaction
func CreateTransaction(conn *grpc.ClientConn, fromAddress types.Address, targets map[types.Address]uint64, pubKeyBytes []byte, signer crypto.Signer) (*types.Transaction, error) {
	var total, adjustedAmount uint64
	for _, amount := range targets {
		total += amount
	}
	change := &corepb.TxOut{ScriptPubKey: getScriptAddress(fromAddress)}
	gasPrice, err := GetGasPrice(conn)
	if err != nil {
		return nil, err
	}
	var tx *corepb.Transaction
	for {
		utxoResponse, err := FundTransaction(conn, fromAddress, total)
		if err != nil {
			return nil, err
		}
		tx, adjustedAmount = tryGenerateTx(utxoResponse.Utxos, targets, change, gasPrice)
		if tx != nil {
			break
		}
		if adjustedAmount > total {
			total = adjustedAmount
		} else {
			return nil, fmt.Errorf("unable to fund transaction with proper gas fee")
		}
	}
	if err := signTransaction(tx, fromAddress.ScriptAddress(), pubKeyBytes, signer); err != nil {
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

func tryGenerateTx(utxos []*rpcpb.Utxo, targets map[types.Address]uint64, change *corepb.TxOut, gasPricePerByte uint64) (*corepb.Transaction, uint64) {
	tx := &corepb.Transaction{}
	var inputAmount, outputAmount uint64
	txIn := make([]*corepb.TxIn, len(utxos))
	for i, utxo := range utxos {
		txIn[i] = &corepb.TxIn{
			PrevOutPoint: &corepb.OutPoint{
				Hash:  utxo.GetOutPoint().Hash,
				Index: utxo.GetOutPoint().GetIndex(),
			},
			ScriptSig: []byte{},
			Sequence:  uint32(i),
		}
		inputAmount += utxo.GetTxOut().GetValue()
	}
	tx.Vin = txIn
	vout := make([]*corepb.TxOut, 0)
	for addr, amount := range targets {
		toScript := getScriptAddress(addr)
		vout = append(vout, &corepb.TxOut{Value: amount, ScriptPubKey: toScript})
		outputAmount += amount
	}

	if inputAmount > outputAmount && change != nil {
		vout = append(vout, change)
	}
	tx.Vout = vout

	feeNeeded := calcFeeNeeded(tx, gasPricePerByte)
	if feeNeeded+outputAmount <= inputAmount {
		change.Value = inputAmount - outputAmount - feeNeeded
		fmt.Println("Fee used:", feeNeeded)
		return tx, 0
	}
	return nil, feeNeeded + outputAmount
}

func calcFeeNeeded(tx *corepb.Transaction, gasFeePerByte uint64) uint64 {
	var totalBytes int
	for _, vin := range tx.Vin {
		totalBytes += len(vin.ScriptSig)
	}
	for _, vout := range tx.Vout {
		totalBytes += len(vout.ScriptPubKey)
	}
	return uint64(totalBytes) * gasFeePerByte
}

func signTransaction(tx *corepb.Transaction, fromPubKeyHash, fromPubKeyBytes []byte, signer crypto.Signer) error {
	// Sign the tx inputs
	fromScript, err := getScriptAddressFromPubKeyHash(fromPubKeyHash)
	if err != nil {
		return err
	}
	prevScriptPubKey := script.NewScriptFromBytes(fromScript)

	typedTx := &types.Transaction{}
	if err := typedTx.FromProtoMessage(tx); err != nil {
		return err
	}
	for txInIdx, txIn := range typedTx.Vin {
		sigHash, err := script.CalcTxHashForSig(fromScript, typedTx, txInIdx)
		if err != nil {
			return err
		}
		sig, err := signer.Sign(sigHash)
		if err != nil {
			return err
		}
		scriptSig := script.SignatureScript(sig, fromPubKeyBytes)
		txIn.ScriptSig = *scriptSig
		tx.Vin[txInIdx].ScriptSig = *scriptSig

		// test to ensure
		if err = script.Validate(scriptSig, prevScriptPubKey, typedTx, txInIdx); err != nil {
			return err
		}
	}
	return nil
}

// FundTransaction gets the utxo of a public key
func FundTransaction(conn *grpc.ClientConn, addr types.Address, amount uint64) (*rpcpb.ListUtxosResponse, error) {
	p2pkScript, err := getScriptAddressFromPubKeyHash(addr.ScriptAddress())
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
