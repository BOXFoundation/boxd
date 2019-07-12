// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"github.com/BOXFoundation/boxd/core/types"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Consensus define consensus interface
type Consensus interface {
	Run() error
	Stop()
	StopMint()
	RecoverMint()

	Verify(*types.Block) error
	Process(*types.Block, interface{}) error
	Finalize(*types.Block) error

	VerifyTx(*types.Transaction) error
}

// SyncManager define sync manager interface
type SyncManager interface {
	StartSync()
	ActiveLightSync(peer.ID) error
	Run()
}
