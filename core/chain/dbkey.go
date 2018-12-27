// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"fmt"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage/key"
)

const (
	readable = true // readable db key or compact key
)

const (
	// BlockTableName is the table name of db to store block chain data
	BlockTableName = "core"

	// WalletTableName is the table name of db to store wallet data
	WalletTableName = "wl"

	// Tail is the db key name of tail block
	Tail = "/tail"

	// Genesis is the db key name of genesis block
	Genesis = "/genesis"

	// Eternal is the db key name of eternal block
	Eternal = "/eternal"

	// Period is the db key name of current period
	Period = "/period/current"

	// BlockPrefix is the key prefix of database key to store block content
	// /bk/{hex encoded block hash}
	// e.g.
	// key: /bk/005973c44c4879b137c3723c96d2e341eeaf83fe58845b2975556c9f3bd640bb
	// value: block binary
	BlockPrefix = "/bk"

	// BlockHashPrefix is the key prefix of database key to store block hash of specified height
	// /bh/{hex encoded height}
	//e.g.
	// key: /bh/3e2d
	// value: block hash binary
	BlockHashPrefix = "/bh"

	// TxIndexPrefix is the key prefix of database key to store tx index
	// /ti/{hex encoded tx hash}
	// e.g.
	// key: /ti/1113b8bdad74cdc045e64e09b3e2f0502d1b7f9bd8123b28239a3360bd3a8757
	// value: 4 bytes height + 4 bytes index in txs
	TxIndexPrefix = "/ti"

	// UtxoPrefix is the key prefix of database key to store utxo content
	// /ut/{hex encoded tx hash}/{vout index}
	// e.g.
	// key: /ut/1113b8bdad74cdc045e64e09b3e2f0502d1b7f9bd8123b28239a3360bd3a8757/2
	// value: utxo wrapper
	UtxoPrefix = "/ut"

	// CandidatesPrefix is the key prefix of database key to store candidates
	CandidatesPrefix = "/candidates"
	// FilterPrefix is the key prefix of block bloom filter to store a filter bytes
	// /bf/{hex encoded block hash}
	// e.g.
	// key: /bf/1113b8bdad74cdc045e64e09b3e2f0502d1b7f9bd8123b28239a3360bd3a8757
	// value: crypto hash
	FilterPrefix = "/bf"
	// SplitAddressPrefix is the key prefix of split address
	SplitAddressPrefix = "/sap"

	// AddressUtxoPrefix is the key prefix of database key to store address related utxo
	AddressUtxoPrefix = "/aut"

	// AddrBalancePrefix is the key prefix of database key to store address box balance
	AddrBalancePrefix = "/bal"
	// AddrTokenBalancePrefix is the key prefix of database key to store address token balance
	AddrTokenBalancePrefix = "/tbal"
)

var blkBase = key.NewKey(BlockPrefix)
var blkHashBase = key.NewKey(BlockHashPrefix)
var txixBase = key.NewKey(TxIndexPrefix)
var utxoBase = key.NewKey(UtxoPrefix)
var candidatesBase = key.NewKey(CandidatesPrefix)
var filterBase = key.NewKey(FilterPrefix)
var splitAddrBase = key.NewKey(SplitAddressPrefix)
var addrUtxoBase = key.NewKey(AddressUtxoPrefix)
var addrBalanceBase = key.NewKey(AddrBalancePrefix)
var addrTokenBalanceBase = key.NewKey(AddrTokenBalancePrefix)

// TailKey is the db key to stoare tail block content
var TailKey = []byte(Tail)

// GenesisKey is the db key to stoare genesis block content
var GenesisKey = []byte(Genesis)

// EternalKey is the db key to stoare eternal block content
var EternalKey = []byte(Eternal)

// PeriodKey is the db key to stoare current period contex content
var PeriodKey = []byte(Period)

// BlockKey returns the db key to stoare block content of the hash
func BlockKey(h *crypto.HashType) []byte {
	return blkBase.ChildString(h.String()).Bytes()
}

// BlockHashKey returns the db key to stoare block hash content of the height
func BlockHashKey(height uint32) []byte {
	return blkHashBase.ChildString(fmt.Sprintf("%x", height)).Bytes()
}

// TxIndexKey returns the db key to stoare tx index of the hash
func TxIndexKey(h *crypto.HashType) []byte {
	return txixBase.ChildString(h.String()).Bytes()
}

// UtxoKey returns the db key to stoare utxo content of the Outpoint
func UtxoKey(op *types.OutPoint) []byte {
	return utxoBase.ChildString(op.Hash.String()).ChildString(fmt.Sprintf("%x", op.Index)).Bytes()
}

// CandidatesKey returns the db key to stoare candidates.
func CandidatesKey(h *crypto.HashType) []byte {
	return candidatesBase.ChildString(h.String()).Bytes()
}

// FilterKey returns the db key to store bloom filter of block
func FilterKey(hash crypto.HashType) []byte {
	if readable {
		return filterBase.ChildString(hash.String()).Bytes()
	}
	buf := filterBase.Base().Bytes()
	buf = append(buf[:], hash.GetBytes()...)
	return buf
}

// SplitAddrKey returns the db key to store split address
func SplitAddrKey(hash []byte) []byte {
	return splitAddrBase.ChildString(fmt.Sprintf("%x", hash)).Bytes()
}

// AddrUtxoKey is the key to store an utxo which belongs to the input param address
func AddrUtxoKey(addr string, op types.OutPoint) []byte {
	return addrUtxoBase.
		ChildString(addr).
		ChildString(op.Hash.String()).
		ChildString(fmt.Sprintf("%x", op.Index)).
		Bytes()
}

// AddrAllUtxoKey is the key prefix to explore all utxos of an address
func AddrAllUtxoKey(addr string) []byte {
	return addrUtxoBase.ChildString(addr).Bytes()
}

// AddrBalanceKey is the key to store an address's balance
func AddrBalanceKey(addr string) []byte {
	return addrBalanceBase.ChildString(addr).Bytes()
}

// AddrTokenBalanceKey is the key to store an address's token balance
func AddrTokenBalanceKey(addr string, token types.OutPoint) []byte {
	return addrTokenBalanceBase.
		ChildString(addr).
		ChildString(token.Hash.String()).
		ChildString(fmt.Sprintf("%x", token.Index)).
		Bytes()
}
