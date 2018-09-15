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

// RocksDBStorage store the node in trie
type RocksDBStorage struct {
	db           *gorocksdb.DB

	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions

	cache *gorocksdb.Cache
}

// NewRocksDBStorage initialize the storage
func NewRocksDBStorage(path string) (*RocksDBStorage, error) {
	bbto := gorocksdb.NewDefaultBlockBasedTableOptions()
	filter := gorocksdb.NewBloomFilter(10)
	bbto.SetFilterPolicy(filter)

	bbto.SetBlockCache(gorocksdb.NewLRUCache(3 << 30))
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
func （db *RocksDBStorage）Put(key, value []byte) error{
	return db.db.Put(db.writeOptions, key, value)
}

// Get is used to return the data associated with the key from the Storage
func (db *RocksDBStorage) Get(key []byte) ([]byte, error) {
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
func (db *RocksDBStorage) Del(key []byte) error {
	return db.db.Delete(db.WriteOptions, key)
}

// Close db
func (db *RocksDBStorage) Close() error {
	db.db.Close()
	return nil
}
