// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
)

// ChainReader defines basic operations blockchain exposes
type ChainReader interface {
	// interface to read transactions
	LoadTxByHash(crypto.HashType) (*types.Transaction, error)
	LoadBlockInfoByTxHash(crypto.HashType) (*types.Block, uint32, error)
	ReadBlockFromDB(*crypto.HashType) (*types.Block, int, error)

	//
	GetDataFromDB([]byte) ([]byte, error)

	//interface to reader block status
	GetBlockHeight() uint32
	GetBlockHash(uint32) (*crypto.HashType, error)
	EternalBlock() *types.Block

	// address related search method
	GetTransactionsByAddr(types.Address) ([]*types.Transaction, error)

	ListTokenIssueTransactions() ([]*types.Transaction, []*types.BlockHeader, error)
	GetTokenTransactions(*script.TokenID) ([]*types.Transaction, error)

	IsCoinBase(*types.Transaction) bool
}
