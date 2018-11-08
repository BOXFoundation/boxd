// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"bytes"
	"fmt"
	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"reflect"
	"sort"
)

var logger = log.NewLogger("rpcclient") // logger for client package

func unmarshalConfig(v *viper.Viper) *config.Config {
	var cfg config.Config
	viper.Unmarshal(&cfg)
	return &cfg
}

func mustConnect(v *viper.Viper) *grpc.ClientConn {
	var cfg = unmarshalConfig(v)
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", cfg.RPC.Address, cfg.RPC.Port), grpc.WithInsecure())
	if err != nil {
		panic("Fail to establish grpc connection")
	}
	return conn
}

func getScriptAddressFromPubKeyHash(pubKeyHash []byte) ([]byte, error) {
	addr, err := types.NewAddressPubKeyHash(pubKeyHash)
	if err != nil {
		return nil, err
	}
	return *script.PayToPubKeyHashScript(addr.Hash()), nil
}

// returns token issurance scriptPubKey
func getIssueTokenScript(pubKeyHash []byte, tokenName string, tokenTotalSupply uint64) ([]byte, error) {
	addr, err := types.NewAddressPubKeyHash(pubKeyHash)
	if err != nil {
		return nil, err
	}
	issueParams := &script.IssueParams{Name: tokenName, TotalSupply: tokenTotalSupply}
	return *script.IssueTokenScript(addr.Hash(), issueParams), nil
}

// returns token transfer scriptPubKey
func getTransferTokenScript(pubKeyHash []byte, tokenTxHash *crypto.HashType, tokenTxOutIdx uint32, amount uint64) ([]byte, error) {
	addr, err := types.NewAddressPubKeyHash(pubKeyHash)
	if err != nil {
		return nil, err
	}

	transferParams := &script.TransferParams{}
	transferParams.Hash = *tokenTxHash
	transferParams.Index = tokenTxOutIdx
	transferParams.Amount = amount

	script := script.TransferTokenScript(addr.Hash(), transferParams)
	return *script, nil
}

// get token amount in the passed utxo
func getUtxoTokenAmount(utxo *rpcpb.Utxo, tokenTxHash *crypto.HashType, tokenTxOutIdx uint32) uint64 {
	scriptPubKey := script.NewScriptFromBytes(utxo.GetTxOut().GetScriptPubKey())

	if scriptPubKey.IsTokenIssue() {
		// token issurance utxo
		// no need to check error since it will not err
		if bytes.Equal(utxo.OutPoint.Hash, tokenTxHash.GetBytes()) && utxo.OutPoint.Index == tokenTxOutIdx {
			params, _ := scriptPubKey.GetIssueParams()
			return params.TotalSupply
		}
	}
	if scriptPubKey.IsTokenTransfer() {
		// token transfer utxo
		// no need to check error since it will not err
		params, _ := scriptPubKey.GetTransferParams()
		if bytes.Equal(params.Hash.GetBytes(), tokenTxHash.GetBytes()) && params.Index == tokenTxOutIdx {
			return params.Amount
		}
	}
	return 0
}

