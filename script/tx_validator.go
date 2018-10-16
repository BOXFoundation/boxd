// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"github.com/BOXFoundation/boxd/core/types"
)

// TxValidateItem holds a transaction along with which input to validate.
type TxValidateItem struct {
	TxInIndex int
	TxIn      *types.TxIn
	Tx        *types.Transaction
}
