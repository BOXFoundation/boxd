// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package memdb

import (
	"fmt"
	"testing"

	dbtest "github.com/BOXFoundation/boxd/storage/dbtest"
	"github.com/facebookgo/ensure"
)

func TestTableCreateClose(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	ensure.Nil(t, table.Put([]byte("!&@%hdg"), []byte("djksfusm, dl")))
	ensure.Nil(t, db.DropTable("t1"))
}

func TestTableDel(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("t1")
	ensure.Nil(t, err)
	dbtest.StorageDel(t, table)
}

func TestTableBatch(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	dbtest.StorageBatch(t, table)
}

func TestTableBatchs(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprint("t", i), TestTableBatch)
	}
}

func TestTableKeys(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	dbtest.StorageKeys(t, table)(t, table)
}

func TestTableIterKeys(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	dbtest.StorageIterKeys(t, table)(t, table)
}

func TestTableIterKeysCancel(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	dbtest.StorageIterKeysCancel(t, table)(t, table)
}

func TestTableKeysWithPrefix(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	dbtest.StoragePrefixKeys(t, table, 10000)(t, table)
}

func TestTableKeysWithPrefixRand(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	dbtest.StoragePrefixKeysRand(t, table)(t, table)
}

func TestTableIterKeysWithPrefix(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	dbtest.StorageIterKeysWithPrefix(t, table)(t, table)
}

func TestTableIterKeysWithPrefixCancel(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	dbtest.StorageIterKeysWithPrefixCancel(t, table)(t, table)
}

func TestTableTransaction(t *testing.T) {
	db, _ := NewMemoryDB("", nil)
	defer db.Close()
	table, _ := db.Table("t1")

	dbtest.StorageTransOps(t, table)
}

func TestTableMulTransactions(t *testing.T) {
	db, _ := NewMemoryDB("", nil)
	defer db.Close()
	t1, _ := db.Table("t1")

	dbtest.StorageMultiTransTable(t, t1)
}

func TestTableTransactionsClose(t *testing.T) {
	db, _ := NewMemoryDB("", nil)
	defer db.Close()
	table, _ := db.Table("t1")

	tx, err := table.NewTransaction()
	ensure.Nil(t, err)
	ensure.NotNil(t, tx)
	defer tx.Discard()
}

func TestTableSyncTransaction(t *testing.T) {
	db, _ := NewMemoryDB("", nil)
	defer db.Close()

	table, _ := db.Table("t1")
	dbtest.StorageSyncTransaction(t, table)
}

func TestTableTransKeysWithPrefixRand(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	verify := dbtest.StoragePrefixKeysRand(t, table)
	tx, _ := table.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}

func TestTableTransKeys(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	verify := dbtest.StorageKeys(t, table)
	tx, _ := table.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}

func TestTableTransIterKeys(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	verify := dbtest.StorageIterKeys(t, table)
	tx, _ := table.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}

func TestTableTransIterKeysWithPrefix(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	verify := dbtest.StorageIterKeysWithPrefix(t, table)
	tx, _ := table.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}
