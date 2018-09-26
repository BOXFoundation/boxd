// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"errors"

	"github.com/tecbot/gorocksdb"
)

const (
	bloomfilterBitsPerKey = 10
	lruCacheSize          = 3 << 30
)

// rocksdbStorage store the data in the concurrent map
type rocksdbStorage struct {
	db *gorocksdb.DB

	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions
	tnxdb        *gorocksdb.TransactionDB
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
		db:           db,
		readOptions:  gorocksdb.NewDefaultReadOptions(),
		writeOptions: gorocksdb.NewDefaultWriteOptions(),
		tnxdbopts:    gorocksdb.TransactionDBOptions(),
		tnxdb:        gorocksdb.TransactionDB("", options, tnxdbopts),
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

// Close db
func (dbstorage *rocksdbStorage) Close() error {
	dbstorage.db.Flush()
	dbstorage.db.Close()
	return nil
}

type rocksBatch struct {
	db *rocksdbStorage
	tx *gorocksdb.Transaction
}

func (dbstorage *rocksdbStorage) NewBatch() Batch {
	return &rocksBatch{tx: tnx_db.TransactionBegin()}
}

func (dbbatch *rocksBatch) Put(key, value []byte) error {
	tx.Put(key, value)
	return nil
}

func (dbbatch *rocksBatch) Del(key []byte) error {
	tx.Delete(key)
	return nil
}

func (dbbatch *rocksBatch) Commit() error {
	tx.Commit()
	return nil
}

func (dbbatch *rocksBatch) Rollback() error {
	tx.Rollback()
	return nil
}

type rocksTable struct {
	rocksdb rocksdbStorage
	prefix  string
}

func NewRocksTable(prefix string) rocksdbStorage {
	return &rocksTable{
		rocksdb: rocksdb
		prefix: prefix
	}
}

func (t *rocksTable) Has(key []byte) (bool, error) {
	return nil
}

func (t *rocksTable) Put(key []byte, value []byte) error {
	return nil
}

func (t *rocksTable) Get(key []byte) ([]byte, error) {
	return nil
}

func (t *rocksTable) Del(key []byte) error {
	return nil
}

func (t *rocksTable) Keys() [][]byte {
	return nil
}

func (t *rocksTable) Close() error {
	return nil
}