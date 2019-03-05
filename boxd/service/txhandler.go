// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
)

// TxHandler defines basic operations txpool exposes
type TxHandler interface {
	ProcessTx(*types.Transaction, core.TransferMode) error
	GetTxByHash(hash *crypto.HashType) (*types.TxWrap, bool)
}
