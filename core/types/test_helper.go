// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/crypto"
)

// NewBlockHeader generates a new BlockHeader
func NewBlockHeader(prevBlockHash crypto.HashType, txsRoot crypto.HashType, timestamp int64) *BlockHeader {
	return &BlockHeader{
		Version:       0,
		PrevBlockHash: prevBlockHash,
		TxsRoot:       txsRoot,
		TimeStamp:     timestamp,
		Magic:         0,
	}
}

// NewBlocks generates a new Block
func NewBlocks(prevBlockHash crypto.HashType, txsRoot crypto.HashType, timestamp int64, prevOutPoint OutPoint, value uint64, lockTime int64, height uint32) *Block {
	return &Block{
		Header: NewBlockHeader(prevBlockHash, txsRoot, timestamp),
		Txs:    []*Transaction{NewTransaction(prevOutPoint, value, lockTime)},
		Height: height,
	}
}

// NewTxOut generates a new TxOut
func NewTxOut(value uint64) *corepb.TxOut {
	return &corepb.TxOut{
		Value:        value,
		ScriptPubKey: []byte{0},
	}
}

// NewTxIn generates a new TxIn
func NewTxIn(prevOutPoint OutPoint) *TxIn {
	return &TxIn{
		PrevOutPoint: prevOutPoint,
		ScriptSig:    []byte{0},
		Sequence:     1,
	}
}

// NewOutPoint generates a new OutPoint
func NewOutPoint(hash crypto.HashType) *OutPoint {
	return &OutPoint{
		Hash:  hash,
		Index: 1,
	}
}

// NewTransaction generates a new Transaction
func NewTransaction(prevOutPoint OutPoint, value uint64, lockTime int64) *Transaction {
	return &Transaction{
		Version:  0,
		Vin:      []*TxIn{NewTxIn(prevOutPoint)},
		Vout:     []*corepb.TxOut{NewTxOut(value)},
		Magic:    0,
		LockTime: lockTime,
	}
}
