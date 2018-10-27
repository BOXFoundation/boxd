// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"github.com/facebookgo/ensure"
	"github.com/jbenet/goprocess"
	"os"
	"testing"
)

func genScoreMgr() *ScoreManager {
	proc := goprocess.WithSignals(os.Interrupt)

	return NewScoreManager(proc, nil, nil)
}

func TestNewScoreManager(t *testing.T) {
	ensure.NotNil(t, genScoreMgr)
}

func TestGc(t *testing.T) {
	scoreMgr := genScoreMgr()
	scoreMgr.peer.Gc()
}
