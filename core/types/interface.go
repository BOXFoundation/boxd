// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Consensus define consensus interface
type Consensus interface {
	Run() error
	Stop()
	StoreCandidateContext(*crypto.HashType, storage.Batch) error
	VerifySign(*Block) (bool, error)
	VerifyMinerEpoch(*Block) error
	StopMint()
	RecoverMint()
	BroadcastEternalMsgToMiners(*Block) error
	ValidateMiner() bool
}

// SyncManager define sync manager interface
type SyncManager interface {
	StartSync()
	ActiveLightSync(peer.ID) error
	Run()
}
