// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"bytes"
	"context"
	"sync"
	"time"

	storage "github.com/BOXFoundation/boxd/storage"
	"github.com/tecbot/gorocksdb"
)

type rocksdb struct {
	sm sync.Mutex

	rocksdb      *gorocksdb.DB
	dboptions    *gorocksdb.Options
	readOptions  *gorocksdb.ReadOptions
	writeOptions *gorocksdb.WriteOptions
	flushOptions *gorocksdb.FlushOptions

	tr        *dbtx
	writeLock chan struct{}

	enableBatch bool
	batch       storage.Batch

	smcfhandlers sync.Mutex
	cfs          map[string]*gorocksdb.ColumnFamilyHandle
	tables       map[string]*rtable
}

func (db *rocksdb) EnableBatch() {
	db.enableBatch = true
	db.batch = db.NewBatch()
}

// DisableBatch disable batch write.
func (db *rocksdb) DisableBatch() {
	db.sm.Lock()
	defer db.sm.Unlock()
	db.enableBatch = false
	db.batch = nil
}

// Create or Get the table associate with the name
func (db *rocksdb) Table(name string) (storage.Table, error) {
	db.smcfhandlers.Lock()
	defer db.smcfhandlers.Unlock()

	cf, ok := db.cfs[name]
	if !ok {
		var err error
		cf, err = db.rocksdb.CreateColumnFamily(db.dboptions, name)
		if err != nil {
			return nil, err
		}
		db.cfs[name] = cf
	}

	t, ok := db.tables[name]
	if !ok {
		t = &rtable{
			rocksdb:      db.rocksdb,
			cf:           cf,
			readOptions:  db.readOptions,
			writeOptions: db.writeOptions,
			writeLock:    make(chan struct{}, 1),
		}
		db.tables[name] = t
	}

	return t, nil
}

// Create or Get the table associate with the name
func (db *rocksdb) DropTable(name string) error {
	db.smcfhandlers.Lock()
	defer db.smcfhandlers.Unlock()

	if cf, ok := db.cfs[name]; ok {
		err := db.rocksdb.DropColumnFamily(cf)
		delete(db.cfs, name)
		delete(db.tables, name)
		return err
	}
	return nil
}

////////////////////////////////////////////////////////////////

// create a new write batch
func (db *rocksdb) NewBatch() storage.Batch {
	return &rbatch{
		rocksdb:      db.rocksdb,
		cf:           nil,
		wb:           gorocksdb.NewWriteBatch(),
		writeOptions: db.writeOptions,
	}
}

func (db *rocksdb) NewTransaction() (storage.Transaction, error) {
	db.sm.Lock()
	defer db.sm.Unlock()

	// if db.tr != nil {
	// 	db.tr.sm.Lock()
	// 	defer db.tr.sm.Unlock()
	// 	if !db.tr.closed {
	// 		return nil, storage.ErrTransactionExists
	// 	}
	// }

	// lock all write operations
	db.writeLock <- struct{}{}
	db.tr = &dbtx{
		db:        db,
		batch:     db.NewBatch(),
		closed:    false,
		writeLock: db.writeLock,
	}

	return db.tr, nil
}

func waitLock(c chan<- struct{}) {
	timer := time.NewTimer(time.Second * 3)
	defer timer.Stop()
	select {
	case c <- struct{}{}:
	case <-timer.C:
		logger.Warn("Locking db write timeout...")
	}
}

// Close closes the database
func (db *rocksdb) Close() error {
	db.sm.Lock()
	defer db.sm.Unlock()

	if err := db.rocksdb.Flush(db.flushOptions); err != nil {
		return err
	}

	waitLock(db.writeLock)
	for _, t := range db.tables {
		waitLock(t.writeLock)
	}

	for _, t := range db.tables {
		t.Close()
	}
	db.rocksdb.Close()

	db.writeOptions.Destroy()
	db.readOptions.Destroy()
	db.flushOptions.Destroy()

	close(db.writeLock)
	db.cfs = nil
	db.tables = nil

	return nil
}

////////////////////////////////////////////////////////////////

// put the value to entry associate with the key
func (db *rocksdb) Put(key, value []byte) error {
	var err error
	if db.enableBatch {
		db.batch.Put(key, value)
	} else {
		db.writeLock <- struct{}{}
		err = db.rocksdb.Put(db.writeOptions, key, value)
		<-db.writeLock
	}
	return err
}

// delete the entry associate with the key in the Storage
func (db *rocksdb) Del(key []byte) error {
	var err error
	if db.enableBatch {
		db.batch.Del(key)
	} else {
		db.writeLock <- struct{}{}
		err = db.rocksdb.Delete(db.writeOptions, key)
		<-db.writeLock
	}

	return err
}

// Flush atomic writes all enqueued put/delete
func (db *rocksdb) Flush() error {
	var err error
	db.writeLock <- struct{}{}
	if db.enableBatch {
		err = db.batch.Write()
	} else {
		err = storage.ErrOnlySupportBatchOpt
	}
	<-db.writeLock
	return err
}

// return value associate with the key in the Storage
func (db *rocksdb) Get(key []byte) ([]byte, error) {
	value, err := db.rocksdb.Get(db.readOptions, key)
	if err != nil {
		return nil, err
	}

	return data(value), nil
}

// return values associate with the keys in the Storage
func (db *rocksdb) MultiGet(key ...[]byte) ([][]byte, error) {
	slices, err := db.rocksdb.MultiGet(db.readOptions, key...)
	if err != nil {
		return nil, err
	}
	values := make([][]byte, 0, len(slices))
	for _, slice := range slices {
		values = append(values, data(slice))
	}
	return values, nil
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

	iter.SeekToFirst()
	var keys [][]byte
	for it := iter; it.Valid(); it.Next() {
		buf := data(it.Key())
		keys = append(keys, buf)
	}
	return keys
}

// return a set of keys with specified prefix in the Storage
func (db *rocksdb) KeysWithPrefix(prefix []byte) [][]byte {
	var iter = db.rocksdb.NewIterator(db.readOptions)
	defer iter.Close()

	iter.Seek(prefix)
	var keys [][]byte
	for it := iter; it.Valid(); it.Next() {
		if bytes.HasPrefix(it.Key().Data(), prefix) {
			buf := data(it.Key())
			keys = append(keys, buf)
		} else {
			break
		}
	}
	return keys
}

// return a chan to iter all keys
func (db *rocksdb) IterKeys(ctx context.Context) <-chan []byte {
	var iter = db.rocksdb.NewIterator(db.readOptions)

	out := make(chan []byte)
	go func() {
		defer close(out)
		defer iter.Close()

		iter.SeekToFirst()
		for {
			if !iter.Valid() {
				return
			}
			select {
			case <-ctx.Done():
				return
			case out <- data(iter.Key()):
				iter.Next()
			}
		}
	}()
	return out
}

// return a set of keys with specified prefix in the Storage
func (db *rocksdb) IterKeysWithPrefix(ctx context.Context, prefix []byte) <-chan []byte {
	var iter = db.rocksdb.NewIterator(db.readOptions)

	out := make(chan []byte)
	go func() {
		defer close(out)
		defer iter.Close()

		iter.Seek(prefix)
		for {
			if !iter.Valid() {
				return
			}

			key := iter.Key()
			if !bytes.HasPrefix(key.Data(), prefix) {
				return
			}
			select {
			case <-ctx.Done():
				return
			case out <- data(key):
				iter.Next()
			}
		}
	}()
	return out
}
