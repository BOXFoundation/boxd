// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"encoding/hex"
	"math"
	"strings"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/script"
	acc "github.com/BOXFoundation/boxd/wallet/account"
)

// TokenTag defines token tag
type TokenTag struct {
	Name    string
	Symbol  string
	Decimal uint8
}

// SortByUTXOValue defines a type suited for sort
type SortByUTXOValue []*rpcpb.Utxo

func (x SortByUTXOValue) Len() int           { return len(x) }
func (x SortByUTXOValue) Less(i, j int) bool { return x[i].TxOut.Value < x[j].TxOut.Value }
func (x SortByUTXOValue) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// NewTokenTag news a TokenTag
func NewTokenTag(name, sym string, decimal uint8) *TokenTag {
	return &TokenTag{
		Name:    name,
		Symbol:  sym,
		Decimal: decimal,
	}
}

// MakeVout makes txOut
func MakeVout(addr string, amount uint64) *corepb.TxOut {
	address, _ := types.NewAddress(addr)
	addrPkh, _ := types.NewAddressPubKeyHash(address.Hash())
	addrScript := *script.PayToPubKeyHashScript(addrPkh.Hash())
	return &corepb.TxOut{
		Value:        amount,
		ScriptPubKey: addrScript,
	}
}

// MakeVoutWithSPk makes txOut
func MakeVoutWithSPk(amount uint64, scriptPk []byte) *corepb.TxOut {
	return &corepb.TxOut{
		Value:        amount,
		ScriptPubKey: scriptPk,
	}
}

// MakeVin makes txIn
func MakeVin(utxo *rpcpb.Utxo, seq uint32) *types.TxIn {
	var hash crypto.HashType
	copy(hash[:], utxo.GetOutPoint().Hash)
	return &types.TxIn{
		PrevOutPoint: types.OutPoint{
			Hash:  hash,
			Index: utxo.GetOutPoint().GetIndex(),
		},
		ScriptSig: []byte{},
		Sequence:  seq,
	}
}

// NewUtxoWrap makes a UtxoWrap
func NewUtxoWrap(addr string, height uint32, value uint64) *types.UtxoWrap {
	return &types.UtxoWrap{
		Output:      MakeVout(addr, value),
		BlockHeight: height,
		IsCoinBase:  false,
		IsModified:  true,
		IsSpent:     false,
	}
}

// NewPbOutPoint constructs a OutPoint
func NewPbOutPoint(hash *crypto.HashType, index uint32) *corepb.OutPoint {
	return &corepb.OutPoint{
		Hash:  (*hash)[:],
		Index: index,
	}
}

// NewOutPoint constructs a OutPoint
func NewOutPoint(hash *crypto.HashType, index uint32) *types.OutPoint {
	return &types.OutPoint{
		Hash:  *hash,
		Index: index,
	}
}

// SignTxWithUtxos sign tx with utxo
func SignTxWithUtxos(
	tx *types.Transaction, utxos []*rpcpb.Utxo, acc *acc.Account,
) error {
	for i, utxo := range utxos {
		scriptPkBytes := utxo.GetTxOut().GetScriptPubKey()
		sigHash, err := script.CalcTxHashForSig(scriptPkBytes, tx, i)
		if err != nil {
			return err
		}
		sig, err := acc.Sign(sigHash)
		if err != nil {
			return err
		}
		scriptSig := script.SignatureScript(sig, acc.PublicKey())
		tx.Vin[i].ScriptSig = *scriptSig
	}
	return nil
}

// ExtractTokenInfo extract token info from a utxo
func ExtractTokenInfo(utxo *rpcpb.Utxo) (*types.OutPoint, uint64, bool) {
	script := script.NewScriptFromBytes(utxo.TxOut.ScriptPubKey)
	if script.IsTokenIssue() {
		issueParam, err := script.GetIssueParams()
		if err == nil {
			outHash := crypto.HashType{}
			outHash.SetBytes(utxo.OutPoint.Hash)
			return &types.OutPoint{Hash: outHash, Index: utxo.OutPoint.Index},
				issueParam.TotalSupply * uint64(math.Pow10(int(issueParam.Decimals))),
				true
		}
	}
	if script.IsTokenTransfer() {
		transferParam, err := script.GetTransferParams()
		if err == nil {
			return &transferParam.OutPoint, transferParam.Amount, true
		}
	}
	return nil, 0, false
}

// MakeTokenVout make token tx vout
func MakeTokenVout(addr string, tokenID *types.OutPoint, amount uint64) *corepb.TxOut {
	address, _ := types.NewAddress(addr)
	addrPkh, _ := types.NewAddressPubKeyHash(address.Hash())
	transferParams := &script.TransferParams{}
	transferParams.Hash = tokenID.Hash
	transferParams.Index = tokenID.Index
	transferParams.Amount = amount
	addrScript := *script.TransferTokenScript(addrPkh.Hash(), transferParams)
	return &corepb.TxOut{Value: dustLimit, ScriptPubKey: addrScript}
}

// MakeSplitAddrVout make split addr vout
func MakeSplitAddrVout(addrs []string, weights []uint64) *corepb.TxOut {
	return &corepb.TxOut{
		Value:        0,
		ScriptPubKey: MakeSplitAddrPubkey(addrs, weights),
	}
}

// MakeSplitAddrPubkey make split addr
func MakeSplitAddrPubkey(addrs []string, weights []uint64) []byte {
	addresses := make([]types.Address, len(addrs))
	for i, addr := range addrs {
		addresses[i], _ = types.NewAddress(addr)
	}
	return *script.SplitAddrScript(addresses, weights)
}

// MakeSplitAddr make split addr
func MakeSplitAddr(addrs []string, weights []uint64) (string, error) {
	pk := MakeSplitAddrPubkey(addrs, weights)
	splitAddrScriptStr := script.NewScriptFromBytes(pk).Disasm()
	s := strings.Split(splitAddrScriptStr, " ")
	pubKeyHash, err := hex.DecodeString(s[1])
	if err != nil {
		return "", err
	}
	addr, err := types.NewAddressPubKeyHash(pubKeyHash)
	if err != nil {
		return "", err
	}
	return addr.String(), nil
}
