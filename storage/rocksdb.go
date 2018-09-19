// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"errors"

	"github.com/tecbot/gorocksdb"
)

const number = 10
const cache = 3 << 30

// rocksdbStorage store the node in trie
type rocksdbStorage struct {
	db *gorocksdb.DB

	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions

	cache *gorocksdb.Cache
}

var _ Storage = (*rocksdbStorage)(nil)

// NewRocksDBStorage initialize the storage
func NewRocksDBStorage(path string) (Storage, error) {
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

	dbstorage := &rocksdbStorage{
		db:           db,
		readOptions:  gorocksdb.NewDefaultReadOptions(),
		writeOptions: gorocksdb.NewDefaultWriteOptions(),
	}
	return dbstorage, nil
}

// Put is used to put the key-value entry to the Storage
func (dbstorage *rocksdbStorage) Put(key []byte, value []byte) error {
	return dbstorage.db.Put(dbstorage.writeOptions, key, value)
}

// Get is used to return the data associated with the key from the Storage
func (dbstorage *rocksdbStorage) Get(key []byte) ([]byte, error) {
	val, err := dbstorage.db.GetBytes(dbstorage.readOptions, key)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, errors.New("Data not found : No such key-value pairs")
	}
	return val, err
}

// Delete is used to delete the key associate with the value in the Storage
func (dbstorage *rocksdbStorage) Del(key []byte) error {
	return dbstorage.db.Delete(dbstorage.writeOptions, key)
}

// Has is used to check if the key existed
func (dbstorage *rocksdbStorage) Has(key []byte) (bool, error) {
	//return dbstorage.db.Has(key, nil)
	return false, nil
}

func (dbstorage *rocksdbStorage) Keys() [][]byte {
	keys := [][]byte{}
	return keys
}

// Close db
func (dbstorage *rocksdbStorage) Close() {
	dbstorage.db.Close()
}
