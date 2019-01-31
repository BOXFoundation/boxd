// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"fmt"
	"math"
	"reflect"

	"github.com/BOXFoundation/boxd/config"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var logger = log.NewLogger("rpcclient") // logger for client package

// TransferParam wraps info of transfer target, type and amount
type TransferParam struct {
	addr    types.Address
	isToken bool
	amount  uint64
	token   *types.OutPoint
	// only used for split addr
	addrs   []types.Address
	weights []uint64
}

func (tp *TransferParam) getScript() ([]byte, error) {
	if tp.addrs != nil {
		// creating split address
		return getSplitAddrScript(tp.addrs, tp.weights)
	}
	if tp.isToken {
		if tp.token == nil {
			return nil, fmt.Errorf("token type needs to be filled")
		}
		return getTransferTokenScript(tp.addr.Hash(), &tp.token.Hash, tp.token.Index, tp.amount)
	}
	return getScriptAddressFromPubKeyHash(tp.addr.Hash())
}

func (tp *TransferParam) getTxOut() (*corepb.TxOut, error) {
	script, err := tp.getScript()
	if err != nil {
		return nil, err
	}
	if tp.isToken {
		return &corepb.TxOut{
			Value:        dustLimit,
			ScriptPubKey: script,
		}, nil
	}
	return &corepb.TxOut{
		Value:        tp.amount,
		ScriptPubKey: script,
	}, nil
}

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
func getIssueTokenScript(pubKeyHash []byte, tokenName, tokenSymbol string, tokenTotalSupply uint64, tokenDecimals uint8) ([]byte, error) {
	addr, err := types.NewAddressPubKeyHash(pubKeyHash)
	if err != nil {
		return nil, err
	}
	issueParams := &script.IssueParams{
		Name:        tokenName,
		Symbol:      tokenSymbol,
		TotalSupply: tokenTotalSupply,
		Decimals:    tokenDecimals,
	}
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

func getSplitAddrScript(addrs []types.Address, weights []uint64) ([]byte, error) {
	script := script.SplitAddrScript(addrs, weights)
	return *script, nil
}

func extractTokenInfo(utxo *rpcpb.Utxo) (*types.OutPoint, uint64) {
	script := script.NewScriptFromBytes(utxo.TxOut.ScriptPubKey)
	if script.IsTokenIssue() {
		issueParam, err := script.GetIssueParams()
		if err == nil {
			outHash := crypto.HashType{}
			outHash.SetBytes(utxo.OutPoint.Hash)
			return &types.OutPoint{Hash: outHash, Index: utxo.OutPoint.Index}, issueParam.TotalSupply * uint64(math.Pow10(int(issueParam.Decimals)))
		}
	}
	if script.IsTokenTransfer() {
		transferParam, err := script.GetTransferParams()
		if err == nil {
			return &transferParam.OutPoint, transferParam.Amount
		}
	}
	return nil, 0
}

func generateTx(fromAddr types.Address, utxos []*rpcpb.Utxo, targets []*TransferParam, change *corepb.TxOut) (*corepb.Transaction, error) {
	tx := &corepb.Transaction{}
	var inputAmount, outputAmount uint64
	tokenAmounts := make(map[types.OutPoint]uint64)
	txIn := make([]*corepb.TxIn, len(utxos))
	for i, utxo := range utxos {
		txIn[i] = &corepb.TxIn{
			PrevOutPoint: &corepb.OutPoint{
				Hash:  utxo.GetOutPoint().Hash,
				Index: utxo.GetOutPoint().GetIndex(),
			},
			ScriptSig: []byte{},
			Sequence:  0,
		}
		tokenInfo, amount := extractTokenInfo(utxo)
		if tokenInfo != nil && amount > 0 {
			if val, ok := tokenAmounts[*tokenInfo]; ok {
				tokenAmounts[*tokenInfo] = amount + val
			} else {
				tokenAmounts[*tokenInfo] = amount
			}
		}
		inputAmount += utxo.GetTxOut().GetValue()
	}
	tx.Vin = txIn
	vout := make([]*corepb.TxOut, 0)
	for _, param := range targets {
		if param.isToken {
			val, ok := tokenAmounts[*param.token]
			if !ok || val < param.amount {
				return nil, fmt.Errorf("Not enough token balance")
			}
			tokenAmounts[*param.token] = val - param.amount
		}

		txOut, err := param.getTxOut()
		if err != nil {
			return nil, err
		}
		vout = append(vout, txOut)
	}

	// generate token change
	for token, amount := range tokenAmounts {
		if amount > 0 {
			tokenChangeScript := script.TransferTokenScript(fromAddr.Hash(), &script.TransferParams{
				TokenID: script.TokenID{
					OutPoint: types.OutPoint{
						Hash:  token.Hash,
						Index: token.Index,
					},
				},
				Amount: amount,
			})

			tokenChange := &corepb.TxOut{
				Value:        dustLimit,
				ScriptPubKey: *tokenChangeScript,
			}
			vout = append(vout, tokenChange)
		}
	}

	if inputAmount > outputAmount && change != nil {
		vout = append(vout, change)
	}
	tx.Vout = vout

	return tx, nil
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

// tryBalance calculate mining fee of a transaction. if txIn of transaction has enough box coins to cover
// write the change amount to change txOut, and returns (true, 0); if not, returns (false, newAmountNeeded)
// note: param change must be an element of the transacton vout
func tryBalance(tx *corepb.Transaction, change *corepb.TxOut, utxos []*rpcpb.Utxo, pricePerByte uint64) (bool, uint64) {
	var totalBytes int
	var totalIn, totalOut uint64
	for _, vin := range tx.Vin {
		totalBytes += len(vin.ScriptSig)
	}
	for _, utxo := range utxos {
		totalIn += utxo.TxOut.GetValue()
	}
	for _, vout := range tx.Vout {
		totalBytes += len(vout.ScriptPubKey)
		if vout != change {
			totalOut += vout.GetValue()
		}
	}
	totalFee := uint64(totalBytes) * pricePerByte
	if totalFee+totalOut < totalIn {
		change.Value = totalIn - totalFee - totalOut
		return true, 0
	} else if totalFee+totalOut == totalIn {
		// when transaction fee exactly matches, and change is not needed
		// ignore the change output will be more efficient
		// notice: change output must be the last element
		tx.Vout = tx.Vout[:len(tx.Vout)-1]
		return true, 0
	}
	return false, totalFee + totalOut
}

func generateTokenIssueTransaction(issueScript []byte, utxos []*rpcpb.Utxo, change *corepb.TxOut) *corepb.Transaction {
	txIn := make([]*corepb.TxIn, len(utxos))
	for i, utxo := range utxos {
		txIn[i] = &corepb.TxIn{
			PrevOutPoint: &corepb.OutPoint{
				Hash:  utxo.GetOutPoint().Hash,
				Index: utxo.GetOutPoint().GetIndex(),
			},
			ScriptSig: []byte{},
			Sequence:  uint32(0),
		}
	}
	tx := &corepb.Transaction{}
	tx.Vin = txIn
	tx.Vout = []*corepb.TxOut{
		{
			Value:        dustLimit,
			ScriptPubKey: issueScript,
		},
		change,
	}
	return tx
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
