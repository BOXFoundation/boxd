// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"fmt"
	"math"
	"reflect"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
)

const (
	// min output value
	dustLimit = 1
)

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
	sc, err := tp.getScript()
	if err != nil {
		return nil, err
	}
	if tp.isToken {
		return &corepb.TxOut{
			Value:        dustLimit,
			ScriptPubKey: sc,
		}, nil
	}
	return &corepb.TxOut{
		Value:        tp.amount,
		ScriptPubKey: sc,
	}, nil
}

func getSplitAddrScript(addrs []types.Address, weights []uint64) ([]byte, error) {
	sc := script.SplitAddrScript(addrs, weights)
	return *sc, nil
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

	sc := script.TransferTokenScript(addr.Hash(), transferParams)
	return *sc, nil
}

func getScriptAddressFromPubKeyHash(pubKeyHash []byte) ([]byte, error) {
	addr, err := types.NewAddressPubKeyHash(pubKeyHash)
	if err != nil {
		return nil, err
	}
	return *script.PayToPubKeyHashScript(addr.Hash()), nil
}

func getScriptAddress(address types.Address) []byte {
	return *script.PayToPubKeyHashScript(address.Hash())
}

// CreateSplitAddrTransaction creates a split address using input addresses and weights
func CreateSplitAddrTransaction(helper TxGenerateHelper, fromAddr types.Address, pubKeyBytes []byte,
	addrs []types.Address, weights []uint64, signer crypto.Signer) (*types.Transaction, error) {
	t := &TransferParam{
		addr:    fromAddr,
		isToken: false,
		amount:  0,
		token:   nil,
		addrs:   addrs,
		weights: weights,
	}
	return CreateTransactionFromTransferParam(helper, fromAddr, pubKeyBytes, []*TransferParam{t}, signer)
}

// CreateTransaction retrieves all the utxo of a public key, and use some of them to send transaction
func CreateTransaction(helper TxGenerateHelper, fromAddress types.Address, targets map[types.Address]uint64,
	pubKeyBytes []byte, signer crypto.Signer) (*types.Transaction, error) {
	transferTargets := make([]*TransferParam, 0, len(targets))
	for addr, amount := range targets {
		transferTargets = append(transferTargets, &TransferParam{
			addr:    addr,
			isToken: false,
			amount:  amount,
			token:   nil,
		})
	}
	return CreateTransactionFromTransferParam(helper, fromAddress, pubKeyBytes, transferTargets, signer)
}

// CreateTransactionFromTransferParam creates transaction from processed transferparam arrays
func CreateTransactionFromTransferParam(helper TxGenerateHelper, fromAddr types.Address, pubKeyBytes []byte,
	targets []*TransferParam, signer crypto.Signer) (*types.Transaction, error) {
	var totalAmount uint64
	for _, t := range targets {
		totalAmount += t.amount
	}
	change := &corepb.TxOut{
		Value:        0,
		ScriptPubKey: getScriptAddress(fromAddr),
	}

	price, err := helper.GetFee()
	if err != nil {
		return nil, err
	}

	var tx *types.Transaction
	for {
		utxos, err := helper.Fund(fromAddr, totalAmount)
		//utxoResponse, err := FundTransaction(conn, fromAddress, totalAmount)
		if err != nil {
			return nil, err
		}
		if tx, err = generateTx(fromAddr, utxos, targets, change); err != nil {
			return nil, err
		}
		if err = signTransaction(tx, utxos, pubKeyBytes, signer); err != nil {
			return nil, err
		}
		ok, adjustedAmount := tryBalance(tx, change, utxos, price)
		if ok {
			// error is ignored because it's the second sign action
			signTransaction(tx, utxos, pubKeyBytes, signer)
			break
		}
		totalAmount = adjustedAmount
	}

	return tx, nil
}

