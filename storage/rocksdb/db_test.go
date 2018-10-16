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
