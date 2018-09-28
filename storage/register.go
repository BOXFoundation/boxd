// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import "fmt"

// newDBFunc defines the function to create a new storage instance.
type newDBFunc func(string, *Options) (Storage, error)

var dbfuncs = make(map[string]newDBFunc)

// Register registers a new DB implementation
func Register(dbname string, fn newDBFunc) {
	dbfuncs[dbname] = fn
}

// newStorage creates a new storage instance associate with specified dbname
func newStorage(dbname string, dbpath string, o *Options) (Storage, error) {
	if dbfunc, ok := dbfuncs[dbname]; ok {
		return dbfunc(dbpath, o)
	}

	return nil, fmt.Errorf("storage %s is not found", dbname)
}
