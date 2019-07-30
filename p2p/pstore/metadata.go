// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

import (
	"bytes"
	"encoding/gob"

	"github.com/BOXFoundation/boxd/storage"
	key "github.com/BOXFoundation/boxd/storage/key"
	"github.com/jbenet/goprocess"
	pool "github.com/libp2p/go-buffer-pool"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/libp2p/go-libp2p-peerstore"
)

// Metadata is stored under the following db key pattern:
// /peers/metadata/<b32 peer id no padding>/<key>
var pmBase = key.NewKey("/peers/metadata")

// PTypeSuf is the suffix of peer type metadata
var PTypeSuf = "PeerType"

type peerMetadata struct {
	store storage.Table
}

var _ peerstore.PeerMetadata = (*peerMetadata)(nil)

func init() {
	// Gob registers basic types by default.
	//
	// Register complex types used by the peerstore itself.
	gob.Register(make(map[string]struct{}))
}

// NewPeerMetadata creates a metadata store backed by a persistent db. It uses gob for serialisation.
//
// See `init()` to learn which types are registered by default. Modules wishing to store
// values of other types will need to `gob.Register()` them explicitly, or else callers
// will receive runtime errors.
func NewPeerMetadata(_ goprocess.Process, store storage.Table) (peerstore.PeerMetadata, error) {
	return &peerMetadata{store}, nil
}

func (pm *peerMetadata) Get(p peer.ID, key string) (interface{}, error) {
	return getMetadata(pm.store, p, key)
}

func (pm *peerMetadata) Put(p peer.ID, key string, val interface{}) error {
	return putMetadata(pm.store, p, key, val)
}

// getMetadata puts the metadata of peer into db.
func getMetadata(db storage.Table, p peer.ID, key string) (interface{}, error) {
	k := pmBase.ChildString(p.Pretty()).ChildString(key)
	value, err := db.Get(k.Bytes())
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return nil, peerstore.ErrNotFound
	}

	var res interface{}
	if err := gob.NewDecoder(bytes.NewReader(value)).Decode(&res); err != nil {
		return nil, err
	}
	return res, nil
}

// putMetadata puts the metadata of peer into db.
func putMetadata(db storage.Table, p peer.ID, key string, val interface{}) error {
	k := pmBase.ChildString(p.Pretty()).ChildString(key)
	var buf pool.Buffer
	if err := gob.NewEncoder(&buf).Encode(&val); err != nil {
		return err
	}
	return db.Put(k.Bytes(), buf.Bytes())
}
