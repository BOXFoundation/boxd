// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package memdb

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	storage "github.com/BOXFoundation/boxd/storage"
)

type memorydb struct {
	sm        sync.RWMutex
	writeLock chan struct{}
	db        map[string][]byte
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

// create a new write batch
func (db *memorydb) NewBatch() storage.Batch {
	return &mbatch{
		memorydb: db,
	}
}

func (db *memorydb) NewTransaction() (storage.Transaction, error) {
	timer := time.NewTimer(time.Millisecond * 100)
	select {
	case <-timer.C:
		return nil, storage.ErrTransactionExists
	case db.writeLock <- struct{}{}:
	}

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
	db.writeLock <- struct{}{}
	defer func() {
		<-db.writeLock
	}()

	db.sm.Lock()
	defer db.sm.Unlock()

	db.db[string(key)] = value
	return nil
}

// delete the entry associate with the key in the Storage
func (db *memorydb) Del(key []byte) error {
	db.writeLock <- struct{}{}
	defer func() {
		<-db.writeLock
	}()

	db.sm.Lock()
	defer db.sm.Unlock()

	delete(db.db, string(key))

	return nil
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
			case out <- k:
			case <-ctx.Done():
				return
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
