// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import "github.com/BOXFoundation/boxd/core/types"

// TxHandler defines basic operations txpool exposes
type TxHandler interface {
	ProcessTx(tx *types.Transaction, broadcast bool) error
	GetOutPointLockedByPool() []types.OutPoint
	// GetTransactionsInPool gets all transactions in memory pool
	GetTransactionsInPool() []*types.Transaction
}
