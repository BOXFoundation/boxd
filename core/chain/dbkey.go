// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"bytes"
	"fmt"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage/key"
	"github.com/BOXFoundation/boxd/util"
)

const (
	readable = true // readable db key or compact key
)

const (
	// BlockTableName is the table name of db to store block chain data
	BlockTableName = "core"

	// Tail is the db key name of tail block
	Tail = "/tail"

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
)

var blkBase = key.NewKey(BlockPrefix)
var blkHashBase = key.NewKey(BlockHashPrefix)
var txixBase = key.NewKey(TxIndexPrefix)
var utxoBase = key.NewKey(UtxoPrefix)

var genesisBlockKey = BlockKey(genesisBlock.BlockHash())

// TailKey is the db key to stoare tail block content
var TailKey = []byte(Tail)

// BlockKey returns the db key to stoare block content of the hash
func BlockKey(h *crypto.HashType) []byte {
	if readable {
		return blkBase.ChildString(h.String()).Bytes()
	}
	return h[:]
}

// BlockHashKey returns the db key to stoare block hash content of the height
func BlockHashKey(height uint32) []byte {
	if readable {
		return blkHashBase.ChildString(fmt.Sprintf("%x", height)).Bytes()
	}
	return util.FromUint32(height)
}

// TxIndexKey returns the db key to stoare tx index of the hash
func TxIndexKey(h *crypto.HashType) []byte {
	if readable {
		return txixBase.ChildString(h.String()).Bytes()
	}
	return h[:]
}

// UtxoKey returns the db key to stoare utxo content of the Outpoint
func UtxoKey(op *types.OutPoint) []byte {
	if readable {
		return utxoBase.ChildString(op.Hash.String()).ChildString(fmt.Sprintf("%x", op.Index)).Bytes()
	}
	buf := make([]byte, crypto.HashSize+4)
	copy(buf, op.Hash[:])
	w := bytes.NewBuffer(buf[crypto.HashSize:])
	util.WriteUint32(w, op.Index)
	return buf
}
