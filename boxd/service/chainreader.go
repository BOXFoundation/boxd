// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/vm"
)

// ChainReader defines basic operations blockchain exposes
type ChainReader interface {
	// interface to read transactions
	LoadBlockInfoByTxHash(crypto.HashType) (*types.Block, *types.Transaction, error)
	ReadBlockFromDB(*crypto.HashType) (*types.Block, int, error)
	NewEvmContextForLocalCallByHeight(msg types.Message, height uint32) (*vm.EVM, func() error, error)
	GetStateDbByHeight(height uint32) (*state.StateDB, error)
	GetLogs(from, to uint32, topicslist [][][]byte) ([]*types.Log, error)
	FilterLogs(logs []*types.Log, topicslist [][][]byte) ([]*types.Log, error)

	//
	GetDataFromDB([]byte) ([]byte, error)
	GetTxReceipt(*crypto.HashType) (*types.Receipt, error)

	//interface to reader block status
	GetBlockHash(uint32) (*crypto.HashType, error)
	EternalBlock() *types.Block
	TailBlock() *types.Block
	TailState() *state.StateDB
}
