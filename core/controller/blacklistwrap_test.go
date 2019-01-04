// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package controller

import (
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/facebookgo/ensure"
)

func TestCalcHash(t *testing.T) {
	evidence := &Evidence{
		PubKey: []byte{},
		Type:   BlockEvidence,
		Block: &types.Block{
			Header:           &types.BlockHeader{},
			Txs:              []*types.Transaction{},
			IrreversibleInfo: &types.IrreversibleInfo{},
		},
		Err: "string",
		Ts:  time.Now().Unix(),
	}
	blm := &BlacklistMsg{
		evidences: []*Evidence{evidence},
	}
	hash, err := blm.calcHash()
	ensure.Nil(t, err)

	blm.hash = hash
	ok, err := blm.validateHash()

	ensure.DeepEqual(t, ok, true)
	ensure.Nil(t, err)
}
