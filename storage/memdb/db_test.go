// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package memdb

import (
	"fmt"
	"testing"

	"github.com/BOXFoundation/boxd/storage/dbtest"
	"github.com/facebookgo/ensure"
)

func TestDBCreateClose(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)

	err = db.Close()
	ensure.Nil(t, err)
}

func TestDBPut(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	t.Run("put1", dbtest.StoragePutGetDelTest(db, []byte("tk1"), []byte("tv1")))
	t.Run("put2", dbtest.StoragePutGetDelTest(db, []byte("tk2"), []byte("tv2")))
	t.Run("put3", dbtest.StoragePutGetDelTest(db, []byte("tk3"), []byte("tv3")))
	t.Run("put4", dbtest.StoragePutGetDelTest(db, []byte("tk4"), []byte("tv4")))
}

func TestDBDel(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	dbtest.StorageDel(t, db)
}

func TestDBBatch(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	dbtest.StorageBatch(t, db)
}

func TestDBBatchs(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprint("t", i), TestDBBatch)
	}
}

func TestDBKeys(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	dbtest.StorageKeys(t, db)(t, db)
}

func TestDBKeysWithPrefix(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	dbtest.StoragePrefixKeys(t, db, 10000)(t, db)
}

func TestDBKeysWithPrefixRand(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	dbtest.StoragePrefixKeysRand(t, db)(t, db)
}

func TestDBTransaction(t *testing.T) {
	db, _ := NewMemoryDB("", nil)
	defer db.Close()

	dbtest.StorageTransOps(t, db)
}

func TestDBTransactionsClose(t *testing.T) {
	db, _ := NewMemoryDB("", nil)
	defer db.Close()

	tx, err := db.NewTransaction()
	ensure.Nil(t, err)
	ensure.NotNil(t, tx)
	defer tx.Discard()
}

func TestDBSyncTransaction(t *testing.T) {
	db, _ := NewMemoryDB("", nil)
	defer db.Close()

	dbtest.StorageSyncTransaction(t, db)
}

func TestDBTransKeysWithPrefixRand(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	verify := dbtest.StoragePrefixKeysRand(t, db)
	tx, _ := db.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}
