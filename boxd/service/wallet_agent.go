// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/rpc/pb"
)

// WalletAgent defines functions an wallet service should provide
type WalletAgent interface {
	Balance(addr string, tid *txlogic.TokenID) (uint64, error)
	Utxos(addr string, tid *txlogic.TokenID, amount uint64) ([]*rpcpb.Utxo, error)
}
