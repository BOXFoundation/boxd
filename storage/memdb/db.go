// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package memdb

import (
	"github.com/BOXFoundation/Quicksilver/log"
	storage "github.com/BOXFoundation/Quicksilver/storage"
)

var logger = log.NewLogger("memdb")

func init() {
	// register memdb impl
	storage.Register("memdb", NewMemoryDB)
}

// NewMemoryDB creates a memorydb instance
func NewMemoryDB(_ string, _ *storage.Options) (storage.Storage, error) {
	logger.Debug("Creating memdb")
	return &memorydb{
		db: make(map[string][]byte),
	}, nil
}
