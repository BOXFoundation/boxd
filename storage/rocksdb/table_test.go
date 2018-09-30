// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"testing"

	storage "github.com/BOXFoundation/Quicksilver/storage"
	"github.com/facebookgo/ensure"
)

func TestTableCreateClose(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	ensure.Nil(t, table.Put([]byte("!&@%hdg"), []byte("djksfusm, dl")))
	ensure.Nil(t, db.DropTable("t1"))
}

func TestTableDel(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)
	defer db.Close()

	var wg sync.WaitGroup

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	var keys = []string{}
	for i := 0; i < 10000; i++ {
		k := fmt.Sprintf("key-%d", i)
		v := fmt.Sprintf("value-%d", i)

		ensure.Nil(t, table.Put([]byte(k), []byte(v)))
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
func TestTableBatch(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	var count = 100
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

	var batch = table.NewBatch()
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
	ensure.Nil(t, batch.Write())

	for _, k := range delkeys {
		delete(kvs, k)
		exist, err := table.Has([]byte(k))
		ensure.Nil(t, err)
		ensure.False(t, exist)
	}

	var wg sync.WaitGroup
	for k, v := range kvs {
		wg.Add(1)

		go func(k, v []byte) {
			defer wg.Done()

			value, err := table.Get(k)
			ensure.Nil(t, err)
			ensure.True(t, bytes.Equal(v, value))
		}([]byte(k), v)
	}
	wg.Wait()
}

func TestTableBatchs(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprint("t", i), TestTableBatch)
	}
}

func TestTableKeys(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	db, err := NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)
	defer db.Close()

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	var count = 10000
	var keys = map[string][]byte{}
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		table.Put(k, v)
		keys[string(k)] = k
	}

	var ks [][]byte
	for _, k := range table.Keys() {
		_, ok := keys[string(k)]
		ensure.True(t, ok)
		ks = append(ks, k)
	}
	ensure.DeepEqual(t, len(ks), count)
}

func TestTablePersistent(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)

	table, err := db.Table("t")
	ensure.Nil(t, err)

	var count = 100 // TODO the test may fail in case the count is greate than 10000
	var kvs = map[string][]byte{}
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		ensure.Nil(t, table.Put(k, v))
		kvs[string(k)] = v
	}
	db.Close()

	db, err = NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)
	defer db.Close()

	table, err = db.Table("t")
	ensure.Nil(t, err)

	for k, v := range kvs {
		value, err := table.Get([]byte(k))
		ensure.Nil(t, err)
		ensure.True(t,
			bytes.Equal(v, value),
			"key "+k+" = "+string(value)+", != expected "+string(v),
		)
	}
}
