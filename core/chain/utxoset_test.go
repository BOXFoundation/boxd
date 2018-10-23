// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"reflect"
	"testing"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
)

func TestUtxoSet_FindUtxo(t *testing.T) {
	outPoint := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 0,
	}
	txIn := &types.TxIn{
		PrevOutPoint: outPoint,
		ScriptSig:    []byte{},
		Sequence:     0,
	}
	vIn := []*types.TxIn{
		txIn,
	}
	txOut := &corepb.TxOut{
		Value:        1,
		ScriptPubKey: []byte{},
	}
	vOut := []*corepb.TxOut{txOut}
	tx := &types.Transaction{
		Version:  1,
		Vin:      vIn,
		Vout:     vOut,
		Magic:    1,
		LockTime: 0,
	}

	utxoWrapOrigin := types.UtxoWrap{
		Output:      txOut,
		BlockHeight: 10000,
		IsCoinBase:  false,
		IsSpent:     false,
		IsModified:  false,
	}

	txHash, _ := tx.TxHash()
	outPointReq := types.OutPoint{
		Hash:  *txHash,
		Index: 0,
	}

	outPoint1 := types.OutPoint{
		Hash:  crypto.HashType{0x0011},
		Index: 0,
	}
	txIn1 := &types.TxIn{
		PrevOutPoint: outPoint1,
		ScriptSig:    []byte{},
		Sequence:     0,
	}
	vIn1 := []*types.TxIn{
		txIn1,
	}
	txOut1 := &corepb.TxOut{
		Value:        2,
		ScriptPubKey: []byte{},
	}
	vOut1 := []*corepb.TxOut{txOut1}
	tx1 := &types.Transaction{
		Version:  1,
		Vin:      vIn1,
		Vout:     vOut1,
		Magic:    2,
		LockTime: 0,
	}

	utxoWrapNew := types.UtxoWrap{
		Output:      txOut1,
		BlockHeight: int32(20000),
		IsCoinBase:  false,
		IsSpent:     false,
		IsModified:  false,
	}

	tx1Hash, _ := tx1.TxHash()
	outPointNew := types.OutPoint{
		Hash:  *tx1Hash,
		Index: 0,
	}

	outPointErr := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 121,
	}

	utxoSet := NewUtxoSet()

	if err := utxoSet.AddUtxo(tx, 0, 10000); err != nil {
		t.Logf("addUtxo error: %+v", err)
	}

	if err := utxoSet.AddUtxo(tx1, 0, 20000); err != nil {
		t.Logf("addUtxo error: %+v", err)
	}

	// test for ErrTxOutIndexOob
	if err := utxoSet.AddUtxo(tx1, 1, 10000); err != nil {
		t.Logf("addUtxo error: %+v", err)
	}

	// test for ErrAddExistingUtxo
	if err := utxoSet.AddUtxo(tx, 0, 10000); err != nil {
		t.Logf("addUtxo error: %+v", err)
	}

	result := *utxoSet.FindUtxo(outPointReq)

	if !reflect.DeepEqual(result, utxoWrapOrigin) {
		t.Errorf("expected utxoentry is %+v, got %+v", utxoWrapOrigin, result)
	}

	result1 := *utxoSet.FindUtxo(outPointNew)

	if !reflect.DeepEqual(result1, utxoWrapNew) {
		t.Errorf("expected utxoWrapNew is %+v, got %+v", utxoWrapNew, result1)
	}

	// test for non-existing utxoSet
	if utxoSet.FindUtxo(outPointErr) == nil {
		t.Logf("there is no such utxoset exists.")
	}

	utxoSet.SpendUtxo(outPointReq)
	spendResult := utxoSet.FindUtxo(outPointReq)
	if !spendResult.IsSpent {
		t.Errorf("expect IsSpent value is %+v, but got %+v", true, spendResult.IsSpent)
	}
}
