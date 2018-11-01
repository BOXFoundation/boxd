// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"os"
	"testing"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/facebookgo/ensure"
	"github.com/jbenet/goprocess"
)

var (
	txOutIdx     = uint32(0)
	txOutIdxErr  = uint32(1)
	value        = uint64(1)
	value1       = uint64(2)
	value2       = uint64(3)
	blockHeight  = uint32(10000)
	blockHeight1 = uint32(20000)
	blockHeight2 = uint32(30000)
	isCoinBase   = false
	isSpent      = false
	isModified   = true

	timestamp     = int64(12345678900000)
	prevBlockHash = crypto.HashType{0x0010}
	txsRoot       = crypto.HashType{0x0022}

	db = NewDB()
)

func createUtxoWrap(val uint64, blkHeight uint32) types.UtxoWrap {
	txOut := &corepb.TxOut{
		Value:        val,
		ScriptPubKey: []byte{},
	}
	utxoWrap := types.UtxoWrap{
		Output:      txOut,
		BlockHeight: blkHeight,
		IsCoinBase:  isCoinBase,
		IsSpent:     isSpent,
		IsModified:  isModified,
	}
	return utxoWrap
}

func createTx(outPointHash crypto.HashType, index uint32, val uint64) *types.Transaction {
	outPoint := types.OutPoint{
		Hash:  outPointHash,
		Index: index,
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

func createOutPoint(txHash crypto.HashType, index uint32) types.OutPoint {
	outPoint := types.OutPoint{
		Hash:  txHash,
		Index: index,
	}
	return outPoint
}

func createBlockHeader(prevBlockHash crypto.HashType, txsRoot crypto.HashType, timestamp int64) *types.BlockHeader {
	return &types.BlockHeader{
		Version:       0,
		PrevBlockHash: prevBlockHash,
		TxsRoot:       txsRoot,
		TimeStamp:     timestamp,
		Magic:         0,
	}
}

func createBlock(head *types.BlockHeader, tx *types.Transaction, height uint32) *types.Block {
	return &types.Block{
		Header: head,
		Txs:    []*types.Transaction{tx},
		Height: height,
	}
}

// NewDB generates a new DB for testing purpose
func NewDB() *storage.Database {
	dbCfg := &storage.Config{
		Name: "memdb",
		Path: "~/tmp",
	}

	proc := goprocess.WithSignals(os.Interrupt)
	db, _ := storage.NewDatabase(proc, dbCfg)
	return db
}

func TestUtxoSet_Utxo(t *testing.T) {
	utxoWrapOrigin := createUtxoWrap(value, blockHeight)
	utxoWrapNew := createUtxoWrap(value1, blockHeight1)
	utxoWrap2 := createUtxoWrap(value2, blockHeight2)

	tx := createTx(crypto.HashType{0x0010}, txOutIdx, value)
	txHash, _ := tx.TxHash()
	outPointOrigin := createOutPoint(*txHash, txOutIdx)

	outPoint1 := createOutPoint(crypto.HashType{0x0020}, txOutIdx)
	tx1 := createTx(outPoint1.Hash, txOutIdx, value1)
	tx1Hash, _ := tx1.TxHash()
	outPointNew := createOutPoint(*tx1Hash, txOutIdx)

	tx2 := createTx(crypto.HashType{0x0022}, txOutIdx, value2)
	tx2Hash, _ := tx2.TxHash()
	outPoint2 := createOutPoint(*tx2Hash, txOutIdx)

	// non-existing utxoSet for this outpoint
	outPointErr := createOutPoint(crypto.HashType{0x0050}, txOutIdx)

	utxoSet := NewUtxoSet()

	err := utxoSet.AddUtxo(tx, txOutIdx, blockHeight)
	ensure.Nil(t, err)

	err = utxoSet.AddUtxo(tx1, txOutIdx, blockHeight1)
	ensure.Nil(t, err)

	// test for ErrTxOutIndexOob
	err = utxoSet.AddUtxo(tx1, txOutIdxErr, blockHeight)
	ensure.NotNil(t, err)
	ensure.DeepEqual(t, err, core.ErrTxOutIndexOob)

	// test for ErrAddExistingUtxo
	err = utxoSet.AddUtxo(tx, txOutIdx, blockHeight)
	ensure.NotNil(t, err)
	ensure.DeepEqual(t, err, core.ErrAddExistingUtxo)

	result := *utxoSet.FindUtxo(outPointOrigin)
	ensure.DeepEqual(t, result, utxoWrapOrigin)

	result = *utxoSet.FindUtxo(outPointNew)
	ensure.DeepEqual(t, result, utxoWrapNew)

	// test for non-existing utxoSet
	errRes := utxoSet.FindUtxo(outPointErr)
	ensure.DeepEqual(t, errRes, (*types.UtxoWrap)(nil))

	// spend outPointOrigin
	utxoSet.SpendUtxo(outPointOrigin)
	spendResult := utxoSet.FindUtxo(outPointOrigin)
	ensure.DeepEqual(t, true, spendResult.IsSpent)

	// test for applyTx
	errApplyTx := utxoSet.ApplyTx(tx2, blockHeight2)
	ensure.Nil(t, errApplyTx)
	resApplyTx := utxoSet.FindUtxo(outPoint2)
	ensure.DeepEqual(t, *resApplyTx, utxoWrap2)

	table, _ := db.Storage.Table("test")

	// test LoadTxUtxos
	err = utxoSet.LoadTxUtxos(tx, table)
	ensure.Nil(t, err)

	// test WriteUtxoSetToDB (empty map to free memory)
	errStorage := utxoSet.WriteUtxoSetToDB(table)
	ensure.Nil(t, errStorage)

	utxoKeyNew := UtxoKey(&outPointNew)
	serialized, _ := table.Get(utxoKeyNew)
	err = utxoWrapNew.Unmarshal(serialized)
	ensure.Nil(t, err)

	// test fetchUtxosFromOutPointSet
	input := make(map[types.OutPoint]struct{})
	err = utxoSet.fetchUtxosFromOutPointSet(input, table)
	ensure.Nil(t, err)

	//test LoadBlockUtxos
	err = utxoSet.LoadBlockUtxos(&genesisBlock, table)
	ensure.Nil(t, err)

	head := createBlockHeader(prevBlockHash, txsRoot, timestamp)
	block := createBlock(head, tx, blockHeight)

	err = utxoSet.LoadBlockUtxos(block, table)
	ensure.Nil(t, err)

	// test fetchUtxoWrapFromDB
	utxoWrap, _ := utxoSet.fetchUtxoWrapFromDB(table, outPointNew)
	ensure.DeepEqual(t, *utxoWrap, utxoWrapNew)

}
