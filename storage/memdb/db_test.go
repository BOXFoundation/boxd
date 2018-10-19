// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package memdb

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	storage "github.com/BOXFoundation/boxd/storage"
	"github.com/facebookgo/ensure"
)

func TestDBCreateClose(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)

	err = db.Close()
	ensure.Nil(t, err)
}

var testFunc = func(db storage.Storage, k, v []byte) func(*testing.T) {
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
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

	t.Run("put1", testFunc(db, []byte("tk1"), []byte("tv1")))
	t.Run("put2", testFunc(db, []byte("tk2"), []byte("tv2")))
	t.Run("put3", testFunc(db, []byte("tk3"), []byte("tv3")))
	t.Run("put4", testFunc(db, []byte("tk4"), []byte("tv4")))
}

func TestDBDel(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
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
	var db, err = NewMemoryDB("", nil)
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
	var db, err = NewMemoryDB("", nil)
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

func TestDBKeysWithPrefix(t *testing.T) {
	var db, err = NewMemoryDB("", nil)
	ensure.Nil(t, err)
	defer db.Close()

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

	var ks [][]byte
	for _, k := range db.KeysWithPrefix(prefix) {
		ensure.True(t, bytes.HasPrefix(k, prefix))
		_, ok := keys[string(k)]
		ensure.True(t, ok)
		ks = append(ks, k)
	}
	ensure.DeepEqual(t, len(ks), len(keys))
}

func TestDBTransaction(t *testing.T) {
	db, _ := NewMemoryDB("", nil)
	defer db.Close()

	var kk = []byte("kkk")
	var vv = []byte("vvv")
	db.Put(kk, vv)

	var count = 10
	tx, err := db.NewTransaction()
	ensure.Nil(t, err)
	defer tx.Discard()

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

	var kk = "kkk"
	var vv = "vvv"

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
