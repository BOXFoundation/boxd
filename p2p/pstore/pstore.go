// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

import (
	"bytes"
	"context"

	storage "github.com/BOXFoundation/Quicksilver/storage"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/libp2p/go-libp2p-peerstore"
)

// DefaultTableName is the default table name for peer storage
const DefaultTableName = "peer"

// NewDefaultPeerstore creates a default peerstore for P2P
func NewDefaultPeerstore(ctx context.Context, s storage.Storage) (peerstore.Peerstore, error) {
	return NewPeerstore(ctx, s, DefaultTableName)
}

// NewPeerstore creates a new peerstore for P2P from a named table
func NewPeerstore(ctx context.Context, s storage.Storage, name string) (peerstore.Peerstore, error) {
	t, err := s.Table(name)
	if err != nil {
		return nil, err
	}
	return newPeerstoreTable(ctx, t)
}

// newPeerstoreTable creates a new peerstore for P2P from a table
func newPeerstoreTable(ctx context.Context, t storage.Table) (peerstore.Peerstore, error) {
	return peerstore.NewPeerstoreDatastore(ctx, NewDatastoreFromTable(t))
}

// NewDatastoreFromTable creates a datastore from a storage table
func NewDatastoreFromTable(t storage.Table) datastore.Batching {
	return &pstore{t: t}
}

type pstore struct {
	t storage.Table
}

// Put stores the object `value` named by `key`.
//
// The generalized Datastore interface does not impose a value type,
// allowing various datastore middleware implementations (which do not
// handle the values directly) to be composed together.
//
// Ultimately, the lowest-level datastore will need to do some value checking
// or risk getting incorrect values. It may also be useful to expose a more
// type-safe interface to your application, and do the checking up-front.
func (s *pstore) Put(key datastore.Key, value []byte) error {
	return s.t.Put(key.Bytes(), value)
}

// Get retrieves the object `value` named by `key`.
// Get will return ErrNotFound if the key is not mapped to a value.
func (s *pstore) Get(key datastore.Key) (value []byte, err error) {
	return s.t.Get(key.Bytes())
}

// Has returns whether the `key` is mapped to a `value`.
// In some contexts, it may be much cheaper only to check for existence of
// a value, rather than retrieving the value itself. (e.g. HTTP HEAD).
// The default implementation is found in `GetBackedHas`.
func (s *pstore) Has(key datastore.Key) (exists bool, err error) {
	return s.t.Has(key.Bytes())
}

// Delete removes the value for given `key`.
func (s *pstore) Delete(key datastore.Key) error {
	return s.t.Del(key.Bytes())
}

// Query searches the datastore and returns a query result. This function
// may return before the query actually runs. To wait for the query:
//
//   result, _ := ds.Query(q)
//
//   // use the channel interface; result may come in at different times
//   for entry := range result.Next() { ... }
//
//   // or wait for the query to be completely done
//   entries, _ := result.Rest()
//   for entry := range entries { ... }
//
func (s *pstore) Query(q query.Query) (query.Results, error) {
	// get all keys
	var keys [][]byte
	if len(q.Prefix) > 0 {
		for _, k := range s.t.Keys() {
			if bytes.HasPrefix(k, []byte(q.Prefix)) {
				keys = append(keys, k)
			}
		}
	} else {
		keys = s.t.Keys()
	}

	var entries []query.Entry
	// set entries per Keyonly is true or false
	if q.KeysOnly {
		for _, k := range keys {
			entries = append(entries, query.Entry{Key: string(k)})
		}
	} else {
		for _, k := range keys {
			v, err := s.t.Get(k)
			if err != nil {
				return nil, err
			}
			entries = append(entries, query.Entry{Key: string(k), Value: v})
		}
	}

	// filters
	for _, f := range q.Filters {
		var es []query.Entry
		for _, e := range entries {
			if f.Filter(e) {
				es = append(es, e)
			}
		}
		entries = es
	}

	// orders
	for _, o := range q.Orders {
		o.Sort(entries)
	}

	// offset
	var offset = q.Offset
	if offset > len(entries) {
		offset = len(entries)
	}
	entries = entries[offset:]

	// limit
	if q.Limit > 0 && q.Limit < len(entries) {
		entries = entries[:q.Limit]
	}

	return query.ResultsWithEntries(q, entries), nil
}

// Batch provides batch data operations.
func (s *pstore) Batch() (datastore.Batch, error) {
	return &dsbatch{bt: s.t.NewBatch()}, nil
}

type dsbatch struct {
	bt storage.Batch
}

func (b *dsbatch) Put(key datastore.Key, val []byte) error {
	b.bt.Put(key.Bytes(), val)
	return nil
}

func (b *dsbatch) Delete(key datastore.Key) error {
	b.bt.Del(key.Bytes())
	return nil
}

func (b *dsbatch) Commit() error {
	return b.bt.Write()
}
