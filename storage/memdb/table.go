// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package memdb

import (
	"errors"
	"strings"

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

// put the value to entry associate with the key
func (t *mtable) Put(key, value []byte) error {
	t.sm.Lock()
	defer t.sm.Unlock()

	t.db[string(t.realkey(key))] = value

	return nil
}

// delete the entry associate with the key in the Storage
func (t *mtable) Del(key []byte) error {
	t.sm.Lock()
	defer t.sm.Unlock()

	delete(t.db, string(t.realkey(key)))

	return nil
}

// return value associate with the key in the Storage
func (t *mtable) Get(key []byte) ([]byte, error) {
	t.sm.Lock()
	defer t.sm.Unlock()

	if value, ok := t.db[string(t.realkey(key))]; ok {
		return value, nil
	}
	return nil, errors.New("key not found")
}

// check if the entry associate with key exists
func (t *mtable) Has(key []byte) (bool, error) {
	t.sm.Lock()
	defer t.sm.Unlock()

	_, ok := t.db[string(t.realkey(key))]

	return ok, nil
}

// return a set of keys in the Storage
func (t *mtable) Keys() [][]byte {
	t.sm.Lock()
	defer t.sm.Unlock()

	var keys [][]byte
	for key := range t.db {
		if strings.HasPrefix(key, t.prefix) {
			keys = append(keys, []byte(key)[len(t.prefix):])
		}
	}

	return keys
}
