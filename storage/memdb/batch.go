// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package memdb

import (
	"sync"

	storage "github.com/BOXFoundation/boxd/storage"
)

type mbatch struct {
	*memorydb

	bsm    sync.Mutex
	prefix string
	ops    []*bop
}

var _ storage.Batch = (*mbatch)(nil)

type op uint

const (
	opPut op = iota
	opDel
)

type bop struct {
	o op
	k []byte
	v []byte
}

func (b *mbatch) realkey(key []byte) []byte {
	var k = make([]byte, len(b.prefix)+len(key))
	copy(k, []byte(b.prefix))
	copy(k[len(b.prefix):], key)

	return k
}

// put the value to entry associate with the key
func (b *mbatch) Put(key, value []byte) {
	b.bsm.Lock()
	defer b.bsm.Unlock()

	b.ops = append(b.ops, &bop{
		o: opPut,
		k: key,
		v: value,
	})
}

// delete the entry associate with the key in the Storage
func (b *mbatch) Del(key []byte) {
	b.bsm.Lock()
	defer b.bsm.Unlock()

	b.ops = append(b.ops, &bop{
		o: opDel,
		k: key,
		v: nil,
	})
}

// remove all the enqueued put/delete
func (b *mbatch) Clear() {
	b.bsm.Lock()
	defer b.bsm.Unlock()

	b.ops = make([]*bop, 0)
}

// returns the number of updates in the batch
func (b *mbatch) Count() int {
	b.bsm.Lock()
	defer b.bsm.Unlock()

	return len(b.ops)
}

// atomic writes all enqueued put/delete
func (b *mbatch) Write() error {
	b.bsm.Lock()
	defer b.bsm.Unlock()

	b.sm.Lock()
	defer b.sm.Unlock()

	for _, o := range b.ops {
		k := b.realkey(o.k)
		switch o.o {
		case opPut:
			b.db[string(k)] = o.v
		case opDel:
			delete(b.db, string(k))
		}
	}

	return nil
}

// close the batch, it must be called to close the batch
func (b *mbatch) Close() {
	b.Clear()
}
