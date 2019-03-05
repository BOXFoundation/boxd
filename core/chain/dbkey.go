// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"fmt"
	"math"
	"strconv"

	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage/key"
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

	// MissCount is the db key name of minuer's block miss rate data
	// value: 4 bytes height + 4 bytes miss count + 10 bytes ts
	MissCount = "/missrate"

	// BlockPrefix is the key prefix of database key to store block content
	// /bk/{hex encoded block hash}
	// e.g.
	// key: /bk/005973c44c4879b137c3723c96d2e341eeaf83fe58845b2975556c9f3bd640bb
	// value: block binary
	BlockPrefix = "/bk"

	// TxPrefix is the key prefix of database key to store tx content
	TxPrefix = "/tx"

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

	// AddressTokenUtxoPrefix is the key prefix of database key to store address related token utxo
	AddressTokenUtxoPrefix = "/atut"

	// AddrBalancePrefix is the key prefix of database key to store address box balance
	AddrBalancePrefix = "/bal"
	// AddrTokenBalancePrefix is the key prefix of database key to store address token balance
	AddrTokenBalancePrefix = "/tbal"
)

var blkBase = key.NewKey(BlockPrefix)
var txBase = key.NewKey(TxPrefix)
var blkHashBase = key.NewKey(BlockHashPrefix)
var txixBase = key.NewKey(TxIndexPrefix)
var utxoBase = key.NewKey(UtxoPrefix)
var candidatesBase = key.NewKey(CandidatesPrefix)
var filterBase = key.NewKey(FilterPrefix)
var splitAddrBase = key.NewKey(SplitAddressPrefix)
var addrUtxoBase = key.NewKey(AddressUtxoPrefix)
var addrTokenUtxoBase = key.NewKey(AddressTokenUtxoPrefix)
var addrBalanceBase = key.NewKey(AddrBalancePrefix)
var addrTokenBalanceBase = key.NewKey(AddrTokenBalancePrefix)

// TailKey is the db key to store tail block content
var TailKey = []byte(Tail)

// GenesisKey is the db key to store genesis block content
var GenesisKey = []byte(Genesis)

// EternalKey is the db key to store eternal block content
var EternalKey = []byte(Eternal)

// PeriodKey is the db key to store current period contex content
var PeriodKey = []byte(Period)

// MissrateKey is the db key to store miner's blocks miss rate
var MissrateKey = []byte(MissCount)

// BlockKey returns the db key to store block content of the hash
func BlockKey(h *crypto.HashType) []byte {
	return blkBase.ChildString(h.String()).Bytes()
}

// TxKey returns the db key to store tx content of the hash
func TxKey(h *crypto.HashType) []byte {
	return txBase.ChildString(h.String()).Bytes()
}

// BlockHashKey returns the db key to store block hash content of the height
func BlockHashKey(height uint32) []byte {
	return blkHashBase.ChildString(fmt.Sprintf("%x", height)).Bytes()
}

// BlockHeightFromBlockHashKey parse height info from a BlockHashKey
func BlockHeightFromBlockHashKey(k []byte) uint32 {
	key := key.NewKeyFromBytes(k)
	segs := key.List()
	if len(segs) > 1 {
		num, err := strconv.ParseUint(segs[1], 16, 32)
		if err == nil {
			return uint32(num)
		}
		logger.Infof("error parsing key: %s", segs[1])
	}
	return math.MaxUint32
}

// BlockHeightPrefix returns the db key prefix to store hash content of the height
func BlockHeightPrefix() []byte {
	return blkHashBase.Bytes()
}

// TxIndexKey returns the db key to store tx index of the hash
func TxIndexKey(h *crypto.HashType) []byte {
	return txixBase.ChildString(h.String()).Bytes()
}

// UtxoKey returns the db key to store utxo content of the Outpoint
// func UtxoKey(op *types.OutPoint) []byte {
// 	return utxoBase.ChildString(op.Hash.String()).ChildString(fmt.Sprintf("%x", op.Index)).Bytes()
// }

// CandidatesKey returns the db key to store candidates.
func CandidatesKey(h *crypto.HashType) []byte {
	return candidatesBase.ChildString(h.String()).Bytes()
}

// FilterKey returns the db key to store bloom filter of block
func FilterKey(height uint32, hash crypto.HashType) []byte {
	return filterBase.
		ChildString(fmt.Sprintf("%x", height)).
		ChildString(hash.String()).
		Bytes()
}

// FilterKeyPrefix returns the db key prefix to store bloom filter of block
func FilterKeyPrefix() []byte {
	return filterBase.Bytes()
}

// FilterHeightHashFromKey parse height and hash info from a FilterKey
func FilterHeightHashFromKey(k []byte) (uint32, string) {
	key := key.NewKeyFromBytes(k)
	segs := key.List()
	if len(segs) > 2 {
		num, err := strconv.ParseUint(segs[1], 16, 32)
		if err == nil {
			return uint32(num), segs[2]
		}
		logger.Infof("error parsing key. num: %s, segs: %v", segs[1], segs)
	}
	return math.MaxUint32, ""
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

// AddrTokenUtxoKey is the key to store an token utxo which belongs to the input param address
func AddrTokenUtxoKey(addr string, tid txlogic.TokenID, op types.OutPoint) []byte {
	return addrTokenUtxoBase.ChildString(addr).ChildString(tid.Hash.String()).
		ChildString(fmt.Sprintf("%x", tid.Index)).ChildString(op.Hash.String()).
		ChildString(fmt.Sprintf("%x", op.Index)).Bytes()
}

// AddrAllTokenUtxoKey is the key prefix to explore all token utxos of an address
func AddrAllTokenUtxoKey(addr string, tid txlogic.TokenID) []byte {
	return addrTokenUtxoBase.ChildString(addr).ChildString(tid.Hash.String()).
		ChildString(fmt.Sprintf("%x", tid.Index)).Bytes()
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
