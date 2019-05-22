// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package memdb

import (
	"context"
	"sync"

	"github.com/BOXFoundation/boxd/storage"
)

type mtx struct {
	txsm      sync.Mutex
	db        storage.Operations
	batch     *mbatch
	closed    bool
	writeLock chan struct{}
}

// put the value to entry associate with the key
func (tx *mtx) Put(key, value []byte) error {
	tx.txsm.Lock()
	defer tx.txsm.Unlock()

	if tx.closed {
		return storage.ErrTransactionClosed
	}

	tx.batch.Put(key, value)
	return nil
}

// delete the entry associate with the key in the Storage
func (tx *mtx) Del(key []byte) error {
	tx.txsm.Lock()
	defer tx.txsm.Unlock()

	if tx.closed {
		return storage.ErrTransactionClosed
	}

	tx.batch.Del(key)
	return nil
}

// return value associate with the key in the Storage
func (tx *mtx) Get(key []byte) ([]byte, error) {
	tx.txsm.Lock()
	defer tx.txsm.Unlock()

	if tx.closed {
		return nil, storage.ErrTransactionClosed
	}

	return tx.db.Get(key)
}

// return values associate with the keys in the Storage
func (tx *mtx) MultiGet(key ...[]byte) ([][]byte, error) {
	tx.txsm.Lock()
	defer tx.txsm.Unlock()

	if tx.closed {
		return nil, storage.ErrTransactionClosed
	}

	slices, err := tx.db.MultiGet(key...)
	if err != nil {
		return nil, err
	}
	return slices, nil
}

// check if the entry associate with key exists
func (tx *mtx) Has(key []byte) (bool, error) {
	tx.txsm.Lock()
	defer tx.txsm.Unlock()

	if tx.closed {
		return false, storage.ErrTransactionClosed
	}

	return tx.db.Has(key)
}

// return a set of keys in the Storage
func (tx *mtx) Keys() [][]byte {
	tx.txsm.Lock()
	defer tx.txsm.Unlock()

	if tx.closed {
		return nil
	}

	return tx.db.Keys()
}

func (tx *mtx) KeysWithPrefix(prefix []byte) [][]byte {
	tx.txsm.Lock()
	defer tx.txsm.Unlock()

	if tx.closed {
		return nil
	}

	return tx.db.KeysWithPrefix(prefix)
}

// return a chan to iter all keys
func (tx *mtx) IterKeys(ctx context.Context) <-chan []byte {
	tx.txsm.Lock()
	defer tx.txsm.Unlock()

	if tx.closed {
		return nil
	}

	return tx.db.IterKeys(ctx)
}

// return a set of keys with specified prefix in the Storage
func (tx *mtx) IterKeysWithPrefix(ctx context.Context, prefix []byte) <-chan []byte {
	tx.txsm.Lock()
	defer tx.txsm.Unlock()

	if tx.closed {
		return nil
	}

	return tx.db.IterKeysWithPrefix(ctx, prefix)
}

func (tx *mtx) Commit() error {
	tx.txsm.Lock()
	defer tx.txsm.Unlock()

	if tx.closed {
		return storage.ErrTransactionClosed
	}

	defer func() {
		tx.closed = true
	}()

	tx.writeLock <- struct{}{}
	err := tx.batch.write(false)
	<-tx.writeLock

	return err
}

func (tx *mtx) Discard() {
	tx.txsm.Lock()
	defer tx.txsm.Unlock()

	if tx.closed {
		return
	}

	tx.closed = true
}