// colored: use colored or uncolored utxos/coins
// tokenTxHash & tokenTxOutIdx only valid for colored utxos
func selectUtxo(resp *rpcpb.ListUtxosResponse, totalAmount uint64, colored bool,
	tokenTxHash *crypto.HashType, tokenTxOutIdx uint32) ([]*rpcpb.Utxo, error) {

	utxoList := resp.GetUtxos()
	sort.Slice(utxoList, func(i, j int) bool {
		// TODO: sort by token amount for token utxos
		return utxoList[i].GetTxOut().GetValue() < utxoList[j].GetTxOut().GetValue()
	})
	var currentAmount uint64
	resultList := []*rpcpb.Utxo{}
	for _, utxo := range utxoList {
		if utxo.IsSpent {
			continue
		}

		var amount uint64
		if !colored {
			scriptPubKey := script.NewScriptFromBytes(utxo.GetTxOut().GetScriptPubKey())
			if !scriptPubKey.IsPayToPubKeyHash() {
				continue
			}
			// p2pkh tx
			amount = utxo.GetTxOut().GetValue()
		} else {
			// token tx
			amount = getUtxoTokenAmount(utxo, tokenTxHash, tokenTxOutIdx)
			if amount == 0 {
				// non-token or different token
				continue
			}
		}
		currentAmount += amount
		resultList = append(resultList, utxo)
		if currentAmount >= totalAmount {
			return resultList, nil
		}
	}
	return nil, fmt.Errorf("Not enough balance")
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

func signTransaction(tx *corepb.Transaction, utxos []*rpcpb.Utxo, fromPubKeyBytes []byte, signer crypto.Signer) error {
	// Sign the tx inputs
	typedTx := &types.Transaction{}
	if err := typedTx.FromProtoMessage(tx); err != nil {
		return err
	}
	for txInIdx, txIn := range tx.Vin {
		prevScriptPubKeyBytes, err := findUtxoScriptPubKey(utxos, txIn.PrevOutPoint)
		if err != nil {
			return err
		}
		prevScriptPubKey := script.NewScriptFromBytes(prevScriptPubKeyBytes)
		sigHash, err := script.CalcTxHashForSig(prevScriptPubKeyBytes, typedTx, txInIdx)
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

func wrapTransaction(addr types.Address, targets map[types.Address]uint64, fromPubKeyBytes []byte, utxoResp *rpcpb.ListUtxosResponse,
	coloredInput, coloredOutput bool, tokenTxHash *crypto.HashType, tokenTxOutIdx uint32, tokenScriptPubKey []byte, signer crypto.Signer) (*corepb.Transaction, error) {

	var total uint64
	for _, amount := range targets {
		total += amount
	}

	utxos, err := selectUtxo(utxoResp, total, coloredInput, tokenTxHash, tokenTxOutIdx)
	if err != nil {
		return nil, err
	}

	tx := &corepb.Transaction{}
	var currentAmount uint64
	txIn := make([]*corepb.TxIn, len(utxos))
	logger.Debugf("wrap transaction, utxos:%+v\n", utxos)
	for i, utxo := range utxos {
		txIn[i] = &corepb.TxIn{
			PrevOutPoint: &corepb.OutPoint{
				Hash:  utxo.GetOutPoint().Hash,
				Index: utxo.GetOutPoint().GetIndex(),
			},
			ScriptSig: []byte{},
			Sequence:  uint32(0),
		}
		// no need to check utxo as selectUtxo() already filters
		if !coloredInput {
			currentAmount += utxo.GetTxOut().GetValue()
		} else {
			currentAmount += getUtxoTokenAmount(utxo, tokenTxHash, tokenTxOutIdx)
		}
	}
	tx.Vin = txIn

	for addr, amount := range targets {
		if !coloredOutput {
			// general tx
			scriptPubKey, err := getScriptAddressFromPubKeyHash(addr.Hash())
			if err != nil {
				return nil, err
			}
			tx.Vout = append(tx.Vout, &corepb.TxOut{Value: amount, ScriptPubKey: scriptPubKey})
			total += amount
		} else {
			// token tx
			tx.Vout = append(tx.Vout, &corepb.TxOut{
				Value:        dustLimit,
				ScriptPubKey: tokenScriptPubKey,
			})
		}
	}

	if !coloredInput {
		if currentAmount > total {
			change := currentAmount - total
			changeScript, err := getScriptAddressFromPubKeyHash(addr.Hash())
			if err != nil {
				return nil, err
			}
			tx.Vout = append(tx.Vout, &corepb.TxOut{
				Value:        change,
				ScriptPubKey: changeScript,
			})
		}
	} else {
		if currentAmount > total {
			change := currentAmount - total
			changeScript, err := getTransferTokenScript(addr.Hash(), tokenTxHash, tokenTxOutIdx, change)
			if err != nil {
				return nil, err
			}
			tx.Vout = append(tx.Vout, &corepb.TxOut{
				Value:        dustLimit,
				ScriptPubKey: changeScript,
			})
		}

		// number of colored output exceeds that of colored input
		if len(tx.Vin) < len(tx.Vout) {
			// only way to reach here is 1 colored input and 2 colored outputs
			if len(tx.Vin) != 1 || len(tx.Vout) != 2 {
				return nil, fmt.Errorf("vin size is not 1: %d or vout size is not 2: %d", len(tx.Vin), len(tx.Vout))
			}
			// need one uncolored utxo to cover associated colored output box deficit
			uncoloredUtxos, err := selectUtxo(utxoResp, dustLimit, false, nil, 0)
			if err != nil {
				return nil, err
			}
			if len(uncoloredUtxos) != 1 {
				return nil, fmt.Errorf("Any utxo should be larger than dust limit")
			}
			utxo := uncoloredUtxos[0]
			utxos = append(utxos, utxo)
			// the uncolored utxo
			tx.Vin = append(tx.Vin, &corepb.TxIn{
				PrevOutPoint: &corepb.OutPoint{
					Hash:  utxo.GetOutPoint().Hash,
					Index: utxo.GetOutPoint().GetIndex(),
				},
				ScriptSig: []byte{},
				Sequence:  uint32(0),
			})
			// uncolored change if any
			if utxo.GetTxOut().GetValue() > dustLimit {
				change := utxo.GetTxOut().GetValue() - dustLimit
				changeScript, err := getScriptAddressFromPubKeyHash(addr.Hash())
				if err != nil {
					return nil, err
				}
				tx.Vout = append(tx.Vout, &corepb.TxOut{
					Value:        change,
					ScriptPubKey: changeScript,
				})
			}
		}
	}
	if err := signTransaction(tx, utxos, fromPubKeyBytes, signer); err != nil {
		return nil, err
	}

	return tx, nil
}

// find an outpoint's referenced utxo's scriptPubKey
func findUtxoScriptPubKey(utxos []*rpcpb.Utxo, outPoint *corepb.OutPoint) ([]byte, error) {
	for _, utxo := range utxos {
		if reflect.DeepEqual(utxo.GetOutPoint(), outPoint) {
			return utxo.GetTxOut().GetScriptPubKey(), nil
		}
	}
	return nil, fmt.Errorf("outPoint's referenced utxo not found")
}

func getScriptAddress(address types.Address) []byte {
	return *script.PayToPubKeyHashScript(address.Hash())
}

// NewConnectionWithViper initializes a grpc connection using configs parsed by viper
func NewConnectionWithViper(v *viper.Viper) *grpc.ClientConn {
	return mustConnect(v)
}

// NewConnectionWithHostPort initializes a grpc connection using host and port params
func NewConnectionWithHostPort(host string, port int) *grpc.ClientConn {
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", host, port), grpc.WithInsecure())
	if err != nil {
		panic("Fail to establish grpc connection")
	}
	return conn
}
