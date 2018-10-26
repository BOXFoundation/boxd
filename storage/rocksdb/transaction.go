// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"context"
	"sync"

	storage "github.com/BOXFoundation/boxd/storage"
)

type dbtx struct {
	db        storage.Operations
	batch     storage.Batch
	closed    bool
	writeLock chan struct{}
	sm        sync.Mutex
}

// put the value to entry associate with the key
func (tr *dbtx) Put(key, value []byte) error {
	tr.sm.Lock()
	defer tr.sm.Unlock()

	if tr.closed {
		return storage.ErrTransactionClosed
	}

	tr.batch.Put(key, value)
	return nil
}

// delete the entry associate with the key in the Storage
func (tr *dbtx) Del(key []byte) error {
	tr.sm.Lock()
	defer tr.sm.Unlock()

	if tr.closed {
		return storage.ErrTransactionClosed
	}

	tr.batch.Del(key)
	return nil
}

// return value associate with the key in the Storage
func (tr *dbtx) Get(key []byte) ([]byte, error) {
	tr.sm.Lock()
	defer tr.sm.Unlock()

	if tr.closed {
		return nil, storage.ErrTransactionClosed
	}

	return tr.db.Get(key)
}

// check if the entry associate with key exists
func (tr *dbtx) Has(key []byte) (bool, error) {
	tr.sm.Lock()
	defer tr.sm.Unlock()

	if tr.closed {
		return false, storage.ErrTransactionClosed
	}

	return tr.db.Has(key)
}

// return a set of keys in the Storage
func (tr *dbtx) Keys() [][]byte {
	tr.sm.Lock()
	defer tr.sm.Unlock()

	if tr.closed {
		return [][]byte{}
	}

	return tr.db.Keys()
}

func (tr *dbtx) KeysWithPrefix(prefix []byte) [][]byte {
	tr.sm.Lock()
	defer tr.sm.Unlock()

	if tr.closed {
		return [][]byte{}
	}

	return tr.db.KeysWithPrefix(prefix)
}

// return a chan to iter all keys
func (tr *dbtx) IterKeys(ctx context.Context) <-chan []byte {
	tr.sm.Lock()
	defer tr.sm.Unlock()

	if tr.closed {
		return nil
	}

	return tr.db.IterKeys(ctx)
}

// return a set of keys with specified prefix in the Storage
func (tr *dbtx) IterKeysWithPrefix(ctx context.Context, prefix []byte) <-chan []byte {
	tr.sm.Lock()
	defer tr.sm.Unlock()

	if tr.closed {
		return nil
	}

	return tr.db.IterKeysWithPrefix(ctx, prefix)
}

// Commit to commit the transaction
func (tr *dbtx) Commit() error {
	tr.sm.Lock()
	defer tr.sm.Unlock()

	if tr.closed {
		return storage.ErrTransactionClosed
	}

	err := tr.batch.Write()
	tr.closed = true
	<-tr.writeLock

	return err
}

// Discard throws away changes recorded in a transaction without committing.
// them to the underlying Storage. Any calls made to Discard after Commit
// has been successfully called will have no effect on the transaction and
// state of the Storage, making it safe to defer.
func (tr *dbtx) Discard() {
	tr.sm.Lock()
	defer tr.sm.Unlock()

	if !tr.closed {
		tr.closed = true
		<-tr.writeLock
	}
}
