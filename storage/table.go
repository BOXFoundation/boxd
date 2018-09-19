// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

type table struct {
	storage Storage
	prefix  string
}

// NewTable returns a Storage object that prefixes all keys with a given string
func NewTable(storage Storage, prefix string) Storage {
	return &table{
		storage: storage,
		prefix:  prefix,
	}
}

func (nt *table) Has(key []byte) (bool, error) {
	return nt.storage.Has(append([]byte(nt.prefix), key...))
}

func (nt *table) Put(key []byte, value []byte) error {
	return nt.storage.Put(append([]byte(nt.prefix), key...), value)
}

func (nt *table) Get(key []byte) ([]byte, error) {
	return nt.storage.Get(append([]byte(nt.prefix), key...))
}

func (nt *table) Del(key []byte) error {
	return nt.storage.Del(append([]byte(nt.prefix), key...))
}

func (nt *table) Keys() [][]byte {
	return nil
}

func (nt *table) Close() {}
