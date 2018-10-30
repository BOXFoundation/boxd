// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"testing"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/facebookgo/ensure"
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
		IsModified:  true,
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
		BlockHeight: uint32(20000),
		IsCoinBase:  false,
		IsSpent:     false,
		IsModified:  true,
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

	err := utxoSet.AddUtxo(tx, 0, 10000)
	ensure.Nil(t, err)

	err2 := utxoSet.AddUtxo(tx1, 0, 20000)
	ensure.Nil(t, err2)

	// test for ErrTxOutIndexOob
	err3 := utxoSet.AddUtxo(tx1, 1, 10000)
	ensure.NotNil(t, err3)
	ensure.DeepEqual(t, err3, core.ErrTxOutIndexOob)

	// test for ErrAddExistingUtxo
	err4 := utxoSet.AddUtxo(tx, 0, 10000)
	ensure.NotNil(t, err4)
	ensure.DeepEqual(t, err4, core.ErrAddExistingUtxo)

	result := *utxoSet.FindUtxo(outPointReq)
	ensure.DeepEqual(t, result, utxoWrapOrigin)

	result1 := *utxoSet.FindUtxo(outPointNew)
	ensure.DeepEqual(t, result1, utxoWrapNew)

	// test for non-existing utxoSet
	errRes := utxoSet.FindUtxo(outPointErr)
	ensure.DeepEqual(t, errRes, (*types.UtxoWrap)(nil))

	utxoSet.SpendUtxo(outPointReq)
	spendResult := utxoSet.FindUtxo(outPointReq)
	ensure.DeepEqual(t, true, spendResult.IsSpent)
}
