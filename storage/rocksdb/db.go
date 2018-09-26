// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	types "github.com/BOXFoundation/Quicksilver/storage/types"
	"github.com/tecbot/gorocksdb"
)

// NewRocksdb creates a rocksdb instance
func NewRocksdb(name string, o *types.Options) (types.Storage, error) {
	bbto := gorocksdb.NewDefaultBlockBasedTableOptions()
	filter := gorocksdb.NewBloomFilter(number)
	bbto.SetFilterPolicy(filter)

	bbto.SetBlockCache(gorocksdb.NewLRUCache(cache))
	options := gorocksdb.NewDefaultOptions()
	options.SetBlockBasedTableFactory(bbto)
	options.SetCreateIfMissing(true)

	// get all column families
	cfnames, err := gorocksdb.ListColumnFamilies(options, name)
	if err != nil {
		return nil, err
	}
	// column families options
	var cfoptions = make([]*gorocksdb.Options, len(cfnames))
	for i := range cfnames {
		cfoptions[i] = options
	}

	// open database with column families
	db, cfhandlers, err := gorocksdb.OpenDbColumnFamilies(options, name, cfnames, cfoptions)
	if err != nil {
		return nil, err
	}

	d := &rocksdb{
		rocksdb:      db,
		cfs:          make(map[string]*gorocksdb.ColumnFamilyHandle),
		dboptions:    options,
		readOptions:  gorocksdb.NewDefaultReadOptions(),
		writeOptions: gorocksdb.NewDefaultWriteOptions(),
		flushOptions: gorocksdb.NewDefaultFlushOptions(),
	}

	for i, cfhandler := range cfhandlers {
		d.cfs[cfnames[i]] = cfhandler
	}

	return d, nil
}
