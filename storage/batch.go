// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

// Batch defines the batch of put, del operations
type Batch interface {
	// put the value to entry associate with the key
	Put(key, value []byte) error

	// delete the entry associate with the key in the Storage
	Del(key []byte) error

	// remove all the enqueued put/delete
	Clear()

	// returns the number of updates in the batch
	Count() int

	// atomic writes all enqueued put/delete
	Write() error

	// close the batch, it must be called to close the batch
	Close()
}
