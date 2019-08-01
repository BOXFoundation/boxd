// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package consensus

import "github.com/BOXFoundation/boxd/core/types"

// Consensus define consensus interface
type Consensus interface {
	Run() error
	Stop()

	StopMint()
	RecoverMint()

	Verify(*types.Block) error
	Finalize(*types.Block) error
}
