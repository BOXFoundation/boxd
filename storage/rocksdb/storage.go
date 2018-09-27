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

const number = 10
const cache = 3 << 30

type rocksdb struct {
	sm sync.Mutex

	rocksdb      *gorocksdb.DB
	dboptions    *gorocksdb.Options
	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions
	flushOptions *gorocksdb.FlushOptions

	smcfhandlers sync.Mutex
	cfs          map[string]*gorocksdb.ColumnFamilyHandle
}

// Create or Get the table associate with the name
func (db *rocksdb) Table(name string) (storage.Table, error) {
	db.smcfhandlers.Lock()
	defer db.smcfhandlers.Unlock()

	cf, ok := db.cfs[name]
	if !ok {
		cf, err := db.rocksdb.CreateColumnFamily(db.dboptions, name)
		if err != nil {
			return nil, err
		}
		db.cfs[name] = cf
	}

	return &rtable{
		rocksdb:      db.rocksdb,
		cf:           cf,
		readOptions:  db.readOptions,
		writeOptions: db.writeOptions,
	}, nil
}

// Create or Get the table associate with the name
func (db *rocksdb) DropTable(name string) error {
	db.smcfhandlers.Lock()
	defer db.smcfhandlers.Unlock()

	if cf, ok := db.cfs[name]; ok {
		err := db.rocksdb.DropColumnFamily(cf)
		delete(db.cfs, name)
		return err
	}
	return nil
}

// create a new write batch
func (db *rocksdb) NewBatch() storage.Batch {
	return &rbatch{
		rocksdb:      db.rocksdb,
		cf:           nil,
		wb:           gorocksdb.NewWriteBatch(),
		writeOptions: db.writeOptions,
	}
}

func (db *rocksdb) Close() error {
	db.sm.Lock()
	defer db.sm.Unlock()

	if err := db.rocksdb.Flush(db.flushOptions); err != nil {
		return err
	}

	db.rocksdb.Close()
	return nil
}

// put the value to entry associate with the key
func (db *rocksdb) Put(key, value []byte) error {
	return db.rocksdb.Put(db.writeOptions, key, value)
}

// delete the entry associate with the key in the Storage
func (db *rocksdb) Del(key []byte) error {
	return db.rocksdb.Delete(db.writeOptions, key)
}

// return value associate with the key in the Storage
func (db *rocksdb) Get(key []byte) ([]byte, error) {
	value, err := db.rocksdb.Get(db.readOptions, key)
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
func (db *rocksdb) Has(key []byte) (bool, error) {
	var iter = db.rocksdb.NewIterator(db.readOptions)
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
func (db *rocksdb) Keys() [][]byte {
	var iter = db.rocksdb.NewIterator(db.readOptions)
	defer iter.Close()

	var keys [][]byte
	for it := iter; it.Valid(); it.Next() {
		key := it.Key()
		keys = append(keys, key.Data())
		key.Free()
	}
	return keys
}
