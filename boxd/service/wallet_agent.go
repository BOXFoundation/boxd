// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import "github.com/BOXFoundation/boxd/core/types"

// WalletAgent defines functions an wallet service should provide
type WalletAgent interface {
	Balance(address types.Address) (uint64, error)
	Utxos(address types.Address) (types.UtxoMap, error)
}
