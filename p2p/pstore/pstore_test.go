// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	r "math/rand"
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/libp2p/go-libp2p-peerstore"

	"github.com/multiformats/go-multiaddr"

	"github.com/BOXFoundation/Quicksilver/storage"
	"github.com/BOXFoundation/Quicksilver/storage/memdb"
	"github.com/BOXFoundation/Quicksilver/storage/rocksdb"
	"github.com/facebookgo/ensure"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
)

func randomid() peer.ID {
	key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
	id, _ := peer.IDFromPublicKey(key.GetPublic())
	return id
}

var addrs = []string{
	"/ip4/192.168.10.1/tcp/8080",
	"/ip4/192.168.10.1/tcp/8081",
	"/ip4/192.168.10.1/tcp/8082",
	"/ip4/192.168.10.1/tcp/8083",
	"/ip4/192.168.10.1/tcp/8084",
}

func randomPath(t *testing.T) string {
	dir, err := ioutil.TempDir("", fmt.Sprintf("%d", r.Int()))
	ensure.Nil(t, err)
	return dir
}

func TestRocksdbPeerstoreAddAddr(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, err = rocksdb.NewRocksDB(dbpath, &o)
	ensure.Nil(t, err)

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	ps, err := NewDefaultPeerstore(ctx, db)
	ensure.Nil(t, err)

	for _, addr := range addrs {
		var pid = randomid()
		var maddr, _ = multiaddr.NewMultiaddr(addr)
		ps.AddAddr(pid, maddr, peerstore.RecentlyConnectedAddrTTL)
		pinfo := ps.PeerInfo(pid)
		ensure.DeepEqual(t, 1, len(pinfo.Addrs))
		ensure.DeepEqual(t, maddr, pinfo.Addrs[0])
	}
}

func TestPeerstoreAddAddr(t *testing.T) {
	var db, _ = memdb.NewMemoryDB("", nil)
	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	var ps, err = NewDefaultPeerstore(ctx, db)
	ensure.Nil(t, err)

	for _, addr := range addrs {
		var pid = randomid()
		var maddr, _ = multiaddr.NewMultiaddr(addr)
		ps.AddAddr(pid, maddr, peerstore.RecentlyConnectedAddrTTL)
		ensure.DeepEqual(t, maddr, ps.PeerInfo(pid).Addrs[0])
	}
}

func peerstoreAddAddrs(ps peerstore.Peerstore) func(*testing.T) {
	return func(t *testing.T) {
		var pid = randomid()
		var maddrs []multiaddr.Multiaddr
		for _, addr := range addrs {
			var maddr, _ = multiaddr.NewMultiaddr(addr)
			maddrs = append(maddrs, maddr)
		}

		var ss sort.StringSlice = addrs
		ss.Sort()

		ps.AddAddrs(pid, maddrs, peerstore.RecentlyConnectedAddrTTL)
		var ss2 sort.StringSlice
		for _, ma := range ps.PeerInfo(pid).Addrs {
			ss2 = append(ss2, ma.String())
		}
		ss2.Sort()

		ensure.DeepEqual(t, ss, ss2)
	}
}

func TestPeerstoreAddAddrs(t *testing.T) {
	var db, _ = memdb.NewMemoryDB("", nil)
	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	var ps, err = NewDefaultPeerstore(ctx, db)
	ensure.Nil(t, err)
	peerstoreAddAddrs(ps)(t)
}

func TestMultiplePeerstoreAddAddrs(t *testing.T) {
	var db, _ = memdb.NewMemoryDB("", nil)
	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	var ps, err = NewDefaultPeerstore(ctx, db)
	ensure.Nil(t, err)

	for i := 0; i < 32; i++ {
		t.Run("t"+strconv.FormatInt(int64(i), 16), peerstoreAddAddrs(ps))
	}
}
