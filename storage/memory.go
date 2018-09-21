// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"errors"
	"sync"

	"github.com/BOXFoundation/Quicksilver/util"
)

// memoryStorage the data in concurrent map
type memoryStorage struct {
	sm *sync.Map
}

var _ Storage = (*memoryStorage)(nil)

// NewMemoryStorage initializes the storage
func NewMemoryStorage(_ *Config) (Storage, error) {
	return &memoryStorage{
		sm: new(sync.Map),
	}, nil
}

// Get returns value to the key in memoryStorage
func (ms *memoryStorage) Get(key []byte) ([]byte, error) {
	if entry, ok := ms.sm.Load(util.Hex(key)); ok {
		return entry.([]byte), nil
	}
	return nil, errors.New("Key not found")
}

// Put is used to put the key-value entry to memoryStorage
func (ms *memoryStorage) Put(key, value []byte) error {
	ms.sm.Store(util.Hex(key), value)
	return nil
}

// Del is used to delete the entry associated with the key in the memoryStorage
func (ms *memoryStorage) Del(key []byte) error {
	ms.sm.Delete(util.Hex(key))
	return nil
}

// Has is used to check if the key exists
func (ms *memoryStorage) Has(key []byte) (bool, error) {
	_, ok := ms.sm.Load(util.Hex(key))
	return ok, nil
}

func (ms *memoryStorage) Keys() [][]byte {
	keys := [][]byte{}
	ms.sm.Range(func(key, value interface{}) bool {
		keys = append(keys, key.([]byte))
		return true
	})
	return keys
}

func (ms *memoryStorage) Flush() error {
	return nil
}

func (ms *memoryStorage) Close() error {
	return nil
}
