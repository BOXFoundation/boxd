// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

// Storage wraps storage operations
type Storage interface {
	// put the k-v to the Storage for database write operation
	Put(key, value []byte) error

	// return value associate with the key in the Storage
	Get(key []byte) ([]byte, error)

	// delete the entry associate with the key in the Storage
	Del(key []byte) error

	Has(key []byte) (bool, error)

	// return a set of keys in the Storage
	Keys() [][]byte

	Flush() error

	Close() error
}

// Transaction supports operations: 1. prepare   2. put the data for temporary    3. commit or rollback
// TODO: add API for multiple transactions
type Transaction interface {
	Prepare() error

	Commit() error

	Rollback() error
}
