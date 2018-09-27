// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

// Operations wraps the basic storage operations
type Operations interface {
	// put the k-v to the Storage for database write operation
	Put(key, value []byte) error

	// delete the entry associate with the key in the Storage
	Del(key []byte) error

	Get(key []byte) ([]byte, error)

	Has(key []byte) (bool, error)

	// return a set of keys in the Storage
	Keys() [][]byte
}

// Batch wraps the batch options
type Batch interface {
	Operations

	// commit changes to db
	Commit() error

	// reset
	Rollback() error
}

// Table wraps the interface of a db table
type Table interface {
	Operations

	BeginTransaction() Batch
}

// Storage wraps the whole db interface
type Storage interface {
	Operations

	BeginTransaction() Batch

	Table(name string) Table

	Close() error
}
