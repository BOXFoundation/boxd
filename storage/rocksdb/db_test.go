// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

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
}

func TestDBCreateClose(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	err = db.Close()
	ensure.Nil(t, err)
}

var dbPutReadOnceTest = func(db storage.Storage, k, v []byte) func(*testing.T) {
	return func(t *testing.T) {
		db.Put(k, v)
		has, err := db.Has(k)
		ensure.Nil(t, err)
		ensure.True(t, has)

		value, err := db.Get(k)
		ensure.Nil(t, err)
		ensure.True(t, bytes.Equal(value, v))

		ensure.Nil(t, db.Del(k))
		has, err = db.Has(k)
		ensure.Nil(t, err)
		ensure.False(t, has)
	}
}

func TestDBPut(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	t.Run("put1", dbPutReadOnceTest(db, []byte("tk1"), []byte("tv1")))
	t.Run("put2", dbPutReadOnceTest(db, []byte("tk2"), []byte("tv2")))
	t.Run("put3", dbPutReadOnceTest(db, []byte("tk3"), []byte("tv3")))
	t.Run("put4", dbPutReadOnceTest(db, []byte("tk4"), []byte("tv4")))
}

func TestDBDel(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	var wg sync.WaitGroup

	var keys = []string{}
	for i := 0; i < 10000; i++ {
		k := fmt.Sprintf("key-%d", i)
		v := fmt.Sprintf("value-%d", i)

		ensure.Nil(t, db.Put([]byte(k), []byte(v)))
		wg.Add(1)
		keys = append(keys, k)
	}

	for _, k := range keys {
		go func(k []byte) {
			has, err := db.Has(k)
			ensure.Nil(t, err)
			ensure.True(t, has)

			ensure.Nil(t, db.Del(k))

			has, err = db.Has(k)
			ensure.Nil(t, err)
			ensure.False(t, has)
			wg.Done()
		}([]byte(k))
	}
	wg.Wait()
}

func TestDBBatch(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	var count = 500
	var kvs = map[string][]byte{}
	var delkeys = []string{}
	for i := 0; i < count; i++ {
		k := fmt.Sprintf("key-%d", i)
		v := fmt.Sprintf("value-%d", i)
		kvs[k] = []byte(v)
		if i%3 == 0 {
			delkeys = append(delkeys, k)
		}
	}

	var batch = db.NewBatch()
	defer batch.Close()

	for k, v := range kvs {
		batch.Put([]byte(k), v)
	}
	ensure.True(t, batch.Count() == count)

	for _, k := range delkeys {
		batch.Del([]byte(k))
	}
	var countAfterDel = count + len(delkeys)
	ensure.True(t, batch.Count() == countAfterDel)

	if err := batch.Write(); err != nil {
		t.Fatal(err)
	}

	for _, k := range delkeys {
		delete(kvs, k)
		exist, err := db.Has([]byte(k))
		ensure.Nil(t, err)
		ensure.False(t, exist)
	}

	var wg sync.WaitGroup
	for k, v := range kvs {
		wg.Add(1)

		go func(k, v []byte) {
			defer wg.Done()

			value, err := db.Get(k)
			ensure.Nil(t, err)
			if !bytes.Equal(v, value) {
				t.Fatalf("value of key %s: expected %s, but actually %s", string(k), string(v), string(value))
			}
		}([]byte(k), v)
	}
	wg.Wait()
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

	var count = 10000
	var keys = map[string][]byte{}
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		db.Put(k, v)
		keys[string(k)] = k
	}

	var ks [][]byte
	for _, k := range db.Keys() {
		if _, ok := keys[string(k)]; !ok {
			t.Fatalf("key %s is unexpected", string(k))
		}
		ks = append(ks, k)
	}
	ensure.DeepEqual(t, len(ks), count)
}

func TestDBKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	var count = 10000
	var keys = map[string][]byte{}
	prefix := []byte("key-0000")
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%06d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		db.Put(k, v)
		if bytes.HasPrefix(k, prefix) {
			keys[string(k)] = k
		}
	}

	var ks [][]byte
	for _, k := range db.KeysWithPrefix(prefix) {
		ensure.True(t, bytes.HasPrefix(k, prefix))
		if _, ok := keys[string(k)]; !ok {
			t.Fatalf("key %s is unexpected", string(k))
		}
		ks = append(ks, k)
	}
	ensure.DeepEqual(t, len(ks), len(keys))
}

