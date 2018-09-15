// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"errors"
	"sync"

	"github.com/BOXFoundation/Quicksilver/util"
)

// MemoryStorage the nodes in trie
type MemoryStorage struct {
	sm *sync.Map
}

// NewMemoryStorage initialize the storage
func NewMemoryStorage() (*MemoryStorage, error) {
	return &MemoryStorage{
		sm: new(sync.Map),
	}, nil
}

// NewMemoryStoragewithCap initialize the storage with the capacity
func NewMemoryStoragewithCap(size int) (*MemoryStorage, error) {
	return &MemoryStorage{ 
		sm: new(sync.Map, size),
	}, nil
}

// Get return value to the key in db
func (db *MemoryStorage) Get(key []byte) ([]byte, error) {
	if entry, ok := db.sm.Load(util.Hex(key)); ok {
		return entry.([]byte), nil
	}
	return nil, errors.New("Data not found : No such key-value pairs")
}

// Put is used to put the key-value entry to db
func (db *MemoryStorage) Put(key, value []byte) error {
	db.sm.Store(util.Hex(key), value)
	return nil
}

// Del is used to delete the entry associate with the key in the db
func (db *MemoryStorage) Del(key []byte) error {
	db.sm.Delete(util.Hex(key))
	return nil
}

func (db *MemoryStorage) Has(key []byte)([]byte, error){
	_,ok := db.sm.Load(util.Hex(key))
	return ok, nil
}

func (db *MemoryStorage) Keys()[][]byte{
	keys := [][]byte{}
	for key := range util.Hex(key){
		keys = append(keys, []byte(key))
	}
	return keys
}

func (db *MemoryStorage) Close(){}