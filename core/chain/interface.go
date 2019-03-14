// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/storage"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Consensus define consensus interface
type Consensus interface {
	Run() error
	Stop()
	StoreCandidateContext(*types.Block, storage.Batch) error
	VerifySign(*types.Block) (bool, error)
	VerifyMinerEpoch(*types.Block) error
	StopMint()
	RecoverMint()
	BroadcastEternalMsgToMiners(*types.Block) error
	ValidateMiner() bool
	TryToUpdateEternalBlock(*types.Block)
	IsCandidateExist(types.AddressHash) bool
	VerifyCandidates(*types.Block) error
	UpdateCandidateContext(*types.Block) error
}

// SyncManager define sync manager interface
type SyncManager interface {
	StartSync()
	ActiveLightSync(peer.ID) error
	Run()
}
