// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"os"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/jbenet/goprocess"
)

func TestProcessEvidence(t *testing.T) {
	block := &types.Block{
		Txs:    make([]*types.Transaction, 0),
		Height: 0,
	}

	blackList := Default()

	blackList.Run(nil, goprocess.WithSignals(os.Interrupt))

	for i := 0; i < 20; i++ {
		blackList.SceneCh <- &Evidence{PubKeyChecksum: 0, Scene: block, Err: nil, Ts: time.Now()}
	}

	time.Sleep(10 * time.Second)
}