func TestDBPersistent(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	var count = 100 // TODO the test may fail in case the count is greate than 10000
	var kvs = map[string][]byte{}
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		ensure.Nil(t, db.Put(k, v))
		kvs[string(k)] = v
	}
	db.Close()

	db, err = NewRocksDB(dbpath, &storage.Options{})
	ensure.Nil(t, err)
	defer db.Close()

	for k, v := range kvs {
		value, err := db.Get([]byte(k))
		ensure.Nil(t, err)
		ensure.True(t,
			bytes.Equal(v, value),
			"key "+k+" = "+string(value)+", != expected "+string(v),
		)
	}
}

func dbPutsFunc(t *testing.T, count int, db storage.Storage, prefix string) {
	for i := 0; i < count; i++ {
		k := fmt.Sprintf("%s-key-%05d", prefix, i)
		v := fmt.Sprintf("%s-value-%05d", prefix, i)
		err := db.Put([]byte(k), []byte(v))
		ensure.Nil(t, err)
		// t.Logf("%s: Put %s=%s", prefix, k, v)
	}
}

func dbGetsFunc(t *testing.T, count int, db storage.Storage, prefix string) {
	for i := 0; i < count; i++ {
		k := fmt.Sprintf("%s-key-%05d", prefix, i)
		v, err := db.Get([]byte(k))
		ensure.Nil(t, err)
		ensure.DeepEqual(t, v, []byte(fmt.Sprintf("%s-value-%05d", prefix, i)))
	}
}

func TestParallelPuts(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	var wg sync.WaitGroup
	const count = 100
	for i := 0; i < count; i++ {
		prefix := fmt.Sprintf("pre%05d", i)
		wg.Add(1)
		go func(p string) {
			dbPutsFunc(t, 1000, db, p)
			go func() {
				dbGetsFunc(t, 1000, db, p)
				wg.Done()
			}()
		}(prefix)
	}
	wg.Wait()
}

func TestDBTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	var kk = []byte("kkk")
	var vv = []byte("vvv")
	db.Put(kk, vv)

	var count = 10
	tx, err := db.NewTransaction()
	ensure.Nil(t, err)

	var kvs = map[string][]byte{}
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("k-%d", i))
		v := []byte(fmt.Sprintf("v-%d", i))
		ensure.Nil(t, tx.Put(k, v))
		kvs[string(k)] = v
	}

	val, err := tx.Get(kk)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, val, vv)

	for i := 0; i < count; i += 3 {
		k := []byte(fmt.Sprintf("k-%d", i))
		v := []byte(fmt.Sprintf("v3-%d", i))
		ensure.Nil(t, tx.Put(k, v))
		kvs[string(k)] = v
	}

	exists, err := tx.Has(kk)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, exists, true)

	for i := 0; i < count; i += 5 {
		k := []byte(fmt.Sprintf("k-%d", i))
		ensure.Nil(t, tx.Del(k))
		delete(kvs, string(k))
	}

	keys := tx.Keys()
	ensure.DeepEqual(t, keys, [][]byte{kk})

	for i := 0; i < count; i += 3 {
		k := []byte(fmt.Sprintf("k-%d", i))
		_, err := tx.Get(k)
		ensure.Nil(t, err)
	}
	ensure.Nil(t, tx.Commit())

	for k, v := range kvs {
		value, err := db.Get([]byte(k))
		ensure.Nil(t, err)
		ensure.DeepEqual(t, value, v)
	}
}

func TestDBMulTransactions(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	tx, err := db.NewTransaction()
	ensure.Nil(t, err)
	ensure.NotNil(t, tx)
	defer tx.Discard()

	tx2, err := db.NewTransaction()
	ensure.DeepEqual(t, err, storage.ErrTransactionExists)
	ensure.Nil(t, tx2)
}

