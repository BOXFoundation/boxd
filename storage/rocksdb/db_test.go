// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"testing"

	storage "github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/storage/dbtest"
	"github.com/facebookgo/ensure"
)

func randomPathB(b *testing.B) string {
	dir, err := ioutil.TempDir("", fmt.Sprintf("%d", rand.Int()))
	ensure.Nil(b, err)
	return dir
}

func randomPath(t *testing.T) string {
	dir, err := ioutil.TempDir("", fmt.Sprintf("%d", rand.Int()))
	ensure.Nil(t, err)
	return dir
}

func getDatabase() (string, storage.Storage, error) {
	dbpath, err := ioutil.TempDir("", fmt.Sprintf("%d", rand.Int()))
	if err != nil {
		return "", nil, err
	}

	db, err := NewRocksDB(dbpath, &storage.Options{})
	if err != nil {
		return dbpath, nil, err
	}
	return dbpath, db, nil
}

func releaseDatabase(dbpath string, db storage.Storage) {
	db.Close()
	os.RemoveAll(dbpath)
}

func TestDBCreateClose(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	err = db.Close()
	ensure.Nil(t, err)
}

func TestDBPut(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	t.Run("put1", dbtest.StoragePutGetDelTest(db, []byte("tk1"), []byte("tv1")))
	t.Run("put2", dbtest.StoragePutGetDelTest(db, []byte("tk2"), []byte("tv2")))
	t.Run("put3", dbtest.StoragePutGetDelTest(db, []byte("tk3"), []byte("tv3")))
	t.Run("put4", dbtest.StoragePutGetDelTest(db, []byte("tk4"), []byte("tv4")))
}

func TestDBDelNotExists(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	ensure.Nil(t, db.Del([]byte{0x00, 0x01}))
}

func TestDBDel(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	dbtest.StorageDel(t, db)
}

func TestDBBatch(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	dbtest.StorageBatch(t, db)
}

func TestDBBatchs(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprint("t", i), TestDBBatch)
	}
}

func TestDBKeys(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	dbtest.StorageKeys(t, db)(t, db)
}

func TestDBIterKeys(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	dbtest.StorageIterKeys(t, db)(t, db)
}

func TestDBIterKeysCancel(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	dbtest.StorageIterKeysCancel(t, db)(t, db)
}

func TestDBKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	dbtest.StoragePrefixKeys(t, db, 10000)(t, db)
}

func TestDBKeysWithPrefixRand(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	dbtest.StoragePrefixKeysRand(t, db)(t, db)
}

func TestDBIterKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	dbtest.StorageIterKeysWithPrefix(t, db)(t, db)
}

func TestDBIterKeysWithPrefixCancel(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	dbtest.StorageIterKeysWithPrefixCancel(t, db)(t, db)
}

func TestDBPersistent(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	verify := dbtest.StorageFillData(t, db, 1000)
	db.Close()

	db, err = NewRocksDB(dbpath, &storage.Options{})
	ensure.Nil(t, err)
	defer db.Close()

	verify(db)
}

func TestParallelPuts(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	var wg sync.WaitGroup
	const count = 10
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("n%06d", i)
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			dbtest.StorageGenDataWithVerification(t, db, name, 3000)(db)
		}(name)
	}
	wg.Wait()
}

func TestDBTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	dbtest.StorageTransOps(t, db)
}

func TestDBMulTransactions(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	dbtest.StorageMultiTrans(t, db)
}

func TestDBTransactionsClose(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	dbtest.StorageDBCloseForTransOpen(t, db, db)
}

func TestDBSyncTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	dbtest.StorageSyncTransaction(t, db)
}

func TestDBBatchAndTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	dbtest.StorageBatchAndTrans(t, db)
}

func TestDBTransactionKeys(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	verify := dbtest.StorageKeys(t, db)

	tx, _ := db.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}

func TestDBTransactionIterKeys(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	verify := dbtest.StorageIterKeys(t, db)
	tx, _ := db.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}

func TestDBTransactionKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	dbtest.StorageTransKeysWithPrefix(t, db)
}

func TestDBTransactionKeysWithPrefixRand(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	verify := dbtest.StoragePrefixKeysRand(t, db)
	tx, _ := db.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}

func TestDBTransactionIterKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	verify := dbtest.StorageIterKeysWithPrefix(t, db)
	tx, _ := db.NewTransaction()
	defer tx.Discard()
	verify(t, tx)
}

func TestDBTransactionsClosed(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	dbtest.StorageTransClosed(t, db)
}

////////////////////////////////////////////////////////////////////////////////
const chars = "1234567890abcdefhijklmnopqrstuvwxyzABCDEFHIJKLMNOPQRSTUVWXYZ"

func BenchmarkPutData(b *testing.B) {
	dbpath, db, err := getDatabase()
	ensure.Nil(b, err)
	defer releaseDatabase(dbpath, db)

	var k = []byte("k1")
	var v = make([]byte, 512*1024*1024)
	var l = len(chars)
	for i := 0; i < len(v); i++ {
		v[i] = chars[i%l]
	}

	b.ResetTimer()
	ensure.Nil(b, db.Put(k, v))
}

func BenchmarkGetData(b *testing.B) {
	dbpath, db, err := getDatabase()
	ensure.Nil(b, err)
	defer releaseDatabase(dbpath, db)

	var k = []byte("k1")
	var v = make([]byte, 512*1024*1024)
	var l = len(chars)
	for i := 0; i < len(v); i++ {
		v[i] = chars[i%l]
	}

	ensure.Nil(b, db.Put(k, v))

	b.ResetTimer()
	value, err := db.Get(k)
	ensure.Nil(b, err)
	b.StopTimer()

	ensure.DeepEqual(b, v, value)
}

func BenchmarkParallelGetData(b *testing.B) {
	dbpath, db, err := getDatabase()
	ensure.Nil(b, err)
	defer releaseDatabase(dbpath, db)

	var k = []byte("k1")
	var v = make([]byte, 32*1024*1024)
	var l = len(chars)
	for i := 0; i < len(v); i++ {
		v[i] = chars[i%l]
	}

	ensure.Nil(b, db.Put(k, v))

	b.N = 32
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			value, err := db.Get(k)
			ensure.Nil(b, err)
			ensure.DeepEqual(b, len(v), len(value))
		}
	})
}
