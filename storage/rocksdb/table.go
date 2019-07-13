// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"bytes"
	"context"
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

	enableBatch bool
	batch       storage.Batch

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

func (t *rtable) NewTransaction() (tr storage.Transaction, err error) {
	defer func() {
		if recover() != nil {
			tr = nil
			err = storage.ErrDatabasePanic
		}
	}()

	// lock all write operations
	t.writeLock <- struct{}{}
	tr = &dbtx{
		db:        t,
		batch:     t.NewBatch(),
		closed:    false,
		writeLock: t.writeLock,
	}

	return tr, nil
}

func (t *rtable) EnableBatch() {
	t.enableBatch = true
	t.batch = t.NewBatch()
}

// DisableBatch disable batch write.
func (t *rtable) DisableBatch() {
	t.sm.Lock()
	defer t.sm.Unlock()
	t.enableBatch = false
	t.batch.Close()
}

// IsInBatch indicates whether db is in batch
func (t *rtable) IsInBatch() bool {
	t.sm.Lock()
	defer t.sm.Unlock()
	return t.enableBatch
}

// put the value to entry associate with the key
func (t *rtable) Put(key, value []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = storage.ErrDatabasePanic
		}
	}()

	if t.enableBatch {
		t.batch.Put(key, value)
	} else {
		t.writeLock <- struct{}{}
		err = t.rocksdb.PutCF(t.writeOptions, t.cf, key, value)
		<-t.writeLock
	}

	return err
}

// delete the entry associate with the key in the Storage
func (t *rtable) Del(key []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = storage.ErrDatabasePanic
		}
	}()

	if t.enableBatch {
		t.batch.Del(key)
	} else {
		t.writeLock <- struct{}{}
		err = t.rocksdb.DeleteCF(t.writeOptions, t.cf, key)
		<-t.writeLock
	}

	return err
}

// Flush atomic writes all enqueued put/delete
func (t *rtable) Flush() (err error) {
	defer func() {
		if recover() != nil {
			err = storage.ErrDatabasePanic
		}
	}()

	if t.enableBatch {
		err = t.batch.Write()
	} else {
		err = storage.ErrOnlySupportBatchOpt
	}
	return err
}

// return value associate with the key in the Storage
func (t *rtable) Get(key []byte) ([]byte, error) {
	value, err := t.rocksdb.GetCF(t.readOptions, t.cf, key)
	if err != nil {
		return nil, err
	}

	return data(value), nil
}

// return values associate with the keys in the Storage
func (t *rtable) MultiGet(key ...[]byte) ([][]byte, error) {
	slices, err := t.rocksdb.MultiGetCF(t.readOptions, t.cf, key...)
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

	iter.Seek(prefix)
	var keys [][]byte
	for it := iter; it.Valid(); it.Next() {
		key := it.Key()
		if bytes.HasPrefix(key.Data(), prefix) {
			keys = append(keys, data(key))
		} else {
			key.Free()
			break
		}
	}
	return keys
}

// return a chan to iter all keys
func (t *rtable) IterKeys(ctx context.Context) <-chan []byte {
	var iter = t.rocksdb.NewIteratorCF(t.readOptions, t.cf)
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
func (t *rtable) IterKeysWithPrefix(ctx context.Context, prefix []byte) <-chan []byte {
	var iter = t.rocksdb.NewIteratorCF(t.readOptions, t.cf)
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
				key.Free()
				return
			}
			select {
			case <-ctx.Done():
				key.Free()
				return
			case out <- data(key):
				iter.Next()
			}
		}
	}()
	return out
}

func (t *rtable) Close() {
	close(t.writeLock)
	t.cf.Destroy()
}
