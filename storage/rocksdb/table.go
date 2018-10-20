// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"bytes"
	"sync"

	storage "github.com/BOXFoundation/boxd/storage"
	"github.com/tecbot/gorocksdb"
)

type rtable struct {
	sm sync.Mutex

	rocksdb      *gorocksdb.DB
	cf           *gorocksdb.ColumnFamilyHandle
	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions

	tr        *dbtx
	writeLock chan struct{}
}

// create a new write batch
func (t *rtable) NewBatch() storage.Batch {
	return &rbatch{
		rocksdb:      t.rocksdb,
		cf:           t.cf,
		wb:           gorocksdb.NewWriteBatch(),
		writeOptions: t.writeOptions,
	}
}

func (t *rtable) NewTransaction() (storage.Transaction, error) {
	t.sm.Lock()
	defer t.sm.Unlock()

	if t.tr != nil {
		t.tr.sm.Lock()
		defer t.tr.sm.Unlock()
		if !t.tr.closed {
			return nil, storage.ErrTransactionExists
		}
	}

	// lock all write operations
	t.writeLock <- struct{}{}
	t.tr = &dbtx{
		db:        t,
		batch:     t.NewBatch(),
		closed:    false,
		writeLock: t.writeLock,
	}

	return t.tr, nil
}

// put the value to entry associate with the key
func (t *rtable) Put(key, value []byte) error {
	t.writeLock <- struct{}{}
	err := t.rocksdb.PutCF(t.writeOptions, t.cf, key, value)
	<-t.writeLock
	return err
}

// delete the entry associate with the key in the Storage
func (t *rtable) Del(key []byte) error {
	return t.rocksdb.DeleteCF(t.writeOptions, t.cf, key)
}

// return value associate with the key in the Storage
func (t *rtable) Get(key []byte) ([]byte, error) {
	value, err := t.rocksdb.GetCF(t.readOptions, t.cf, key)
	if err != nil {
		return nil, err
	}

	return data(value), nil
}

// check if the entry associate with key exists
func (t *rtable) Has(key []byte) (bool, error) {
	var iter = t.rocksdb.NewIteratorCF(t.readOptions, t.cf)
	defer iter.Close()

	iter.Seek(key)
	if iter.Valid() {
		var k = iter.Key()
		defer k.Free()

		return bytes.Equal(key, k.Data()), nil
	}

	return false, nil
}

// return a set of keys in the Storage
func (t *rtable) Keys() [][]byte {
	var iter = t.rocksdb.NewIteratorCF(t.readOptions, t.cf)
	defer iter.Close()

	iter.SeekToFirst()
	var keys [][]byte
	for it := iter; it.Valid(); it.Next() {
		keys = append(keys, data(it.Key()))
	}
	return keys
}

func (t *rtable) KeysWithPrefix(prefix []byte) [][]byte {
	var iter = t.rocksdb.NewIteratorCF(t.readOptions, t.cf)
	defer iter.Close()

	iter.SeekToFirst()
	var keys [][]byte
	for it := iter; it.Valid(); it.Next() {
		key := it.Key()
		if bytes.HasPrefix(key.Data(), prefix) {
			keys = append(keys, data(key))
		}
	}
	return keys
}
