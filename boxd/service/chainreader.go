// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"github.com/BOXFoundation/boxd/core/types"
)

// ChainReader defines basic operations blockchain exposes
type ChainReader interface {
	ListAllUtxos() map[types.OutPoint]*types.UtxoWrap
	LoadUtxoByPubKeyScript(pubkey []byte) (map[types.OutPoint]*types.UtxoWrap, error)
}
