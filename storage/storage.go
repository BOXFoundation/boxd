// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

// Storage defines the data persistanse methods
type Storage interface {
	Table

	// Create or Get the table associate with the name
	Table(string) (Table, error)
	DropTable(string) error

	Close() error
}

// Options defines the db options
type Options map[string]interface{}
