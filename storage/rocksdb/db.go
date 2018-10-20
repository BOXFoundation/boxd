// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"io/ioutil"

	"github.com/BOXFoundation/boxd/log"
	storage "github.com/BOXFoundation/boxd/storage"
	"github.com/tecbot/gorocksdb"
)

var logger = log.NewLogger("rocksdb")

const number = 10
const cache = 3 << 30

func init() {
	// register rocksdb impl
	storage.Register("rocksdb", NewRocksDB)
}

func prepare(path string) {
	files, err := ioutil.ReadDir(path)
	if err != nil || len(files) == 0 {
		dbpath := gorocksdb.NewDBPath(path, 0)
		defer dbpath.Destroy()
	}
}

// NewRocksDB creates a rocksdb instance
func NewRocksDB(name string, o *storage.Options) (storage.Storage, error) {
	logger.Infof("Creating rocksdb at %s", name)

	// bbto := gorocksdb.NewDefaultBlockBasedTableOptions()
	// filter := gorocksdb.NewBloomFilter(number)
	// bbto.SetFilterPolicy(filter)

	// bbto.SetBlockCache(gorocksdb.NewLRUCache(cache))
	options := gorocksdb.NewDefaultOptions()
	// options.SetBlockBasedTableFactory(bbto)
	options.SetCreateIfMissing(true)
	options.SetCreateIfMissingColumnFamilies(true)
	options.SetMaxBackgroundFlushes(4)

	prepare(name)
	// get all column families
	cfnames, err := gorocksdb.ListColumnFamilies(options, name)
	if err != nil {
		logger.Debug(err)
	}

	var cfhandlers []*gorocksdb.ColumnFamilyHandle
	var db *gorocksdb.DB
	if len(cfnames) == 0 {
		db, err = gorocksdb.OpenDb(options, name)
	} else {
		// column families options
		var cfoptions = make([]*gorocksdb.Options, len(cfnames))
		for i := range cfnames {
			cfoptions[i] = options
		}

		// open database with column families
		db, cfhandlers, err = gorocksdb.OpenDbColumnFamilies(options, name, cfnames, cfoptions)
		if err != nil {
			return nil, err
		}
	}

	d := &rocksdb{
		rocksdb:      db,
		cfs:          map[string]*gorocksdb.ColumnFamilyHandle{},
		tables:       map[string]*rtable{},
		dboptions:    options,
		readOptions:  gorocksdb.NewDefaultReadOptions(),
		writeOptions: gorocksdb.NewDefaultWriteOptions(),
		flushOptions: gorocksdb.NewDefaultFlushOptions(),
		writeLock:    make(chan struct{}, 1),
	}
	// d.flushOptions.SetWait(true)
	// d.writeOptions.SetSync(true)

	for i, cfhandler := range cfhandlers {
		d.cfs[cfnames[i]] = cfhandler
	}

	return d, nil
}
