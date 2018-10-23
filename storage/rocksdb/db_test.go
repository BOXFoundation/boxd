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

	t.Run("put1", storagePutGetDelTest(db, []byte("tk1"), []byte("tv1")))
	t.Run("put2", storagePutGetDelTest(db, []byte("tk2"), []byte("tv2")))
	t.Run("put3", storagePutGetDelTest(db, []byte("tk3"), []byte("tv3")))
	t.Run("put4", storagePutGetDelTest(db, []byte("tk4"), []byte("tv4")))
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

	storageDel(t, db)
}

func TestDBBatch(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	storageBatch(t, db)
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

	storageKeys(t, db, 10000)
}

func TestDBKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	storagePrefixKeys(t, db, 10000)
}

func TestDBPersistent(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	verify := storageFillData(t, db, 1000)
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
			storageGenDataWithVerification(t, db, name, 3000)(db)
		}(name)
	}
	wg.Wait()
}

func TestDBTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	storageTransOps(t, db)
}

func TestDBMulTransactions(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	storageMultiTrans(t, db)
}

func TestDBTransactionsClose(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	storageDBCloseForTransOpen(t, db, db)
}

func TestDBSyncTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	storageSyncTransaction(t, db)
}

func TestDBBatchAndTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	storageBatchAndTrans(t, db)
}

func TestDBTransactionKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	storageTransKeysWithPrefix(t, db)
}

func TestDBTransactionsClosed(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	storageTransClosed(t, db)
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
