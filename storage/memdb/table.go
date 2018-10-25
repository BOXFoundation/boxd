// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package memdb

import (
	"context"
	"strings"
	"time"

	storage "github.com/BOXFoundation/boxd/storage"
)

type mtable struct {
	*memorydb

	prefix string
}

var _ storage.Table = (*mtable)(nil)

// create a new write batch
func (t *mtable) NewBatch() storage.Batch {
	return &mbatch{
		memorydb: t.memorydb,
		prefix:   t.prefix,
	}
}

func (t *mtable) realkey(key []byte) []byte {
	var k = make([]byte, len(t.prefix)+len(key))
	copy(k, []byte(t.prefix))
	copy(k[len(t.prefix):], key)

	return k
}

func (t *mtable) NewTransaction() (storage.Transaction, error) {
	timer := time.NewTimer(time.Millisecond * 100)
	select {
	case <-timer.C:
		return nil, storage.ErrTransactionExists
	case t.writeLock <- struct{}{}:
	}

	return &mtx{
		db:        t,
		closed:    false,
		batch:     &mbatch{memorydb: t.memorydb, prefix: t.prefix},
		writeLock: t.writeLock,
	}, nil
}

// put the value to entry associate with the key
func (t *mtable) Put(key, value []byte) error {
	t.writeLock <- struct{}{}
	defer func() {
		<-t.writeLock
	}()

	t.sm.Lock()
	defer t.sm.Unlock()

	t.db[string(t.realkey(key))] = value

	return nil
}

// delete the entry associate with the key in the Storage
func (t *mtable) Del(key []byte) error {
	t.writeLock <- struct{}{}
	defer func() {
		<-t.writeLock
	}()

	t.sm.Lock()
	defer t.sm.Unlock()

	delete(t.db, string(t.realkey(key)))

	return nil
}

// return value associate with the key in the Storage
func (t *mtable) Get(key []byte) ([]byte, error) {
	t.sm.RLock()
	defer t.sm.RUnlock()

	if value, ok := t.db[string(t.realkey(key))]; ok {
		return value, nil
	}
	return nil, nil
}

// check if the entry associate with key exists
func (t *mtable) Has(key []byte) (bool, error) {
	t.sm.RLock()
	defer t.sm.RUnlock()

	_, ok := t.db[string(t.realkey(key))]

	return ok, nil
}

// return a set of keys in the Storage
func (t *mtable) Keys() [][]byte {
	return t.KeysWithPrefix([]byte{})
}

func (t *mtable) KeysWithPrefix(prefix []byte) [][]byte {
	t.sm.RLock()
	defer t.sm.RUnlock()

	var keys [][]byte
	for key := range t.db {
		if strings.HasPrefix(key, t.prefix+string(prefix)) {
			keys = append(keys, []byte(key)[len(t.prefix):])
		}
	}

	return keys
}

// return a chan to iter all keys
func (t *mtable) IterKeys(ctx context.Context) <-chan []byte {
	keys := t.Keys()

	out := make(chan []byte)
	go func() {
		defer close(out)

		for _, k := range keys {
			select {
			case <-ctx.Done():
				return
			case out <- k:
			}
		}
	}()
	return out
}

// return a set of keys with specified prefix in the Storage
func (t *mtable) IterKeysWithPrefix(ctx context.Context, prefix []byte) <-chan []byte {
	keys := t.KeysWithPrefix(prefix)

	out := make(chan []byte)
	go func() {
		defer close(out)

		for _, k := range keys {
			select {
			case out <- k:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
