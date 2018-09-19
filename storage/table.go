// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

type table struct {
	storage Storage
	prefix  string
}

var _ Storage = (*table)(nil)

// NewTable returns a Storage object that prefixes all keys with a given string
func NewTable(storage Storage, prefix string) Storage {
	return &table{
		storage: storage,
		prefix:  prefix,
	}
}

func (t *table) Has(key []byte) (bool, error) {
	return t.storage.Has(append([]byte(t.prefix), key...))
}

func (t *table) Put(key []byte, value []byte) error {
	return t.storage.Put(append([]byte(t.prefix), key...), value)
}

func (t *table) Get(key []byte) ([]byte, error) {
	return t.storage.Get(append([]byte(t.prefix), key...))
}

func (t *table) Del(key []byte) error {
	return t.storage.Del(append([]byte(t.prefix), key...))
}

func (t *table) Keys() [][]byte {
	return nil
}

func (t *table) Flush() error {
	return nil
}

func (t *table) Close() error {
	return nil
}
