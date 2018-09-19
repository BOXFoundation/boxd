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

// rocksdbStorage store the node in the concurrent map
type rocksdbStorage struct {
	db *gorocksdb.DB

	mutex        sync.Mutex
	isBatch      bool
	batchOptions map[string]*batchOption

	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions
}

type batchOption struct {
	key     []byte
	value   []byte
	deleted bool
}

var _ Storage = (*rocksdbStorage)(nil)

// NewRocksDBStorage initializes the storage
func NewRocksDBStorage(path string) (Storage, error) {
	// NewDefaultBlockBasedTableOptions is used to create a default BlockBasedTableOptions object
	bbto := gorocksdb.NewDefaultBlockBasedTableOptions()

	// If your working dataset does not fit in memory, you'll want to add a bloom filter to your database.
	// NewBloomFilter creates a new initialized bloom filter for given bitsPerKey
	filter := gorocksdb.NewBloomFilter(bloomfilterBitsPerKey)

	// SetFilterPolicy sets the filter policy opts reduce disk reads
	bbto.SetFilterPolicy(filter)

	// SetBlockCache sets the control over blocks (user data is stored in a set of blocks, and a block is the unit of reading from disk)
	bbto.SetBlockCache(gorocksdb.NewLRUCache(lruCacheSize))

	// NewDefaultOptions builds a set of options
	options := gorocksdb.NewDefaultOptions()
	options.SetBlockBasedTableFactory(bbto)
	options.SetCreateIfMissing(true)

	db, err := gorocksdb.OpenDb(options, path)
	if err != nil {
		return nil, err
	}

	dbstorage := &rocksdbStorage{
		db: db,
		// NewDefaultReadOptions creates a default ReadOptions object
		readOptions: gorocksdb.NewDefaultReadOptions(),

		// NewDefaultWriteOptions creates a default WriteOptions object
		writeOptions: gorocksdb.NewDefaultWriteOptions(),
		batchOptions: make(map[string]*batchOption),
		isBatch:      false,
	}
	return dbstorage, nil
}

// Put is used to put the key-value entry to the Storage
func (dbstorage *rocksdbStorage) Put(key []byte, value []byte) error {
	if dbstorage.isBatch {
		dbstorage.mutex.Lock()
		defer dbstorage.mutex.Unlock()

		dbstorage.batchOptions[util.Hex(key)] = &batchOption{
			key:     key,
			value:   value,
			deleted: false,
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
		dbstorage.batchOptions[util.Hex(key)] = &batchOption{
			key:     key,
			deleted: true,
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
		key := make([]byte, 4)
		copy(key, iter.Key().Data())
		keys = append(keys, key)
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

	for _, option := range dbstorage.batchOptions {
		if option.deleted {
			writeBatch.Delete(option.key)
		} else {
			writeBatch.Put(option.key, option.value)
		}
	}

	dbstorage.batchOptions = make(map[string]*batchOption)

	return dbstorage.db.Write(dbstorage.writeOptions, writeBatch)
}

// Close db
func (dbstorage *rocksdbStorage) Close() error {
	dbstorage.db.Close()
	return nil
}
