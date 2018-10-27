// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

// SyncManager define sync manager interface
type SyncManager interface {
	StartSync()
	Run()
}
