// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rocksdb

import (
	"github.com/tecbot/gorocksdb"
)

type rbatch struct {
	rocksdb *gorocksdb.DB
	cf      *gorocksdb.ColumnFamilyHandle
	wb      *gorocksdb.WriteBatch

	writeOptions *gorocksdb.WriteOptions
}

// put the value to entry associate with the key
func (b *rbatch) Put(key, value []byte) {
	if b.cf != nil {
		b.wb.PutCF(b.cf, key, value)
	} else {
		b.wb.Put(key, value)
	}
}

// delete the entry associate with the key in the Storage
func (b *rbatch) Del(key []byte) {
	if b.cf != nil {
		b.wb.DeleteCF(b.cf, key)
	} else {
		b.wb.Delete(key)
	}
}

// remove all the enqueued put/delete
func (b *rbatch) Clear() {
	b.wb.Clear()
}

// returns the number of updates in the batch
func (b *rbatch) Count() int {
	return b.wb.Count()
}

// atomic writes all enqueued put/delete
func (b *rbatch) Write() error {
	return b.rocksdb.Write(b.writeOptions, b.wb)
}

// close the batch, it must be called to close the batch
func (b *rbatch) Close() {
	b.wb.Destroy()
}
