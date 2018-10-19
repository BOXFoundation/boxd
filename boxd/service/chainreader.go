// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
)

// ChainReader defines basic operations blockchain exposes
type ChainReader interface {
	// interface to reader utxos
	ListAllUtxos() map[types.OutPoint]*types.UtxoWrap
	LoadUtxoByPubKeyScript([]byte) (map[types.OutPoint]*types.UtxoWrap, error)

	//interface to reader block status
	GetBlockHeight() int32
	GetBlockHash(int32) (*crypto.HashType, error)
	LoadBlockByHash(crypto.HashType) (*types.Block, error)
}
