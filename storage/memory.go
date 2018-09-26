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

// enforce memoryStorage implements the Storage interface
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

func (ms *memoryStorage) Close() error {
	return nil
}

func (ms *memoryStorage) BeginTransaction() Batch {
	//lock map
	//new object
	return &memoryBatch{ms: ms}
}

type kv struct {
	key    []byte
	value  []byte
	delete bool
}

type memoryBatch struct {
	ms    *memoryStorage
	write []kv
	size  int
}

func (mb *memoryBatch) Put(key, value []byte) error {
	mb.write = append(mb.write, kv{util.CopyBytes(key), util.CopyBytes(value), false})
	mb.size += len(value)
	return nil
}

func (mb *memoryBatch) Del(key []byte) error {
	mb.write = append(mb.write, kv{util(key), nil, true})
	mb.size += 1
	return nil
}

func (mb *memoryBatch) Commit() error {
	for _, kv := range mb.write {
		if kv.delete {
			delete(mb.ms.sm, string(kv.key))
			continue
		}
		mb.ms.sm[string(kv.key)] = kv.key
	}
	return nil
}

func (mb *memoryBatch) Rollback() error {
	mb.write = mb.write[:0]
	mb.size = 0
}

type table struct {
	storage Storage
	prefix  string
}

func NewTable(prefix string) Storage {
	return &table{
		storage: storage,
		prefix:  prefix,
	}
}

func (t *table) Has(key []byte) (bool, error) {
	return t.storage.Has(append([]byte(t.prefix), key...))
}

func (t *table) Put(key []byte, value []byte) error {
	return t.storage.Put(append([]byte(t.prefix), key...), value)
}

func (t *table) Get(key []byte) ([]byte, error) {
	return t.storage.Get(append([]byte(t.prefix), key...))
}

func (t *table) Del(key []byte) error {
	return t.storage.Del(append([]byte(t.prefix), key...))
}

func (t *table) Keys() [][]byte {
	return nil
}

func (t *table) Close() error {
	return nil
}
