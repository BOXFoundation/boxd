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

var (
	txOutIdx     = uint32(0)
	txOutIdxErr  = uint32(1)
	value        = uint64(1)
	value1       = uint64(2)
	blockHeight  = uint32(10000)
	blockHeight1 = uint32(20000)
	isCoinBase   = false
	isSpent      = false
	isModified   = true
)

func createUtxoWrap(val uint64, blkHeight uint32) *types.UtxoWrap {

	utxoWrap := types.NewUtxoWrap(val, []byte{}, blkHeight)
	return utxoWrap
}

func createTx(outPointHash crypto.HashType, val uint64) *types.Transaction {
	outPoint := types.OutPoint{
		Hash:  outPointHash,
		Index: txOutIdx,
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
		Value:        val,
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
	return tx
}

func createOutPoint(txHash crypto.HashType) types.OutPoint {
	outPoint := types.OutPoint{
		Hash:  txHash,
		Index: txOutIdx,
	}
	return outPoint
}

func TestUtxoSet_FindUtxo(t *testing.T) {
	utxoWrapOrigin := createUtxoWrap(value, blockHeight)
	utxoWrapNew := createUtxoWrap(value1, blockHeight1)

	tx := createTx(crypto.HashType{0x0010}, value)
	txHash, _ := tx.TxHash()
	outPointOrigin := createOutPoint(*txHash)

	outPoint1 := createOutPoint(crypto.HashType{0x0020})
	tx1 := createTx(outPoint1.Hash, value1)
	tx1Hash, _ := tx1.TxHash()
	outPointNew := createOutPoint(*tx1Hash)

	// non-existing utxoSet for this outpoint
	outPointErr := createOutPoint(crypto.HashType{0x0050})

	utxoSet := NewUtxoSet()

	err := utxoSet.AddUtxo(tx, txOutIdx, blockHeight)
	ensure.Nil(t, err)

	err2 := utxoSet.AddUtxo(tx1, txOutIdx, blockHeight1)
	ensure.Nil(t, err2)

	// test for ErrTxOutIndexOob
	err3 := utxoSet.AddUtxo(tx1, txOutIdxErr, blockHeight)
	ensure.NotNil(t, err3)
	ensure.DeepEqual(t, err3, core.ErrTxOutIndexOob)

	// test for ErrAddExistingUtxo
	err4 := utxoSet.AddUtxo(tx, txOutIdx, blockHeight)
	ensure.NotNil(t, err4)
	ensure.DeepEqual(t, err4, core.ErrAddExistingUtxo)

	result := utxoSet.FindUtxo(outPointOrigin)
	ensure.DeepEqual(t, result, utxoWrapOrigin)

	result1 := utxoSet.FindUtxo(outPointNew)
	ensure.DeepEqual(t, result1, utxoWrapNew)

	// test for non-existing utxoSet
	errRes := utxoSet.FindUtxo(outPointErr)
	ensure.DeepEqual(t, errRes, (*types.UtxoWrap)(nil))

	utxoSet.SpendUtxo(outPointOrigin)
	spendResult := utxoSet.FindUtxo(outPointOrigin)
	ensure.DeepEqual(t, true, spendResult.IsSpent())
}
