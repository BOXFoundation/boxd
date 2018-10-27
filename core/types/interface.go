// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"github.com/BOXFoundation/boxd/crypto"
)

// Consensus define consensus interface
type Consensus interface {
	Run() error
	Stop()
	StoreCandidateContext(crypto.HashType) error
	VerifySign(*Block) (bool, error)
	StopMint()
	RecoverMint()
	UpdateEternalBlock(string, []byte)
	BroadcastEternalMsgToMiners(*Block) error
}

// SyncManager define sync manager interface
type SyncManager interface {
	StartSync()
	Run()
}
