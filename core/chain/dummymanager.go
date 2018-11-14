// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"os"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/jbenet/goprocess"
)

// DummySyncManager is only used to test
type DummySyncManager struct{}

// NewDummySyncManager returns a new DummySyncManager
func NewDummySyncManager() *DummySyncManager {
	return &DummySyncManager{}
}

// StartSync starts sync
func (dm *DummySyncManager) StartSync() {}

// Run starts run
func (dm *DummySyncManager) Run() {}

// NewTestBlockChain generate a chain for testing
func NewTestBlockChain() *BlockChain {
	dbCfg := &storage.Config{
		Name: "memdb",
		Path: "~/tmp",
	}

	proc := goprocess.WithSignals(os.Interrupt)
	db, _ := storage.NewDatabase(proc, dbCfg)
	blockChain, _ := NewBlockChain(proc, p2p.NewDummyPeer(), db, eventbus.Default())
	// set sync manager
	blockChain.Setup(new(DummyDpos), NewDummySyncManager())
	return blockChain
}

// DummyDpos dummy dpos
type DummyDpos struct{}

// Run dummy dpos
func (dpos *DummyDpos) Run() error { return nil }

// Stop dummy dpos
func (dpos *DummyDpos) Stop() {}

// StoreCandidateContext store candidate context
func (dpos *DummyDpos) StoreCandidateContext(*crypto.HashType) error { return nil }

// VerifySign verify sign
func (dpos *DummyDpos) VerifySign(*types.Block) (bool, error) { return true, nil }

// VerifyMinerEpoch verify miner epoch
func (dpos *DummyDpos) VerifyMinerEpoch(*types.Block) error { return nil }

// RecoverMint revover mint
func (dpos *DummyDpos) RecoverMint() {}

// StopMint stop mint
func (dpos *DummyDpos) StopMint() {}

// BroadcastEternalMsgToMiners broadcast etrnalmsg to miners
func (dpos *DummyDpos) BroadcastEternalMsgToMiners(block *types.Block) error { return nil }

// ValidateMiner validate miner
func (dpos *DummyDpos) ValidateMiner() bool { return true }
