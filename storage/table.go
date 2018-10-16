// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

// Table defines all methods of database table.
type Table interface {
	Operations

	// create a new write batch
	NewBatch() Batch
}
