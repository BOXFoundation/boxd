// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

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
