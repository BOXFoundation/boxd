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
	"time"

	storage "github.com/BOXFoundation/boxd/storage"
	"github.com/facebookgo/ensure"
)

func TestTableCreateClose(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	ensure.Nil(t, table.Put([]byte("!&@%hdg"), []byte("djksfusm, dl")))
	ensure.Nil(t, db.DropTable("t1"))
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

func TestTableDel(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	var wg sync.WaitGroup

	table, err := db.Table("t1")
	ensure.Nil(t, err)

	var keys = [][]byte{}
	for i := 0; i < 100; i++ {
		k := []byte(fmt.Sprintf("key-%d", i))
		v := []byte(fmt.Sprintf("value-%d", i))

		ensure.Nil(t, table.Put(k, v))

		has, err := db.Has(k)
		ensure.Nil(t, err)
		ensure.False(t, has)

		keys = append(keys, k)
		wg.Add(1)
	}

	for _, k := range keys {
		go func(k []byte) {
			defer wg.Done()
			ensure.Nil(t, db.Del(k))

			has, err := db.Has(k)
			ensure.Nil(t, err)
			ensure.False(t, has)
		}(k)
	}
	wg.Wait()
}

func TestTableBatch(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

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
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

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

func TestTableKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	var count = 10000
	var keys = map[string][]byte{}
	var prefix = []byte("key-0000")
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%06d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		table.Put(k, v)
		if bytes.HasPrefix(k, prefix) {
			keys[string(k)] = k
		}
	}

	var ks [][]byte
	for _, k := range table.KeysWithPrefix(prefix) {
		ensure.True(t, bytes.HasPrefix(k, prefix))
		_, ok := keys[string(k)]
		ensure.True(t, ok)
		ks = append(ks, k)
	}
	ensure.DeepEqual(t, len(ks), len(keys))
}

func TestTablePersistent(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

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

	db, err = NewRocksDB(dbpath, &storage.Options{})
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

func TestTableTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	table, _ := db.Table("t1")

	var kk = []byte("kkk")
	var vv = []byte("vvv")
	table.Put(kk, vv)

	var count = 10
	tx, err := table.NewTransaction()
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
		value, err := table.Get([]byte(k))
		ensure.Nil(t, err)
		ensure.DeepEqual(t, value, v)
	}
}

func TestTableMulTransactions(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	t1, _ := db.Table("t1")

	t1tx, err := t1.NewTransaction()
	ensure.Nil(t, err)
	ensure.NotNil(t, t1tx)
	t1tx.Put([]byte{0x00, 0x01}, []byte{0x00})
	defer t1tx.Discard()

	// t1tx2, err := t1.NewTransaction()
	// ensure.DeepEqual(t, err, storage.ErrTransactionExists)
	// ensure.Nil(t, t1tx2)

	t2, _ := db.Table("t2")
	t2tx1, err := t2.NewTransaction()
	ensure.Nil(t, err)
	ensure.NotNil(t, t2tx1)
	t2tx1.Put([]byte{0x00, 0x01}, []byte{0x00})
	defer t2tx1.Discard()

	tx, err := db.NewTransaction()
	ensure.Nil(t, err)
	ensure.NotNil(t, tx)
	tx.Put([]byte{0x00, 0x01}, []byte{0x00})
	ensure.Nil(t, tx.Commit())
	defer tx.Discard()

	ensure.Nil(t, t2tx1.Commit())
}

func TestTableTransactionsClose(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	table, _ := db.Table("t1")

	tx, err := table.NewTransaction()
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

func TestTableSyncTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	table, _ := db.Table("t1")

	var kk = "kkk"
	var vv = "vvv"

	table.Put([]byte(kk), []byte(vv))

	c := make(chan [][]byte)
	go func(c chan<- [][]byte) {
		var tx, err = table.NewTransaction()
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
		v, _ := table.Get([]byte(kk))
		ensure.DeepEqual(t, []byte(vv), v)

		v2, err := table.Get(k[0])
		ensure.Nil(t, err)
		ensure.DeepEqual(t, len(v2), 0)
	}

	for i, k := range keys {
		v, err := table.Get(k)
		ensure.Nil(t, err)
		ensure.DeepEqual(t, v, values[i])
	}
}

func TestTableBatchAndTransaction(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)
	defer db.Close()

	table, _ := db.Table("t1")

	kk := "kkk"
	vv := "vvv"
	table.Put([]byte(kk), []byte(vv))

	c := make(chan [][]byte)
	go func(c chan<- [][]byte) {
		var tx, err = table.NewTransaction()
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

	var batch = table.NewBatch()
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
		v, err := table.Get(k)
		ensure.Nil(t, err)
		ensure.DeepEqual(t, v, values[i])
	}
}

func TestTableTransactionsClosed(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer os.RemoveAll(dbpath)

	table, _ := db.Table("t1")

	tx, err := table.NewTransaction()
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

func TestTableTransactionKeysWithPrefix(t *testing.T) {
	dbpath, db, err := getDatabase()
	ensure.Nil(t, err)
	defer releaseDatabase(dbpath, db)

	table, err := db.Table("tx")
	ensure.Nil(t, err)

	var count = 10000
	var keys = map[string][]byte{}
	var prefix = []byte("key-0000")
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%06d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		table.Put(k, v)
		if bytes.HasPrefix(k, prefix) {
			keys[string(k)] = k
		}
	}

	tx, err := table.NewTransaction()
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