// tryBalance calculate mining fee of a transaction. if txIn of transaction has enough box coins to cover
// write the change amount to change txOut, and returns (true, 0); if not, returns (false, newAmountNeeded)
// note: param change must be an element of the transacton vout
func tryBalance(tx *types.Transaction, change *corepb.TxOut, utxos map[types.OutPoint]*types.UtxoWrap, pricePerByte uint64) (bool, uint64) {
	var totalBytes int
	var totalIn, totalOut uint64
	for _, vin := range tx.Vin {
		totalBytes += len(vin.ScriptSig)
	}
	for _, utxo := range utxos {
		totalIn += utxo.Output.GetValue()
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

func signTransaction(tx *types.Transaction, utxos map[types.OutPoint]*types.UtxoWrap, fromPubKeyBytes []byte, signer crypto.Signer) error {
	// Sign the tx inputs
	//typedTx := &types.Transaction{}
	//if err := typedTx.FromProtoMessage(tx); err != nil {
	//	return err
	//}
	for txInIdx, txIn := range tx.Vin {
		prevScriptPubKeyBytes, err := findUtxoScriptPubKey(utxos, txIn.PrevOutPoint)
		if err != nil {
			return err
		}
		prevScriptPubKey := script.NewScriptFromBytes(prevScriptPubKeyBytes)
		sigHash, err := script.CalcTxHashForSig(prevScriptPubKeyBytes, tx, txInIdx)
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
		if err = script.Validate(scriptSig, prevScriptPubKey, tx, txInIdx); err != nil {
			return err
		}
	}
	return nil
}

// find an outpoint's referenced utxo's scriptPubKey
func findUtxoScriptPubKey(utxos map[types.OutPoint]*types.UtxoWrap, outPoint types.OutPoint) ([]byte, error) {
	for op, utxo := range utxos {
		if reflect.DeepEqual(op, outPoint) {
			return utxo.Output.GetScriptPubKey(), nil
		}
	}
	return nil, fmt.Errorf("outPoint's referenced utxo not found")
}

func generateTx(fromAddr types.Address, utxos map[types.OutPoint]*types.UtxoWrap, targets []*TransferParam,
	change *corepb.TxOut) (*types.Transaction, error) {
	tx := &types.Transaction{}
	var inputAmount, outputAmount uint64
	tokenAmounts := make(map[types.OutPoint]uint64)
	txIn := make([]*types.TxIn, len(utxos))
	var i uint32
	for op, wrap := range utxos {
		txIn[i] = &types.TxIn{
			PrevOutPoint: types.OutPoint{
				Hash:  op.Hash,
				Index: op.Index,
			},
			ScriptSig: []byte{},
			Sequence:  0,
		}
		tokenInfo, amount := extractTokenInfo(op, wrap)
		if tokenInfo != nil && amount > 0 {
			if val, ok := tokenAmounts[*tokenInfo]; ok {
				tokenAmounts[*tokenInfo] = amount + val
			} else {
				tokenAmounts[*tokenInfo] = amount
			}
		}
		inputAmount += wrap.Output.GetValue()
		i++
	}
	tx.Vin = txIn
	vout := make([]*corepb.TxOut, 0)
	for _, param := range targets {
		if param.isToken {
			val, ok := tokenAmounts[*param.token]
			if !ok || val < param.amount {
				return nil, fmt.Errorf("not enough token balance")
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

// extractTokenInfo returns token outpoint and amount of an utxo
func extractTokenInfo(op types.OutPoint, wrap *types.UtxoWrap) (*types.OutPoint, uint64) {
	sc := script.NewScriptFromBytes(wrap.Output.ScriptPubKey)
	if sc.IsTokenIssue() {
		issueParam, err := sc.GetIssueParams()
		if err == nil {
			return &op, issueParam.TotalSupply * uint64(math.Pow10(int(issueParam.Decimals)))
		}
	}
	if sc.IsTokenTransfer() {
		transferParam, err := sc.GetTransferParams()
		if err == nil {
			return &transferParam.OutPoint, transferParam.Amount
		}
	}
	return nil, 0
}
