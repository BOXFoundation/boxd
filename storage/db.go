// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"sync"

	"github.com/BOXFoundation/boxd/log"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("storage")

// Config defines the database configuration
type Config struct {
	Name    string  `mapstructure:"name"`
	Path    string  `mapstructure:"path"`
	Options Options `mapstructure:"options"`
}

// Database is a wrapper of Storage, implementing the database life cycle
type Database struct {
	Storage
	proc goprocess.Process
	sm   sync.Mutex
}

// NewDatabase creates a database instance
func NewDatabase(parent goprocess.Process, cfg *Config) (*Database, error) {
	var storage, err = newStorage(cfg.Name, cfg.Path, &cfg.Options)
	if err != nil {
		return nil, err
	}

	var database = &Database{
		Storage: storage,
		proc:    goprocess.WithParent(parent),
	}
	database.proc.SetTeardown(database.shutdown)
	return database, nil
}

// Proc returns the gopreocess of database
func (db *Database) Proc() goprocess.Process {
	return db.proc
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
