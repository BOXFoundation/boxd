// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"testing"

	"github.com/BOXFoundation/boxd/crypto"
	"github.com/facebookgo/ensure"
)

func TestBlockCovertWithProtoMessage(t *testing.T) {
	var prevBlockHash = crypto.HashType{0x0010}
	var txsRoot = crypto.HashType{0x0022}
	var timestamp int64 = 12345678900000
	var prevOutPoint = NewOutPoint(crypto.HashType{0x0012})
	var value int64 = 111111
	var lockTime int64 = 19871654300000000
	var height int32 = 10
	var txs = []*Transaction{}
	block := NewBlocks(prevBlockHash, txsRoot, timestamp, *prevOutPoint, value, lockTime, height)
	block1 := &Block{}
	msg, err := block.ToProtoMessage()
	ensure.Nil(t, err)
	err = block1.FromProtoMessage(msg)
	ensure.Nil(t, err)
	// remove hash due to FromProtoMessage not convert hash in block
	block1.Hash = nil
	// set transactions as empty to skip compare transaction part
	block.Txs = txs
	block1.Txs = txs
	ensure.DeepEqual(t, block, block1)
}

func TestBlockHeaderConvertWithProtoMessage(t *testing.T) {
	var prevBlockHash = crypto.HashType{0x0011}
	var txsRoot = crypto.HashType{0x0020}
	var timestamp int64 = 98765432100000
	header := NewBlockHeader(prevBlockHash, txsRoot, timestamp)
	var header1 = &BlockHeader{}
	msg, err := header.ToProtoMessage()
	ensure.Nil(t, err)
	err = header1.FromProtoMessage(msg)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, header, header1)
}
