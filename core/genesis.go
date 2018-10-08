// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"time"

	"github.com/BOXFoundation/Quicksilver/core/types"
	"github.com/BOXFoundation/Quicksilver/crypto"
)

var genesisCoinbaseTx = types.Transaction{
	Version: 1,
	Vin: []*types.TxIn{
		{
			PrevOutPoint: types.OutPoint{
				Hash:  crypto.HashType{},
				Index: 0xffffffff,
			},
			ScriptSig: []byte{},
			Sequence:  0xffffffff,
		},
	},
	Vout: []*types.TxOut{
		{
			Value:        0x12a05f200,
			ScriptPubKey: []byte{},
		},
	},
	LockTime: 0,
}

var genesisHash = crypto.HashType([crypto.HashSize]byte{})

var genesisMerkleRoot = crypto.HashType([crypto.HashSize]byte{})

var genesisBlock = types.Block{
	Header: &types.BlockHeader{
		Version:       1,
		PrevBlockHash: crypto.HashType{}, // 0000000000000000000000000000000000000000000000000000000000000000
		TxsRoot:       genesisMerkleRoot,
		TimeStamp:     time.Now().UnixNano(),
	},
	Txs:    []*types.Transaction{&genesisCoinbaseTx},
	Height: 0,
}
