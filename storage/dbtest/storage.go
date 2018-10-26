// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dbtest

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	storage "github.com/BOXFoundation/boxd/storage"
	"github.com/facebookgo/ensure"
)

// StorageSyncTransaction is a dbtest helper method
func StorageSyncTransaction(t *testing.T, s storage.Table) {
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

// StorageBatchAndTrans is a dbtest helper method
func StorageBatchAndTrans(t *testing.T, s storage.Table) {
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

// StorageTransClosed is a dbtest helper method
func StorageTransClosed(t *testing.T, s storage.Table) {
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

// StorageTransKeysWithPrefixRand is a dbtest helper method
func StorageTransKeysWithPrefixRand(t *testing.T, s storage.Table) {
	var kmap = make(map[string]interface{})
	prefix := []byte("key-1")
	for i := 0; i < 10000; i++ {
		n := rand.Intn(10000)
		k := []byte(fmt.Sprintf("key-%04d", n))
		v := []byte(fmt.Sprintf("value-%d", i))
		s.Put(k, v)
		if bytes.HasPrefix(k, prefix) {
			kmap[string(k)] = nil
		}
	}
	var keys [][]byte
	for k := range kmap {
		keys = append(keys, []byte(k))
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

// StorageTransKeysWithPrefix is a dbtest helper method
func StorageTransKeysWithPrefix(t *testing.T, s storage.Table) {
	var keys [][]byte
	prefix := []byte("key-0001")
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

// StorageDBCloseForTransOpen is a dbtest helper method
func StorageDBCloseForTransOpen(t *testing.T, s storage.Table, db storage.Storage) {
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

// StorageMultiTrans is a dbtest helper method
func StorageMultiTrans(t *testing.T, s storage.Storage) {
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

// StorageMultiTransTable is a dbtest helper method
func StorageMultiTransTable(t *testing.T, s storage.Table) {
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

// StorageTransOps is a dbtest helper method
func StorageTransOps(t *testing.T, s storage.Table) {
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

// StorageFillData is a dbtest helper method
func StorageFillData(t *testing.T, s storage.Table, count int) func(storage.Table) {
	return StorageGenDataWithVerification(t, s, "test", count)
}

// StorageGenDataWithVerification is a dbtest helper method
func StorageGenDataWithVerification(t *testing.T, s storage.Table, name string, count int) func(storage.Table) {
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

// StoragePrefixKeys is a dbtest helper method
func StoragePrefixKeys(t *testing.T, s storage.Operations, count int) func(*testing.T, storage.Operations) {
	var keys [][]byte
	prefix := []byte("key-0001")
	for i := 0; i < count; i++ {
		k := []byte(fmt.Sprintf("key-%06d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		s.Put(k, v)
		if bytes.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}

	return func(t *testing.T, s storage.Operations) {
		var keyset [][]byte
		for _, k := range s.KeysWithPrefix(prefix) {
			ensure.True(t, bytes.HasPrefix(k, prefix))
			keyset = append(keyset, k)
		}

		ensureBytesEqual(t, keyset, keys)
	}
}

// StoragePrefixKeysRand is a dbtest helper method
func StoragePrefixKeysRand(t *testing.T, s storage.Operations) func(*testing.T, storage.Operations) {
	var kmap = make(map[string]interface{})
	prefix := []byte("key-1")
	for i := 0; i < 10000; i++ {
		n := rand.Intn(10000)
		k := []byte(fmt.Sprintf("key-%04d", n))
		v := []byte(fmt.Sprintf("value-%d", i))
		s.Put(k, v)
		if bytes.HasPrefix(k, prefix) {
			kmap[string(k)] = nil
		}
	}
	var keys [][]byte
	for k := range kmap {
		keys = append(keys, []byte(k))
	}

	return func(t *testing.T, s storage.Operations) {
		var keyset [][]byte
		for _, k := range s.KeysWithPrefix(prefix) {
			ensure.True(t, bytes.HasPrefix(k, prefix))
			keyset = append(keyset, k)
		}

		ensureBytesEqual(t, keyset, keys)
	}
}

// StorageKeys is a dbtest helper method
func StorageKeys(t *testing.T, s storage.Operations) func(*testing.T, storage.Operations) {
	var keys [][]byte
	for i := 0; i < 10000; i++ {
		k := []byte(fmt.Sprintf("key-%d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		ensure.Nil(t, s.Put(k, v))
		keys = append(keys, k)
	}

	return func(t *testing.T, s storage.Operations) {
		var ks [][]byte
		for _, k := range s.Keys() {
			ks = append(ks, k)
		}
		ensureBytesEqual(t, keys, ks)
	}
}

// StorageBatch is a dbtest helper method
func StorageBatch(t *testing.T, s storage.Table) {
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

// StorageDel is a dbtest helper method
func StorageDel(t *testing.T, s storage.Table) {
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

// StoragePutGetDelTest is a dbtest helper method
func StoragePutGetDelTest(s storage.Table, k, v []byte) func(*testing.T) {
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

// StorageIterKeysWithPrefix is a dbtest helper method
func StorageIterKeysWithPrefix(t *testing.T, s storage.Operations) func(*testing.T, storage.Operations) {
	var kmap = make(map[string]interface{})
	prefix := []byte("key-1")
	for i := 0; i < 10000; i++ {
		n := rand.Intn(10000)
		k := []byte(fmt.Sprintf("key-%04d", n))
		v := []byte(fmt.Sprintf("value-%d", i))
		s.Put(k, v)
		if bytes.HasPrefix(k, prefix) {
			kmap[string(k)] = nil
		}
	}
	var keys [][]byte
	for k := range kmap {
		keys = append(keys, []byte(k))
	}

	return func(t *testing.T, s storage.Operations) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var keyset [][]byte
		for k := range s.IterKeysWithPrefix(ctx, prefix) {
			ensure.True(t, bytes.HasPrefix(k, prefix))
			keyset = append(keyset, k)
		}

		ensureBytesEqual(t, keyset, keys)
	}
}

// StorageIterKeys is a dbtest helper method
func StorageIterKeys(t *testing.T, s storage.Operations) func(*testing.T, storage.Operations) {
	var kmap = make(map[string]interface{})
	for i := 0; i < 10000; i++ {
		n := rand.Intn(10000)
		k := []byte(fmt.Sprintf("key-%04d", n))
		v := []byte(fmt.Sprintf("value-%d", i))
		s.Put(k, v)
		kmap[string(k)] = nil
	}
	var keys [][]byte
	for k := range kmap {
		keys = append(keys, []byte(k))
	}

	return func(t *testing.T, s storage.Operations) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var keyset [][]byte
		for k := range s.IterKeys(ctx) {
			keyset = append(keyset, k)
		}

		ensureBytesEqual(t, keyset, keys)
	}
}

// StorageIterKeysCancel is a dbtest helper method
func StorageIterKeysCancel(t *testing.T, s storage.Operations) func(*testing.T, storage.Operations) {
	for i := 0; i < 1000; i++ {
		k := []byte(fmt.Sprintf("key-%04d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		s.Put(k, v)
	}

	return func(t *testing.T, s storage.Operations) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var i = 0
		for range s.IterKeys(ctx) {
			i++
			if i == 100 {
				cancel()
			}
		}

		ensure.True(t, i < 1000)
	}
}

// StorageIterKeysWithPrefixCancel is a dbtest helper method
func StorageIterKeysWithPrefixCancel(t *testing.T, s storage.Operations) func(*testing.T, storage.Operations) {
	for i := 0; i < 1000; i++ {
		k := []byte(fmt.Sprintf("key-%04d", i))
		v := []byte(fmt.Sprintf("value-%d", i))
		s.Put(k, v)
	}

	return func(t *testing.T, s storage.Operations) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var prefix = []byte("key-00")
		var i = 0
		for range s.IterKeysWithPrefix(ctx, prefix) {
			i++
			if i == 50 {
				cancel()
			}
		}

		ensure.True(t, i < 100)
	}
}
