// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"errors"
	"sync"

	"github.com/BOXFoundation/Quicksilver/util"
)

// memoryStorage the nodes in trie
type memoryStorage struct {
	sm *sync.Map
}

// NewMemoryStorage initialize the storage
func NewMemoryStorage() (*memoryStorage, error) {
	return &memoryStorage{
		sm: new(sync.Map),
	}, nil
}

// Get return value to the key in db
func (db *memoryStorage) Get(key []byte) ([]byte, error) {
	if entry, ok := db.sm.Load(util.Hex(key)); ok {
		return entry.([]byte), nil
	}
	return nil, errors.New("Data not found : No such key-value pairs")
}

// Put is used to put the key-value entry to db
func (db *memoryStorage) Put(key, value []byte) error {
	db.sm.Store(util.Hex(key), value)
	return nil
}

// Del is used to delete the entry associate with the key in the db
func (db *memoryStorage) Del(key []byte) error {
	db.sm.Delete(util.Hex(key))
	return nil
}

func (db *memoryStorage) Has(key []byte) (bool, error) {
	_, ok := db.sm.Load(util.Hex(key))
	return ok, nil
}

func (db *memoryStorage) Keys() [][]byte {
	keys := [][]byte{}
	db.sm.Range(func(key, value interface{}) bool {
		keys = append(keys, key.([]byte))
		return true
	})
	return keys
}

func (db *memoryStorage) Close() {}
