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

const (
	bloomfilterBitsPerKey = 10
	lruCacheSize          = 3 << 30
)

// rocksdbStorage store the data in the concurrent map
type rocksdbStorage struct {
	db *gorocksdb.DB

	// synchronize access to the storage
	mutex sync.Mutex
	//
	isBatch         bool
	batchPutOptions map[string]*batchOption
	batchDelOptions map[string]*batchOption

	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions
}

type batchOption struct {
	key    []byte
	value  []byte
	delete bool
}

var _ Storage = (*rocksdbStorage)(nil)

// NewRocksDBStorage initializes the storage
func NewRocksDBStorage(cfg *Config) (Storage, error) {
	bbto := gorocksdb.NewDefaultBlockBasedTableOptions()

	// If your working dataset does not fit in memory, you'll want to add a bloom filter to your database.
	// NewBloomFilter creates a new initialized bloom filter for given bitsPerKey
	filter := gorocksdb.NewBloomFilter(bloomfilterBitsPerKey)
	bbto.SetFilterPolicy(filter)
	bbto.SetBlockCache(gorocksdb.NewLRUCache(lruCacheSize))

	// NewDefaultOptions builds a set of options
	options := gorocksdb.NewDefaultOptions()
	options.SetBlockBasedTableFactory(bbto)
	options.SetCreateIfMissing(true)

	db, err := gorocksdb.OpenDb(options, cfg.Path)
	if err != nil {
		return nil, err
	}

	dbstorage := &rocksdbStorage{
		db:              db,
		readOptions:     gorocksdb.NewDefaultReadOptions(),
		writeOptions:    gorocksdb.NewDefaultWriteOptions(),
		batchDelOptions: make(map[string]*batchOption),
		batchPutOptions: make(map[string]*batchOption),
		isBatch:         false,
	}
	return dbstorage, nil
}

// Put is used to put the key-value entry to the Storage
func (dbstorage *rocksdbStorage) Put(key []byte, value []byte) error {
	if dbstorage.isBatch {
		dbstorage.mutex.Lock()
		defer dbstorage.mutex.Unlock()

		dbstorage.batchPutOptions[util.Hex(key)] = &batchOption{
			key:    key,
			value:  value,
			delete: false,
		}
	}
	return dbstorage.db.Put(dbstorage.writeOptions, key, value)
}

// Get is used to return the data associated with the key from the Storage
func (dbstorage *rocksdbStorage) Get(key []byte) ([]byte, error) {
	val, err := dbstorage.db.GetBytes(dbstorage.readOptions, key)
	if val == nil {
		return nil, errors.New("Key not found")
	}
	if err != nil {
		return nil, err
	}
	return val, err
}

// Delete is used to delete the key associated with the value in the Storage
func (dbstorage *rocksdbStorage) Del(key []byte) error {
	if dbstorage.isBatch {
		dbstorage.mutex.Lock()
		defer dbstorage.mutex.Unlock()
		dbstorage.batchDelOptions[util.Hex(key)] = &batchOption{
			key:    key,
			delete: true,
		}
		return nil
	}
	return dbstorage.db.Delete(dbstorage.writeOptions, key)
}

// Has is used to check if the key exists
func (dbstorage *rocksdbStorage) Has(key []byte) (bool, error) {
	haskey, err := dbstorage.Get(key)
	if haskey == nil {
		return false, errors.New("key not found")
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

//Keys is used to return all keys in the Storage
func (dbstorage *rocksdbStorage) Keys() [][]byte {
	keys := [][]byte{}
	iter := dbstorage.db.NewIterator(dbstorage.readOptions)
	defer iter.Close()

	for iter.SeekToFirst(); iter.Valid(); iter.Next() {
		keys = append(keys, iter.Key().Data())
	}
	return keys
}

func (dbstorage *rocksdbStorage) Flush() error {
	dbstorage.mutex.Lock()
	defer dbstorage.mutex.Unlock()

	if !dbstorage.isBatch {
		return nil
	}

	writeBatch := gorocksdb.NewWriteBatch()
	defer writeBatch.Destroy()

	for _, putoption := range dbstorage.batchPutOptions {
		writeBatch.Put(putoption.key, putoption.value)
	}

	for _, deloption := range dbstorage.batchDelOptions {
		writeBatch.Delete(deloption.key)
	}

	dbstorage.batchPutOptions = make(map[string]*batchOption)
	dbstorage.batchDelOptions = make(map[string]*batchOption)

	return dbstorage.db.Write(dbstorage.writeOptions, writeBatch)
}

// Close db
func (dbstorage *rocksdbStorage) Close() error {
	dbstorage.db.Close()
	return nil
}
