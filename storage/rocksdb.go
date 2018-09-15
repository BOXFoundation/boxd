// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"errors"
	"sync"

	"github.com/BOXFoundation/Quicksilver/util"
	"github.com/tecbot/gorocksdb"
)

const number = 10
const cache = 3 << 30

// rocksdbStorage store the node in trie
type rocksdbStorage struct {
	db           *gorocksdb.DB

	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions

	cache *gorocksdb.Cache
}

// NewRocksDBStorage initialize the storage
func NewRocksDBStorage(path string) (*rocksdbStorage, error) {
	bbto := gorocksdb.NewDefaultBlockBasedTableOptions()
	filter := gorocksdb.NewBloomFilter(number)
	bbto.SetFilterPolicy(filter)

	bbto.SetBlockCache(gorocksdb.NewLRUCache(cache))
	options := gorocksdb.NewDefaultOptions()
	options.SetBlockBasedTableFactory(bbto)
	options.SetCreateIfMissing(true)

	db, err := gorocksdb.OpenDb(options, path)
	if err != nil {
		return nil, err
	}

	db := &RocksDBStorage{
	db:           db,
	readOptions:  gorocksdb.NewDefaultReadOptions(),
	writeOptions: gorocksdb.NewDefaultWriteOptions(),
	}
	return dbstorage, nil
}

// Put is used to put the key-value entry to the Storage
func （db *rocksdbStorage）Put(key, value []byte) error{
	return db.db.Put(db.writeOptions, key, value)
}

// Get is used to return the data associated with the key from the Storage
func (db *rocksdbStorage) Get(key []byte) ([]byte, error) {
	val, err := db.db.Get(db.readOptions, key)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, errors.New("Data not found : No such key-value pairs")
	}
	return val, err
}

// Delete is used to delete the key associate with the value in the Storage
func (db *rocksdbStorage) Del(key []byte) error {
	return db.db.Delete(db.WriteOptions, key)
}

// Close db
func (db *rocksdbStorage) Close() error {
	db.db.Close()
	return nil
}
