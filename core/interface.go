// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

// Consensus define consensus interface
type Consensus interface {
	Run() error
	Stop()
}
