// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import "github.com/BOXFoundation/boxd/core/types"

// TxGenerateHelper is an interface which provides help to generate transaction
type TxGenerateHelper interface {
	GetFee() (uint64, error)
	Fund(fromAddr types.Address, amountRequired uint64) (map[types.OutPoint]*types.UtxoWrap, error)
}
