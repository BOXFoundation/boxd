// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package memdb

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	storage "github.com/BOXFoundation/boxd/storage"
)

type memorydb struct {
	sm          sync.RWMutex
	writeLock   chan struct{}
	db          map[string][]byte
	enableBatch bool
	batch       storage.Batch
}

var _ storage.Storage = (*memorydb)(nil)

// Create or Get the table associated with the name
func (db *memorydb) Table(name string) (storage.Table, error) {
	return &mtable{
		memorydb: db,
		prefix:   fmt.Sprintf("%s.", name),
	}, nil
}

// Drop the table associated with the name
func (db *memorydb) DropTable(name string) error {
	db.writeLock <- struct{}{}
	defer func() {
		<-db.writeLock
	}()

	db.sm.Lock()
	defer db.sm.Unlock()

	var keys [][]byte
	for key := range db.db {
		keys = append(keys, []byte(key))
	}

	var prefix = fmt.Sprintf("%s.", name)
	for _, key := range keys {
		if bytes.HasPrefix(key, []byte(prefix)) {
			delete(db.db, string(key))
		}
	}

	return nil
}

func (db *memorydb) EnableBatch() {
	db.enableBatch = true
	db.batch = db.NewBatch()
}

// DisableBatch disable batch write.
func (db *memorydb) DisableBatch() {
	db.sm.Lock()
	defer db.sm.Unlock()
	db.enableBatch = false
	db.batch = nil
}

// IsInBatch indicates whether db is in batch
func (db *memorydb) IsInBatch() bool {
	db.sm.Lock()
	defer db.sm.Unlock()
	return db.enableBatch
}

// create a new write batch
func (db *memorydb) NewBatch() storage.Batch {
	return &mbatch{
		memorydb: db,
	}
}

func (db *memorydb) NewTransaction() (storage.Transaction, error) {
	return &mtx{
		db:        db,
		closed:    false,
		batch:     &mbatch{memorydb: db},
		writeLock: db.writeLock,
	}, nil
}

func (db *memorydb) Close() error {
	db.writeLock <- struct{}{}
	defer func() {
		<-db.writeLock
	}()

	db.sm.Lock()
	defer db.sm.Unlock()

	db.db = make(map[string][]byte)

	return nil
}

// put the value to entry associate with the key
func (db *memorydb) Put(key, value []byte) error {

	if db.enableBatch {
		db.batch.Put(key, value)
	} else {
		db.writeLock <- struct{}{}
		defer func() {
			<-db.writeLock
		}()

		db.sm.Lock()
		defer db.sm.Unlock()

		db.db[string(key)] = value
	}

	return nil
}

// delete the entry associate with the key in the Storage
func (db *memorydb) Del(key []byte) error {

	if db.enableBatch {
		db.batch.Del(key)
	} else {
		db.writeLock <- struct{}{}
		defer func() {
			<-db.writeLock
		}()

		db.sm.Lock()
		defer db.sm.Unlock()

		delete(db.db, string(key))
	}

	return nil
}

// Flush atomic writes all enqueued put/delete
func (db *memorydb) Flush() error {

	var err error
	if db.enableBatch {
		err = db.batch.Write()
	} else {
		err = storage.ErrOnlySupportBatchOpt
	}
	return err
}

// return value associate with the key in the Storage
func (db *memorydb) Get(key []byte) ([]byte, error) {
	db.sm.RLock()
	defer db.sm.RUnlock()

	if value, ok := db.db[string(key)]; ok {
		return value, nil
	}

	return nil, nil
}

// return values associate with the keys in the Storage
func (db *memorydb) MultiGet(key ...[]byte) ([][]byte, error) {
	db.sm.RLock()
	defer db.sm.RUnlock()

	values := [][]byte{}
	for _, k := range key {
		if value, ok := db.db[string(k)]; ok {
			values = append(values, value)
		} else {
			return nil, nil
		}
	}
	return values, nil
}

// check if the entry associate with key exists
func (db *memorydb) Has(key []byte) (bool, error) {
	db.sm.RLock()
	defer db.sm.RUnlock()

	_, ok := db.db[string(key)]
	return ok, nil
}

// return a set of keys in the Storage
func (db *memorydb) Keys() [][]byte {
	db.sm.RLock()
	defer db.sm.RUnlock()

	var keys [][]byte
	for key := range db.db {
		keys = append(keys, []byte(key))
	}

	return keys
}

func (db *memorydb) KeysWithPrefix(prefix []byte) [][]byte {
	db.sm.RLock()
	defer db.sm.RUnlock()

	var keys [][]byte
	for key := range db.db {
		k := []byte(key)
		if bytes.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}

	return keys
}

// return a chan to iter all keys
func (db *memorydb) IterKeys(ctx context.Context) <-chan []byte {
	keys := db.Keys()

	out := make(chan []byte)
	go func() {
		defer close(out)

		for _, k := range keys {
			select {
			case <-ctx.Done():
				return
			case out <- k:
			}
		}
	}()
	return out
}

// return a set of keys with specified prefix in the Storage
func (db *memorydb) IterKeysWithPrefix(ctx context.Context, prefix []byte) <-chan []byte {
	keys := db.KeysWithPrefix(prefix)

	out := make(chan []byte)
	go func() {
		defer close(out)

		for _, k := range keys {
			select {
			case out <- k:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
