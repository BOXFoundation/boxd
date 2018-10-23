// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	storage "github.com/BOXFoundation/boxd/storage"
	"github.com/facebookgo/ensure"
)

func storageSyncTransaction(t *testing.T, s storage.Table) {
	var (
		kk = "kkk"
		vv = "vvv"
	)

	s.Put([]byte(kk), []byte(vv))

	c := make(chan [][]byte)
	next := make(chan struct{})
	go func(c chan<- [][]byte) {
		tx, _ := s.NewTransaction()
		defer tx.Discard()

		for i := 0; i < 20; i++ {
			k := []byte(fmt.Sprintf("k-%d", i))
			v := []byte(fmt.Sprintf("v-%d", i))
			tx.Put(k, v)
			c <- [][]byte{k, v}
		}
		close(c)

		<-next
		ensure.Nil(t, tx.Commit())
		<-next
	}(c)

	var keys [][]byte
	var values [][]byte
	for k := range c {
		keys = append(keys, k[0])
		values = append(values, k[1])
		v, _ := s.Get([]byte(kk))
		ensure.DeepEqual(t, []byte(vv), v)

		v2, err := s.Get(k[0])
		ensure.Nil(t, err)
		ensure.DeepEqual(t, len(v2), 0)
	}
	next <- struct{}{}
	next <- struct{}{}

	for i, k := range keys {
		v, err := s.Get(k)
		ensure.Nil(t, err)
		ensure.DeepEqual(t, v, values[i])
	}
}

func storageBatchAndTrans(t *testing.T, s storage.Table) {
	kk := []byte("kkk")
	vv := []byte("vvv")
	s.Put(kk, vv)

	c := make(chan [][]byte)
	go func(c chan<- [][]byte) {
		tx, _ := s.NewTransaction()
		defer tx.Discard()
		defer close(c)

		for i := 0; i < 20; i++ {
			k := []byte(fmt.Sprintf("k-%d", i))
			v := []byte(fmt.Sprintf("v-%d", i))
			ensure.Nil(t, tx.Put(k, v))
			c <- [][]byte{k, v}
		}
		tx.Commit()
	}(c)

	var batch = s.NewBatch()
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
		v, err := s.Get(k)
		ensure.Nil(t, err)
		ensure.DeepEqual(t, v, values[i])
	}
}

