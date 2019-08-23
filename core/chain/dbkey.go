// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"encoding/hex"
	"fmt"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage/key"
)

const (
	// BlockTableName is the table name of db to store block chain data
	BlockTableName = "core"

	// WalletTableName is the table name of db to store wallet data
	WalletTableName = "wl"

	// SectionTableName is the table name of db to store section data
	SectionTableName = "sec"

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

	// Section is the db key name of section manager's section
	Section = "/sec"

	// BlockPrefix is the key prefix of database key to store block content
	// /bk/{hex encoded block hash}
	// e.g.
	// key: /bk/005973c44c4879b137c3723c96d2e341eeaf83fe58845b2975556c9f3bd640bb
	// value: block binary
	BlockPrefix = "/bk"

	// TxPrefix is the key prefix of database key to store tx content
	TxPrefix = "/tx"

	// SecBloomBitSetPrefix is the key prefix of database key to store bloom bit set of section
	SecBloomBitSetPrefix = "/sec/bf"

	// SplitTxHashPrefix is the key prefix of database key to store split tx mapping
	SplitTxHashPrefix = "/split"

	// BlockHashPrefix is the key prefix of database key to store block hash of specified height
	// /bh/{hex encoded height}
	//e.g.
	// key: /bh/3e2d
	// value: block hash binary
	BlockHashPrefix = "/bh"

	// ReceiptPrefix is the key prefix of database key to store block receipts
	// of specified block hash
	// /br/{hex encoded block hash}
	// e.g.
	// key: /bk/005973c44c4879b137c3723c96d2e341eeaf83fe58845b2975556c9f3bd640bb
	// value: receipts binary
	ReceiptPrefix = "/rc"

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

	// ContractAddressPrefix is the key prefix of contract address
	ContractAddressPrefix = "/cap"

	// AddressUtxoPrefix is the key prefix of database key to store address related utxo
	AddressUtxoPrefix = "/aut"

	// AddressTokenUtxoPrefix is the key prefix of database key to store address related token utxo
	AddressTokenUtxoPrefix = "/atut"
)

var blkBase = key.NewKey(BlockPrefix)
var txBase = key.NewKey(TxPrefix)
var splitTxHashBase = key.NewKey(SplitTxHashPrefix)
var blkHashBase = key.NewKey(BlockHashPrefix)
var txixBase = key.NewKey(TxIndexPrefix)
var utxoBase = key.NewKey(UtxoPrefix)
var receiptBase = key.NewKey(ReceiptPrefix)
var candidatesBase = key.NewKey(CandidatesPrefix)
var splitAddrBase = key.NewKey(SplitAddressPrefix)
var contractAddrBase = key.NewKey(ContractAddressPrefix)
var addrUtxoBase = key.NewKey(AddressUtxoPrefix)
var addrTokenUtxoBase = key.NewKey(AddressTokenUtxoPrefix)
var secBloomBitSetBase = key.NewKey(SecBloomBitSetPrefix)

// TailKey is the db key to store tail block content
var TailKey = []byte(Tail)

// GenesisKey is the db key to store genesis block content
var GenesisKey = []byte(Genesis)

// EternalKey is the db key to store eternal block content
var EternalKey = []byte(Eternal)

// PeriodKey is the db key to store current period contex content
var PeriodKey = []byte(Period)

// MissrateKey is the db key to store bookkeeper's blocks miss rate
var MissrateKey = []byte(MissCount)

// SectionKey is the db key to store section manager's section
var SectionKey = []byte(Section)

// BlockKey returns the db key to store block content of the hash
func BlockKey(h *crypto.HashType) []byte {
	return blkBase.ChildString(h.String()).Bytes()
}

// ReceiptKey returns the db key to store receipt content of the block hash
func ReceiptKey(h *crypto.HashType) []byte {
	return receiptBase.ChildString(h.String()).Bytes()
}

// TxKey returns the db key to store tx content of the hash
func TxKey(h *crypto.HashType) []byte {
	return txBase.ChildString(h.String()).Bytes()
}

// SplitTxHashKey returns the db key to store split tx mapping
func SplitTxHashKey(h *crypto.HashType) []byte {
	return splitTxHashBase.ChildString(h.String()).Bytes()
}

// BlockHashKey returns the db key to store block hash content of the height
func BlockHashKey(height uint32) []byte {
	return blkHashBase.ChildString(fmt.Sprintf("%x", height)).Bytes()
}

// TxIndexKey returns the db key to store tx index of the hash
func TxIndexKey(h *crypto.HashType) []byte {
	return txixBase.ChildString(h.String()).Bytes()
}

// CandidatesKey returns the db key to store candidates.
func CandidatesKey(h *crypto.HashType) []byte {
	return candidatesBase.ChildString(h.String()).Bytes()
}

// SplitAddrKey returns the db key to store split address
func SplitAddrKey(hash []byte) []byte {
	return splitAddrBase.ChildString(fmt.Sprintf("%x", hash)).Bytes()
}

// ContractAddrKey returns the db key to store contract address
func ContractAddrKey(hash []byte) []byte {
	return contractAddrBase.ChildString(fmt.Sprintf("%x", hash)).Bytes()
}

// AddrUtxoKey is the key to store an utxo which belongs to the input param address
func AddrUtxoKey(addrHash *types.AddressHash, op types.OutPoint) []byte {
	return addrUtxoBase.ChildString(hex.EncodeToString(addrHash[:])).
		ChildString(op.Hash.String()).ChildString(fmt.Sprintf("%x", op.Index)).Bytes()
}

// AddrAllUtxoKey is the key prefix to explore all utxos of an address
func AddrAllUtxoKey(addrHash *types.AddressHash) []byte {
	return addrUtxoBase.ChildString(hex.EncodeToString(addrHash[:])).Bytes()
}

// AddrTokenUtxoKey is the key to store an token utxo which belongs to the input param address
func AddrTokenUtxoKey(addrHash *types.AddressHash, tid types.TokenID, op types.OutPoint) []byte {
	return addrTokenUtxoBase.ChildString(hex.EncodeToString(addrHash[:])).
		ChildString(tid.Hash.String()).ChildString(fmt.Sprintf("%x", tid.Index)).
		ChildString(op.Hash.String()).ChildString(fmt.Sprintf("%x", op.Index)).Bytes()
}

// AddrAllTokenUtxoKey is the key prefix to explore all token utxos of an address
func AddrAllTokenUtxoKey(addrHash *types.AddressHash, tid types.TokenID) []byte {
	return addrTokenUtxoBase.ChildString(hex.EncodeToString(addrHash[:])).
		ChildString(tid.Hash.String()).ChildString(fmt.Sprintf("%x", tid.Index)).Bytes()
}

// SecBloomBitSetKey is the key to store bloom bit set
func SecBloomBitSetKey(section uint32, bit uint) []byte {
	return secBloomBitSetBase.ChildString(fmt.Sprintf("%x", section)).
		ChildString(fmt.Sprintf("%x", bit)).Bytes()
}
