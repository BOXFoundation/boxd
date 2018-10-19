// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

// Transaction allow user to read/write Storage atomicly.
// Actions performed on a transaction will not take hold until a successful
// call to Commit has been made.
type Transaction interface {
	Operations

	// Commit finalizes a transaction, attempting to commit it to the Storage.
	Commit() error

	// Discard throws away changes recorded in a transaction without committing.
	// them to the underlying Storage. Any calls made to Discard after Commit
	// has been successfully called will have no effect on the transaction and
	// state of the Storage, making it safe to defer.
	Discard()
}
