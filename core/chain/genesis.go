// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"time"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
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
	Vout: []*corepb.TxOut{
		{
			Value:        0x12a05f200,
			ScriptPubKey: []byte{},
		},
	},
	LockTime: 0,
}

var genesisMerkleRoot = crypto.HashType([crypto.HashSize]byte{})

var genesisBlock = types.Block{
	Header: &types.BlockHeader{
		Version:       1,
		PrevBlockHash: crypto.HashType{}, // 0000000000000000000000000000000000000000000000000000000000000000
		TxsRoot:       genesisMerkleRoot,
		TimeStamp:     time.Date(2018, 1, 31, 0, 0, 0, 0, time.UTC).Unix(),
		// ConsensusRoot: initGenesisConsensusContext(),
	},
	Txs:    []*types.Transaction{&genesisCoinbaseTx},
	Height: 0,
}

// GenesisHash is the hash of genesis block
var GenesisHash = *(genesisBlock.BlockHash())

// var GenesisPeriod = []string{
// 	"b1YVxYU6ANzJ7GNpM2Utv67zDVAidd63fJ1",
// }

// GenesisPeriod genesis period
var GenesisPeriod = []map[string]string{
	{
		"addr":   "b1YVxYU6ANzJ7GNpM2Utv67zDVAidd63fJ1",
		"peerID": "12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cza",
	},
	{
		"addr":   "b1ZF5VWYdHv5RrRzP3SD5huGFyqWNq8rquJ",
		"peerID": "12D3KooWDyVN22wTKTe77pvqSwBuAquPzPvK2NvxUjeYo8SNe6WA",
	},
}

// func initGenesisConsensusContext() crypto.HashType {
// 	hashs := make([]*crypto.HashType, len(GenesisPeriod))
// 	for index := range GenesisPeriod {
// 		hash := crypto.DoubleHashH([]byte(GenesisPeriod[index]))
// 		hashs[index] = &hash
// 	}
// 	merkRootHashs := util.BuildMerkleRoot(hashs)
// 	rootHash := merkRootHashs[len(merkRootHashs)-1]
// 	return *rootHash
// }
