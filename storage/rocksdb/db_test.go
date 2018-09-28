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

	storage "github.com/BOXFoundation/Quicksilver/storage"
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

func TestDBCreateClose(t *testing.T) {
	var dbpath = randomPath(t)
	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	err = db.Close()
	ensure.Nil(t, err)
}

var testFunc = func(t *testing.T, db storage.Storage, k, v []byte) func(*testing.T) {
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
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)
	defer db.Close()

	t.Run("put1", testFunc(t, db, []byte("tk1"), []byte("tv1")))
	t.Run("put2", testFunc(t, db, []byte("tk2"), []byte("tv2")))
	t.Run("put3", testFunc(t, db, []byte("tk3"), []byte("tv3")))
	t.Run("put4", testFunc(t, db, []byte("tk4"), []byte("tv4")))
}

func TestDBDel(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)
	defer db.Close()

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
			ensure.Nil(t, db.Del(k))
			wg.Done()
		}([]byte(k))
	}
	wg.Wait()
}

func TestDBBatch(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)
	defer db.Close()

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
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)
	defer db.Close()

	var count = 10000
	var keys = map[string][]byte{}
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		db.Put(k, v)
		keys[string(k)] = k
	}

	for _, k := range db.Keys() {
		if _, ok := keys[string(k)]; !ok {
			t.Fatalf("key %s is unexpected", string(k))
		}
	}
}

func TestDBPersistent(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)

	var count = 100 // TODO the test may fail in case the count is greate than 10000
	var kvs = map[string][]byte{}
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		ensure.Nil(t, db.Put(k, v))
		kvs[string(k)] = v
	}
	db.Close()

	db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)

	for k, v := range kvs {
		value, err := db.Get([]byte(k))
		ensure.Nil(t, err)
		ensure.True(t,
			bytes.Equal(v, value),
			"key "+k+" = "+string(value)+", != expected "+string(v),
		)
	}
}

func dbPutGet(t *testing.T, db storage.Storage, k []byte, v []byte) func(*testing.T) {
	return func(t *testing.T) {
		ensure.Nil(t, db.Put(k, v))
		value, err := db.Get(k)
		ensure.Nil(t, err)
		ensure.DeepEqual(t, len(v), len(value))
		ensure.DeepEqual(t, v, value)
	}
}

const chars = "1234567890abcdefhijklmnopqrstuvwxyzABCDEFHIJKLMNOPQRSTUVWXYZ"

func BenchmarkPutData(b *testing.B) {
	var dbpath = randomPathB(b)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(b, err)
	defer db.Close()

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
	var dbpath = randomPathB(b)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(b, err)
	defer db.Close()

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
	var dbpath = randomPathB(b)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(b, err)
	defer db.Close()

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
