// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blacklist

import (
	"os"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/facebookgo/ensure"
	"github.com/jbenet/goprocess"
)

func TestProcessEvidence(t *testing.T) {
	block := &types.Block{
		Txs:    make([]*types.Transaction, 0),
		Height: 0,
	}

	blackList := Default()

	blackList.Run(nil, eventbus.Default(), goprocess.WithSignals(os.Interrupt))

	for i := 0; i < 20; i++ {
		blackList.SceneCh <- &Evidence{PubKey: []byte{}, Block: block, Type: BlockEvidence, Err: "", Ts: time.Now().Unix()}
	}
}

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
