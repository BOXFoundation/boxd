// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"testing"

	storage "github.com/BOXFoundation/Quicksilver/storage"
)

func randomPath() string {
	return fmt.Sprintf("/tmp/%d", rand.Int())
}

func TestDBCreateClose(t *testing.T) {
	var dbpath = randomPath()
	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	if err != nil {
		t.Fatalf("DB not created: %v", err)
	}
	defer os.RemoveAll(dbpath)

	err = db.Close()
	if err != nil {
		t.Fatalf("DB not closed: %v", err)
	}

}

var testFunc = func(t *testing.T, db storage.Storage, k, v []byte) func(*testing.T) {
	return func(t *testing.T) {
		db.Put(k, v)
		if has, err := db.Has(k); err != nil || !has {
			t.Fatalf("put a key but err=%v, has=%v", err, has)
		}

		var value []byte
		var err error
		if value, err = db.Get(k); err != nil {
			t.Fatalf("get key %s err: %v", k, err)
		}
		if !bytes.Equal(value, v) {
			t.Fatalf("%v != %v", v, value)
		}

		db.Del(k)
		if has, err := db.Has(k); err != nil || has {
			t.Fatalf("del a key but err %v, has=%v", err, has)
		}
	}
}

func TestDBPut(t *testing.T) {
	var dbpath = randomPath()
	var o storage.Options
	var db, err = NewRocksDB(dbpath, &o)
	if err != nil {
		t.Fatalf("DB not created: %v", err)
	}
	defer os.RemoveAll(dbpath)
	defer db.Close()

	t.Run("put1", testFunc(t, db, []byte("tk1"), []byte("tv1")))
	t.Run("put2", testFunc(t, db, []byte("tk2"), []byte("tv2")))
	t.Run("put3", testFunc(t, db, []byte("tk3"), []byte("tv3")))
	t.Run("put4", testFunc(t, db, []byte("tk4"), []byte("tv4")))
}
