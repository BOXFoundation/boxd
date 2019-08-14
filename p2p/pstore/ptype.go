// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

import (
	"encoding/gob"
	"fmt"

	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/storage/key"
	pool "github.com/libp2p/go-buffer-pool"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Metadata is stored under the following db key pattern:
// /peers/type/<peer type>/<b32 peer id no padding>
var ptBase = key.NewKey("/peers/type")

var tb = &typeBook{}

// PeerType is used to distinguish peer types in the p2p network.
type PeerType uint8

const (
	// UnknownPeer means I dont know what kind of node I am.
	UnknownPeer PeerType = iota
	// ServerPeer provides RPC services. It is recommended to configure it as a seed node.
	ServerPeer PeerType = iota
	// MinerPeer acts as the mining node.
	MinerPeer
	// CandidatePeer identifies the MinerPeer of the next dynasty.
	CandidatePeer
	// LayfolkPeer identifies a common full node.
	LayfolkPeer
)

// Mask coverts PeerType to uint8. Each bit refers to a PeerType.
func (pt PeerType) Mask() uint8 {
	return 1 << (pt - 1)
}

type typeBook struct {
	store storage.Table
}

func (tb *typeBook) setStore(s storage.Table) {
	tb.store = s
}

// PutType puts the type of peer into db.
func PutType(p peer.ID, key uint32, val int64) error {
	k := ptBase.ChildString(fmt.Sprintf("%x", key)).ChildString(p.Pretty())
	var buf pool.Buffer
	if err := gob.NewEncoder(&buf).Encode(&val); err != nil {
		return err
	}
	return tb.store.Put(k.Bytes(), buf.Bytes())
}

// ListPeerIDByType list peer ids by peer type.
func ListPeerIDByType(pt PeerType) ([]peer.ID, error) {
	peerIDs := []peer.ID{}
	for _, k := range tb.store.KeysWithPrefix(ptBase.ChildString(fmt.Sprintf("%x", pt.Mask())).Bytes()) {
		pk := key.NewKeyFromBytes(k)
		pid, err := peer.IDB58Decode(pk.Parent().BaseName())
		if err != nil {
			return nil, err
		}
		logger.Errorf("PID: %s", pid.Pretty())
		peerIDs = append(peerIDs, pid)
	}
	return peerIDs, nil
}