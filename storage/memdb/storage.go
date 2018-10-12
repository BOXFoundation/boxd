// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package memdb

import (
	"bytes"
	"fmt"
	"sync"

	storage "github.com/BOXFoundation/boxd/storage"
)

type memorydb struct {
	sm sync.Mutex
	db map[string][]byte
}

var _ storage.Storage = (*memorydb)(nil)

// Create or Get the table associate with the name
func (db *memorydb) Table(name string) (storage.Table, error) {
	return &mtable{
		memorydb: db,
		prefix:   fmt.Sprintf("%s.", name),
	}, nil
}

// Create or Get the table associate with the name
func (db *memorydb) DropTable(name string) error {
	db.sm.Lock()
	defer db.sm.Unlock()

	var keys [][]byte
	for key := range db.db {
		keys = append(keys, []byte(key))
	}

	var prefix = fmt.Sprintf("%s.", name)
	var l = len(prefix)
	for _, key := range keys {
		if bytes.Equal(key[:l], []byte(prefix)) {
			delete(db.db, string(key))
		}
	}

	return nil
}

// create a new write batch
func (db *memorydb) NewBatch() storage.Batch {
	return &mbatch{
		memorydb: db,
	}
}

func (db *memorydb) Close() error {
	db.sm.Lock()
	defer db.sm.Unlock()

	db.db = make(map[string][]byte)

	return nil
}

// put the value to entry associate with the key
func (db *memorydb) Put(key, value []byte) error {
	db.sm.Lock()
	defer db.sm.Unlock()

	db.db[string(key)] = value
	return nil
}

// delete the entry associate with the key in the Storage
func (db *memorydb) Del(key []byte) error {
	db.sm.Lock()
	defer db.sm.Unlock()

	delete(db.db, string(key))

	return nil
}

// return value associate with the key in the Storage
func (db *memorydb) Get(key []byte) ([]byte, error) {
	db.sm.Lock()
	defer db.sm.Unlock()

	if value, ok := db.db[string(key)]; ok {
		return value, nil
	}

	return nil, storage.ErrKeyNotExists
}

// check if the entry associate with key exists
func (db *memorydb) Has(key []byte) (bool, error) {
	db.sm.Lock()
	defer db.sm.Unlock()

	_, ok := db.db[string(key)]
	return ok, nil
}

// return a set of keys in the Storage
func (db *memorydb) Keys() [][]byte {
	db.sm.Lock()
	defer db.sm.Unlock()

	var keys [][]byte
	for key := range db.db {
		keys = append(keys, []byte(key))
	}

	return keys
}