func storageTransClosed(t *testing.T, s storage.Table) {
	tx, err := s.NewTransaction()
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

func storageTransKeysWithPrefix(t *testing.T, s storage.Table) {
	var keys [][]byte
	prefix := []byte("key-0000")
	for i := 0; i < 10000; i++ {
		k := []byte(fmt.Sprintf("key-%06d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		s.Put(k, v)
		if bytes.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}

	tx, err := s.NewTransaction()
	ensure.Nil(t, err)
	ensure.NotNil(t, tx)
	defer tx.Discard()

	var keyset [][]byte
	for _, k := range tx.KeysWithPrefix(prefix) {
		ensure.True(t, bytes.HasPrefix(k, prefix))
		keyset = append(keyset, k)
	}

	ensureBytesEqual(t, keyset, keys)
}

func ensureBytesEqual(t *testing.T, ks1 [][]byte, ks2 [][]byte) {
	ensure.DeepEqual(t, len(ks1), len(ks2))

	for _, k := range ks1 {
		exists := false
		for _, l := range ks2 {
			if bytes.Equal(k, l) {
				exists = true
				break
			}
		}
		ensure.True(t, exists)
	}
}

func storageDBCloseForTransOpen(t *testing.T, s storage.Table, db storage.Storage) {
	tx, _ := s.NewTransaction()
	ensure.NotNil(t, tx)
	defer tx.Discard()

	c := make(chan struct{})
	go func() {
		db.Close()
		c <- struct{}{}
	}()

	t1 := time.NewTimer(100 * time.Millisecond)
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
}

func storageMultiTrans(t *testing.T, s storage.Storage) {
	tx, err := s.NewTransaction()
	ensure.Nil(t, err)
	ensure.NotNil(t, tx)
	defer tx.Discard()

	out := make(chan int)
	defer close(out)

	done := func(i int) {
		out <- i
	}

	txci := func(name string) {
		var table storage.Table
		if len(name) == 0 {
			table = s
		} else {
			table, _ = s.Table(name)
		}
		tx, err := table.NewTransaction()
		if err != nil {
			done(0)
			return
		}
		defer tx.Discard()

		for i := uint(0); i <= uint(1024); i++ {
			tx.Put([]byte{0x00, uint8(i % 256), uint8(i % 13)}, []byte{0x00, uint8(i), 0x10})
			tx.Get([]byte{uint8(i)})
		}

		if err := tx.Commit(); err == nil {
			done(1)
		} else {
			done(0)
		}
	}

	txdiscard := func(name string) {
		var table storage.Table
		if len(name) == 0 {
			table = s
		} else {
			table, _ = s.Table(name)
		}
		tx, err := table.NewTransaction()
		if err != nil {
			done(0)
			return
		}
		defer tx.Discard()

		for i := uint(0); i <= uint(1024); i++ {
			tx.Put([]byte{0x00, uint8(i % 256), uint8(i % 13)}, []byte{0x00, uint8(i), 0x10})
			tx.Get([]byte{uint8(i)})
		}
		done(1)
	}

	goroutines := 0
	for i := 0; i < 256; i++ {
		goroutines += 2
		go txci(fmt.Sprintf("%04d", i%24))
		go txdiscard(fmt.Sprintf("%04d", i%25))
	}

	timer := time.NewTimer(time.Second * 5)
	defer timer.Stop()

	returns := 0
	succ := 0
Loop:
	for {
		select {
		case result := <-out:
			returns++
			succ += result
			if returns == goroutines {
				break Loop
			}
		case <-timer.C:
			break Loop
		}
	}
	ensure.DeepEqual(t, returns, goroutines)
}

func storageMultiTransTable(t *testing.T, s storage.Table) {
	out := make(chan int)
	defer close(out)

	done := func(i int) {
		out <- i
	}

	txci := func() {
		tx, err := s.NewTransaction()
		if err != nil {
			done(0)
			return
		}
		defer tx.Discard()

		for i := uint(0); i <= uint(1024); i++ {
			tx.Put([]byte{0x00, uint8(i % 256), uint8(i % 13)}, []byte{0x00, uint8(i), 0x10})
		}

		if err := tx.Commit(); err != nil {
			done(0)
			return
		}

		tx2, err := s.NewTransaction()
		if err != nil {
			done(0)
			return
		}
		defer tx2.Discard()

		for i := uint(0); i <= uint(1024); i++ {
			value, err := tx2.Get([]byte{0x00, uint8(i % 256), uint8(i % 13)})
			if err != nil {
				done(0)
				return
			}
			if !bytes.Equal(value, []byte{0x00, uint8(i), 0x10}) {
				done(0)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			done(0)
			return
		}
	}

	txdiscard := func() {
		tx, err := s.NewTransaction()
		if err != nil {
			done(0)
			return
		}
		defer tx.Discard()

		for i := uint(0); i <= uint(1024); i++ {
			tx.Put([]byte{0x00, uint8(i % 256), uint8(i % 13)}, []byte{0x00, uint8(i), 0x10})
			tx.Get([]byte{uint8(i)})
		}
		done(1)
	}

	goroutines := 0
	for i := 0; i < 256; i++ {
		goroutines += 2
		go txci()
		go txdiscard()
	}

	timer := time.NewTimer(time.Second * 5)
	defer timer.Stop()

	returns := 0
	succ := 0
Loop:
	for {
		select {
		case result := <-out:
			returns++
			succ += result
			if returns == goroutines {
				break Loop
			}
		case <-timer.C:
			break Loop
		}
	}
	ensure.DeepEqual(t, returns, goroutines)
}

func storageTransOps(t *testing.T, s storage.Table) {
	var kk = []byte("kkk")
	var vv = []byte("vvv")
	s.Put(kk, vv)

	var count = 10
	tx, err := s.NewTransaction()
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
		value, err := s.Get([]byte(k))
		ensure.Nil(t, err)
		ensure.DeepEqual(t, value, v)
	}
}

func storageFillData(t *testing.T, s storage.Table, count int) func(storage.Table) {
	return storageGenDataWithVerification(t, s, "test", count)
}

func storageGenDataWithVerification(t *testing.T, s storage.Table, name string, count int) func(storage.Table) {
	var kvs = map[string][]byte{}
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%s-%d", name, i))
		v := []byte(fmt.Sprintf("value-%s-%d", name, i))
		ensure.Nil(t, s.Put(k, v))
		kvs[string(k)] = v
	}

	verify := func(s storage.Table) {
		for k, v := range kvs {
			value, err := s.Get([]byte(k))
			ensure.Nil(t, err)
			ensure.True(t,
				bytes.Equal(v, value),
				"key "+k+" = "+string(value)+", != expected "+string(v),
			)
		}

	}
	return verify
}

func storagePrefixKeys(t *testing.T, s storage.Table, count int) {
	var keys [][]byte
	prefix := []byte("key-0000")
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%06d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		s.Put(k, v)
		if bytes.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}

	var keyset [][]byte
	for _, k := range s.KeysWithPrefix(prefix) {
		ensure.True(t, bytes.HasPrefix(k, prefix))
		keyset = append(keyset, k)
	}

	ensureBytesEqual(t, keyset, keys)
}

func storageKeys(t *testing.T, s storage.Table, count int) {
	var keys [][]byte
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		ensure.Nil(t, s.Put(k, v))
		keys = append(keys, k)
	}

	var ks [][]byte
	for _, k := range s.Keys() {
		ks = append(ks, k)
	}
	ensureBytesEqual(t, keys, ks)
}

