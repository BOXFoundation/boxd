// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"testing"

	storage "github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/storage/dbtest"
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

	t.Run("put1", dbtest.StoragePutGetDelTest(t1, []byte("tk1"), []byte("tv1")))
	t.Run("put2", dbtest.StoragePutGetDelTest(t1, []byte("tk2"), []byte("tv2")))
	t.Run("put3", dbtest.StoragePutGetDelTest(t1, []byte("tk3"), []byte("tv3")))
	t.Run("put4", dbtest.StoragePutGetDelTest(t1, []byte("tk4"), []byte("tv4")))
}

func _TestTableBatchAndPut(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	t1, err := db.Table("t1")
	ensure.Nil(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		bt := t1.(*rtable).NewBatch()
		for i := 0; i < 100000; i++ {
			buf := make([]byte, 4)
			binary.LittleEndian.PutUint32(buf, uint32(i))
			bt.Put(buf, buf)
		}
		if err := bt.Write(); err != nil {
			t.Fatal(err)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		var batch = t1.NewBatch()
		for i := 50000; i < 150000; i++ {
			buf := make([]byte, 4)
			binary.LittleEndian.PutUint32(buf, uint32(i))
			batch.Put(buf, buf)
		}
		if err := batch.Write(); err != nil {
			t.Fatal(err)
		}
		wg.Done()
	}()

	for i := 50000; i < 150000; i++ {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(i))
		if err := t1.Put(buf, buf); err != nil {
			t.Fatal(err)
		}
	}

	wg.Wait()
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

	dbtest.StorageDel(t, table)
}

func TestTableBatch(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

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
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	dbtest.StorageKeys(t, table)(t, table)
}

func TestTableIterKeys(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, _ := db.Table("t1")
	verify := dbtest.StorageIterKeys(t, table)

	verify(t, table)
}

func TestTableIterKeysCancel(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, _ := db.Table("t1")
	dbtest.StorageIterKeysCancel(t, table)(t, table)
}

func TestTableKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	dbtest.StoragePrefixKeys(t, table, 10000)(t, table)
}

func TestTableKeysWithPrefixRand(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	dbtest.StoragePrefixKeysRand(t, table)(t, table)
}

func TestTableIterKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, _ := db.Table("t1")
	dbtest.StorageIterKeysWithPrefix(t, table)(t, table)
}

func TestTableIterKeysWithPrefixCancel(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, _ := db.Table("t1")
	dbtest.StorageIterKeysWithPrefixCancel(t, table)(t, table)
}

func TestTablePersistent(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	table, err := db.Table("t")
	ensure.Nil(t, err)

	verify := dbtest.StorageFillData(t, table, 1000)
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
	dbtest.StorageTransOps(t, table)
}

func TestTableMulTransactions(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	t1, _ := db.Table("tx")
	dbtest.StorageMultiTransTable(t, t1)
}

func TestTableTransactionsClose(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	table, _ := db.Table("t1")

	dbtest.StorageDBCloseForTransOpen(t, table, db)
}

func TestTableSyncTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	table, _ := db.Table("t1")
	dbtest.StorageSyncTransaction(t, table)
}

func TestTableBatchAndTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	table, _ := db.Table("t1")
	dbtest.StorageBatchAndTrans(t, table)
}

func TestTableTransactionsClosed(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	table, _ := db.Table("t1")
	dbtest.StorageTransClosed(t, table)
}

func TestTableTransactionKeys(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, _ := db.Table("trans")
	verify := dbtest.StorageKeys(t, table)

	tx, _ := table.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}

func TestTableTransactionKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, _ := db.Table("trans")
	dbtest.StorageTransKeysWithPrefix(t, table)
}

func TestTableTransactionKeysWithPrefixRand(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, _ := db.Table("trans")

	verify := dbtest.StoragePrefixKeysRand(t, table)
	tx, _ := table.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}

func TestTableTransactionIterKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, _ := db.Table("trans")
	verify := dbtest.StorageIterKeysWithPrefix(t, table)

	tx, _ := table.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}

func TestTableTransactionIterKeys(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, _ := db.Table("trans")
	verify := dbtest.StorageIterKeys(t, table)

	tx, _ := table.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}
