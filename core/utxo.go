// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/BOXFoundation/Quicksilver/core/types"
	"github.com/BOXFoundation/Quicksilver/storage"
)

// UtxoUnspentCache define unspent transaction  pool
type UtxoUnspentCache struct {
	outPointMap map[types.OutPoint]*UtxoWrap
	tailHash    *types.Block
}

// UtxoWrap utxo wrap
type UtxoWrap struct {
	Value        int64
	ScriptPubKey []byte
	blockHeight  int
	IsPacked     bool
	IsCoinbase   bool
}

// NewUtxoUnspentCache returns a new empty unspent transaction pool
func NewUtxoUnspentCache() *UtxoUnspentCache {
	return &UtxoUnspentCache{
		outPointMap: make(map[types.OutPoint]*UtxoWrap),
	}
}

//LoadUtxoFromDB load related unspent utxo
func (uup *UtxoUnspentCache) LoadUtxoFromDB(db storage.Storage, outpoints map[types.OutPoint]struct{}) error {
	return nil
}

// FindByOutPoint returns utxo wrap from cache by outpoint.
func (uup *UtxoUnspentCache) FindByOutPoint(outpoint types.OutPoint) *UtxoWrap {
	return uup.outPointMap[outpoint]
}

// RemoveByOutPoint delete utxo wrap from cache by outpoint.
func (uup *UtxoUnspentCache) RemoveByOutPoint(outpoint types.OutPoint) {
	delete(uup.outPointMap, outpoint)
}
