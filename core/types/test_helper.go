// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/crypto"
)

// NewBlockHeader generates a new BlockHeader
func NewBlockHeader(prevBlockHash crypto.HashType, txsRoot crypto.HashType, timestamp int64, height uint32) *BlockHeader {
	return &BlockHeader{
		Version:       0,
		PrevBlockHash: prevBlockHash,
		TxsRoot:       txsRoot,
		TimeStamp:     timestamp,
		Magic:         0,
		Height:        height,
		Bloom:         CreateReceiptsBloom(nil),
	}
}

// NewBlocks generates a new Block
func NewBlocks(prevBlockHash crypto.HashType, txsRoot crypto.HashType, timestamp int64, prevOutPoint OutPoint, value uint64, lockTime int64, height uint32) *Block {
	return &Block{
		Header: NewBlockHeader(prevBlockHash, txsRoot, timestamp, height),
		Txs:    []*Transaction{NewTransaction(prevOutPoint, value, lockTime)},
	}
}

// NewTransaction generates a new Transaction
func NewTransaction(prevOutPoint OutPoint, value uint64, lockTime int64) *Transaction {
	return &Transaction{
		Version:  0,
		Vin:      []*TxIn{NewTxIn(&prevOutPoint, nil, 0)},
		Vout:     []*corepb.TxOut{NewTxOut(value, nil)},
		Magic:    0,
		LockTime: lockTime,
	}
}
