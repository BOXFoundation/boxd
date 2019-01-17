// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
)

// Writer defines the write operations on database/table
type Writer interface {
	// put the value to entry associate with the key
	Put(key, value []byte) error

	// delete the entry associate with the key in the Storage
	Del(key []byte) error
}

// Reader defines common read operations on database/table
type Reader interface {
	// return value associate with the key in the Storage
	Get(key []byte) ([]byte, error)

	// return values associate with the keys in the Storage
	MultiGet(key ...[]byte) ([][]byte, error)

	// check if the entry associate with key exists
	Has(key []byte) (bool, error)

	// return a set of keys in the Storage
	Keys() [][]byte

	// return a chan to iter all keys
	IterKeys(ctx context.Context) <-chan []byte

	// return a set of keys with specified prefix in the Storage
	KeysWithPrefix(prefix []byte) [][]byte

	// return a chan to iter all keys with specified prefix
	IterKeysWithPrefix(ctx context.Context, prefix []byte) <-chan []byte
}

// Operations defines common data operations on database/table
type Operations interface {
	Writer
	Reader
}
