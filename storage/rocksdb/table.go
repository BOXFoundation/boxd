// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"bytes"
	"sync"

	storage "github.com/BOXFoundation/Quicksilver/storage"
	"github.com/tecbot/gorocksdb"
)

type rtable struct {
	sm sync.Mutex

	rocksdb *gorocksdb.DB
	cf      *gorocksdb.ColumnFamilyHandle

	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions
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

// put the value to entry associate with the key
func (t *rtable) Put(key, value []byte) error {
	return t.rocksdb.PutCF(t.writeOptions, t.cf, key, value)
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

	// var buf = make([]byte, value.Size())
	// copy(buf, value.Data())
	// value.Free()
	// return buf, nil
	defer value.Free()
	return value.Data(), nil
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

	var keys [][]byte
	for it := iter; it.Valid(); it.Next() {
		key := it.Key()
		keys = append(keys, key.Data())
		key.Free()
	}
	return keys
}
