// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"sync"

	"github.com/BOXFoundation/Quicksilver/log"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("storage")

// Config defines the database configuration
type Config struct {
	Name    string
	Path    string
	Options Options
}

// Options defines the options of database impl
type Options struct {
	//options map[string]interface{}
}

// Database is a wrapper of Storage, implementing the database life cycle
type Database struct {
	Storage
	proc goprocess.Process
	sm   sync.Mutex
}

type newDatabaseFunc func(*Config) (Storage, error)

var dbs = map[string]newDatabaseFunc{
	"mem":     NewMemoryStorage,
	"rocksdb": NewRocksDBStorage,
}

// NewDatabase creates a database instance
func NewDatabase(parent goprocess.Process, cfg *Config) (*Database, error) {
	var storageFunc, ok = dbs[cfg.Name]
	if !ok {
		storageFunc = NewMemoryStorage
	}

	var db, err = storageFunc(cfg)
	if err != nil {
		return nil, err
	}

	var database = &Database{
		Storage: db,
		proc:    goprocess.WithParent(parent),
	}
	database.proc.SetTeardown(database.shutdown)
	return database, nil
}

// Close closes the database
func (db *Database) Close() error {
	db.sm.Lock()

	if db.proc != nil {
		defer db.sm.Unlock()
		return db.proc.Close()
	}
	db.sm.Unlock()
	return db.shutdown()
}

// the real shutdown func of database
func (db *Database) shutdown() error {
	db.sm.Lock()
	defer db.sm.Unlock()

	logger.Info("Shutdown database...")
	db.Storage.Close()
	return nil
}
