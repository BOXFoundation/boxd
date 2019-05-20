// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/vm"
)

// ChainReader defines basic operations blockchain exposes
type ChainReader interface {
	// interface to read transactions
	LoadBlockInfoByTxHash(crypto.HashType) (*types.Block, *types.Transaction, error)
	ReadBlockFromDB(*crypto.HashType) (*types.Block, int, error)
	GetEvmByHeight(msg types.Message, height uint32) (*vm.EVM, func() error, error)
	NonceByHeight(address *types.AddressHash, height uint32) (uint64, error)

	//
	GetDataFromDB([]byte) ([]byte, error)

	//interface to reader block status
	GetBlockHeight() uint32
	GetBlockHash(uint32) (*crypto.HashType, error)
	EternalBlock() *types.Block
}
