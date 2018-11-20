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

// GenesisBlock represents genesis block.
var GenesisBlock = types.Block{
	Header: &types.BlockHeader{
		Version:       1,
		PrevBlockHash: crypto.HashType{}, // 0000000000000000000000000000000000000000000000000000000000000000
		TxsRoot:       genesisMerkleRoot,
		TimeStamp:     time.Date(2018, 1, 31, 0, 0, 0, 0, time.UTC).Unix(),
	},
	Txs:    []*types.Transaction{&genesisCoinbaseTx},
	Height: 0,
}

// GenesisHash is the hash of genesis block
var GenesisHash = *(GenesisBlock.BlockHash())

// GenesisPeriod genesis period
var GenesisPeriod = []map[string]string{
	{
		"addr":   "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o",
		"peerID": "12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cza",
	},
	{
		"addr":   "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ",
		"peerID": "12D3KooWKPRAK7vBBrVv9szEin55kBnJEEuHG4gDTQEM72ByZDpA",
	},
	{
		"addr":   "b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7",
		"peerID": "12D3KooWSdXLNeoRQQ2a7yiS6xLpTn3LdCr8B8fqPz94Bbi7itsi",
	},
	{
		"addr":   "b1UP5pbfJgZrF1ezoSHLdvkxvgF2BYLtGva",
		"peerID": "12D3KooWRHVAwymCVcA8jqyjpP3r3HBkCW2q5AZRTBvtaungzFSJ",
	},
	{
		"addr":   "b1ZWSdrg48g145VdcmBwMPVuDFdaxDLoktk",
		"peerID": "12D3KooWQSaxCgbWakLcU69f4gmNFMszwhyHbwx4xPAhV7erDC2P",
	},
	{
		"addr":   "b1fRtRnKF4qhQG7bSwqbgR2BMw9VfM2XpT4",
		"peerID": "12D3KooWNcJQzHaNpW5vZDQbTcoLXVCyGS755hTpendGzb5Hqtcu",
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