func TestDBTransactionsClose(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	tx, err := db.NewTransaction()
	ensure.Nil(t, err)
	ensure.NotNil(t, tx)

	c := make(chan struct{})
	go func() {
		db.Close()
		c <- struct{}{}
	}()

	t1 := time.NewTimer(1 * time.Second)
	defer t1.Stop()

	select {
	case <-t1.C:
	case <-c:
		t.Error("not timeout...")
	}

	t2 := time.NewTimer(5 * time.Second)
	defer t2.Stop()

	select {
	case <-t2.C:
		t.Error("timeout...")
	case <-c:
	}
	tx.Discard()
}

func TestDBSyncTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	const (
		kk = "kkk"
		vv = "vvv"
	)
	db.Put([]byte(kk), []byte(vv))

	c := make(chan [][]byte)
	go func(c chan<- [][]byte) {
		var tx, err = db.NewTransaction()
		defer tx.Discard()
		ensure.Nil(t, err)
		for i := 0; i < 20; i++ {
			k := []byte(fmt.Sprintf("k-%d", i))
			v := []byte(fmt.Sprintf("v-%d", i))
			ensure.Nil(t, tx.Put(k, v))
			c <- [][]byte{k, v}
		}
		ensure.Nil(t, tx.Commit())

		close(c)
	}(c)

	var keys [][]byte
	var values [][]byte
	for k := range c {
		keys = append(keys, k[0])
		values = append(values, k[1])
		v, _ := db.Get([]byte(kk))
		ensure.DeepEqual(t, []byte(vv), v)

		v2, err := db.Get(k[0])
		ensure.Nil(t, err)
		ensure.DeepEqual(t, len(v2), 0)
	}

	for i, k := range keys {
		v, err := db.Get(k)
		ensure.Nil(t, err)
		ensure.DeepEqual(t, v, values[i])
	}
}

func TestDBTransactionKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	var count = 10000
	var keys = map[string][]byte{}
	var prefix = []byte("key-0000")
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%06d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		db.Put(k, v)
		if bytes.HasPrefix(k, prefix) {
			keys[string(k)] = k
		}
	}

	tx, err := db.NewTransaction()
	ensure.Nil(t, err)
	ensure.NotNil(t, tx)
	defer tx.Discard()

	var ks [][]byte
	for _, k := range tx.KeysWithPrefix(prefix) {
		ensure.True(t, bytes.HasPrefix(k, prefix))
		_, ok := keys[string(k)]
		ensure.True(t, ok)
		ks = append(ks, k)
	}
	ensure.DeepEqual(t, len(ks), len(keys))
}

func TestDBBatchAndTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	const (
		kk = "kkk"
		vv = "vvv"
	)
	db.Put([]byte(kk), []byte(vv))

	c := make(chan [][]byte)
	go func(c chan<- [][]byte) {
		var tx, err = db.NewTransaction()
		defer tx.Discard()
		ensure.Nil(t, err)
		for i := 0; i < 20; i++ {
			k := []byte(fmt.Sprintf("k-%d", i))
			v := []byte(fmt.Sprintf("v-%d", i))
			ensure.Nil(t, tx.Put(k, v))
			c <- [][]byte{k, v}
		}
		ensure.Nil(t, tx.Commit())

		close(c)
	}(c)

	var batch = db.NewBatch()
	var keys [][]byte
	var values [][]byte
	for k := range c {
		keys = append(keys, k[0])
		v := append(k[1], 0x00, 0x01, 0x02)
		values = append(values, v)
		batch.Put(k[0], v)
	}
	ensure.Nil(t, batch.Write())

	for i, k := range keys {
		v, err := db.Get(k)
		ensure.Nil(t, err)
		ensure.DeepEqual(t, v, values[i])
	}
}

func TestDBTransactionsClosed(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	tx, err := db.NewTransaction()
	ensure.Nil(t, err)
	ensure.NotNil(t, tx)
	tx.Discard()

	ensure.DeepEqual(t, tx.Put([]byte{0x00}, []byte{0x00}), storage.ErrTransactionClosed)
	_, err = tx.Get([]byte{0x00})
	ensure.DeepEqual(t, err, storage.ErrTransactionClosed)
	_, err = tx.Has([]byte{0x00})
	ensure.DeepEqual(t, err, storage.ErrTransactionClosed)
	keys := tx.Keys()
	ensure.DeepEqual(t, keys, [][]byte{})
	ensure.DeepEqual(t, tx.Commit(), storage.ErrTransactionClosed)
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
