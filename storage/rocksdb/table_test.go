// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"fmt"
	"os"
	"testing"

	storage "github.com/BOXFoundation/boxd/storage"
	"github.com/facebookgo/ensure"
)

func TestTableCreateDrop(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	ensure.Nil(t, table.Put([]byte("1234"), []byte("4321")))
	ensure.Nil(t, table.Put([]byte("!&@%hdg"), []byte("djksfusm, dl")))
	ensure.Nil(t, db.DropTable("t1"))

	table, err = db.Table("t1")
	ensure.Nil(t, err)
	has, err := table.Has([]byte("1234"))
	ensure.Nil(t, err)
	ensure.False(t, has)
}

func TestTableCreate(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	t1, err := db.Table("t1")
	ensure.Nil(t, err)

	t2, err := db.Table("t1")
	ensure.Nil(t, err)

	ensure.True(t, t1 == t2)
}

func TestTablePutGetDel(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	t1, err := db.Table("t1")
	ensure.Nil(t, err)

	t.Run("put1", storagePutGetDelTest(t1, []byte("tk1"), []byte("tv1")))
	t.Run("put2", storagePutGetDelTest(t1, []byte("tk2"), []byte("tv2")))
	t.Run("put3", storagePutGetDelTest(t1, []byte("tk3"), []byte("tv3")))
	t.Run("put4", storagePutGetDelTest(t1, []byte("tk4"), []byte("tv4")))
}

func TestTableDelNotExists(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("t1")
	ensure.Nil(t, err)
	ensure.Nil(t, table.Del([]byte{0x00, 0x01}))
}

func TestTableDel(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	storageDel(t, table)
}

func TestTableBatch(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	storageBatch(t, table)
}

func TestTableBatchs(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprint("t", i), TestTableBatch)
	}
}

func TestTableKeys(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	storageKeys(t, table, 10000)
}

func TestTableKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	storagePrefixKeys(t, table, 10000)
}

func TestTablePersistent(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	table, err := db.Table("t")
	ensure.Nil(t, err)

	verify := storageFillData(t, table, 1000)
	db.Close()

	db, err = NewRocksDB(dbpath, &storage.Options{})
	ensure.Nil(t, err)
	defer db.Close()

	table, err = db.Table("t")
	ensure.Nil(t, err)

	verify(table)
}

func TestTableTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	table, _ := db.Table("t1")
	storageTransOps(t, table)
}

func TestTableMulTransactions(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	t1, _ := db.Table("tx")
	storageMultiTransTable(t, t1)
}

func TestTableTransactionsClose(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	table, _ := db.Table("t1")

	storageDBCloseForTransOpen(t, table, db)
}

func TestTableSyncTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	table, _ := db.Table("t1")
	storageSyncTransaction(t, table)
}

func TestTableBatchAndTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	table, _ := db.Table("t1")
	storageBatchAndTrans(t, table)
}

func TestTableTransactionsClosed(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	table, _ := db.Table("t1")
	storageTransClosed(t, table)
}

func TestTableTransactionKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, _ := db.Table("trans")
	storageTransKeysWithPrefix(t, table)
}
