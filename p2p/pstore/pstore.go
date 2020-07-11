// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

import (
	"bytes"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	log "github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/storage/key"
	"github.com/BOXFoundation/boxd/util"
	"github.com/jbenet/goprocess"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
)

var logger = log.NewLogger("p2p/pstore")

// DefaultTableName is the default table name for peer storage
const DefaultTableName = "peer"

// NewDefaultAddrBook creates a default addrbook
func NewDefaultAddrBook(proc goprocess.Process, s storage.Storage, bus eventbus.Bus) (peerstore.AddrBook, error) {
	t, err := s.Table(DefaultTableName)
	if err != nil {
		return nil, err
	}
	return NewAddrBook(proc, t, bus, 1024), nil
}

// NewDefaultPeerstoreWithAddrBook creates a default peerstore for P2P
func NewDefaultPeerstoreWithAddrBook(proc goprocess.Process, s storage.Storage, ab peerstore.AddrBook) (peerstore.Peerstore, error) {
	t, err := s.Table(DefaultTableName)
	if err != nil {
		return nil, err
	}

	kb, err := NewKeyBook(proc, t)
	if err != nil {
		return nil, err
	}

	md, err := NewPeerMetadata(proc, t)
	if err != nil {
		return nil, err
	}
	tb.setStore(t)

	return peerstore.NewPeerstore(kb, ab, md), nil
}

func uniquePeerIDs(store storage.Table, prefix []byte, parse func(key.Key) string) (peer.IDSlice, error) {
	// txn, err := store.NewTransaction()
	// if err != nil {
	// 	return nil, err
	// }
	// defer txn.Discard()

	idset := make(map[peer.ID]struct{})
	// get all peer addrs in database
	for _, k := range store.KeysWithPrefix(prefix) {
		pk := key.NewKeyFromBytes(k)
		pid, err := peer.IDB58Decode(parse(pk))
		if err != nil {
			return nil, err
		}
		idset[pid] = struct{}{}
	}

	pids := make([]peer.ID, len(idset))
	i := 0
	for k := range idset {
		pids[i] = k
		i++
	}
	return pids, nil
}

// MarshalPeerAddr writes peer addr and type to bytes.
func MarshalPeerAddr(peertype uint8, addr []byte) (data []byte, err error) {
	var buf bytes.Buffer
	if err := util.WriteUint8(&buf, peertype); err != nil {
		return nil, err
	}
	if err := util.WriteBytes(&buf, addr); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalPeerAddr return peer type and addr from bytes.
func UnmarshalPeerAddr(data []byte) (peertype uint8, addr []byte, err error) {
	buf := bytes.NewBuffer(data)
	if peertype, err = util.ReadUint8(buf); err != nil {
		return
	}
	addr = make([]byte, buf.Len())
	if err = util.ReadBytes(buf, addr); err != nil {
		return
	}
	return
}