func storageBatch(t *testing.T, s storage.Table) {
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

	var batch = s.NewBatch()
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
		exist, err := s.Has([]byte(k))
		ensure.Nil(t, err)
		ensure.False(t, exist)
	}

	for k, v := range kvs {
		value, err := s.Get([]byte(k))
		ensure.Nil(t, err)
		ensure.True(t, bytes.Equal(v, value))
	}
}

func storageDel(t *testing.T, s storage.Table) {
	var keys = [][]byte{}
	for i := 0; i < 100; i++ {
		k := []byte(fmt.Sprintf("key-%d", i))
		v := []byte(fmt.Sprintf("value-%d", i))

		ensure.Nil(t, s.Put(k, v))

		has, err := s.Has(k)
		ensure.Nil(t, err)
		ensure.True(t, has)

		keys = append(keys, k)
	}

	var wg sync.WaitGroup
	for _, k := range keys {
		wg.Add(1)
		go func(k []byte) {
			defer wg.Done()
			ensure.Nil(t, s.Del(k))

			has, err := s.Has(k)
			ensure.Nil(t, err)
			ensure.False(t, has)
		}(k)
	}
	wg.Wait()
}

func storagePutGetDelTest(s storage.Table, k, v []byte) func(*testing.T) {
	return func(t *testing.T) {
		s.Put(k, v)
		has, err := s.Has(k)
		ensure.Nil(t, err)
		ensure.True(t, has)

		value, err := s.Get(k)
		ensure.Nil(t, err)
		ensure.True(t, bytes.Equal(value, v))

		ensure.Nil(t, s.Del(k))
		has, err = s.Has(k)
		ensure.Nil(t, err)
		ensure.False(t, has)
	}
}
