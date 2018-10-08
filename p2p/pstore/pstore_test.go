// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	r "math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	storage "github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/BOXFoundation/boxd/storage/rocksdb"
	"github.com/facebookgo/ensure"
	datastore "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	multiaddr "github.com/multiformats/go-multiaddr"
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

func TestRocksdbPeerstoreAddAddrs(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, _ = rocksdb.NewRocksDB(dbpath, &o)

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	var ps, err = NewDefaultPeerstore(ctx, db)
	ensure.Nil(t, err)
	peerstoreAddAddrs(ps)(t)
}

func TestRocksdbMultiplePeerstoreAddAddrs(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, _ = rocksdb.NewRocksDB(dbpath, &o)

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	var ps, err = NewDefaultPeerstore(ctx, db)
	ensure.Nil(t, err)

	for i := 0; i < 32; i++ {
		t.Run("t"+strconv.FormatInt(int64(i), 16), peerstoreAddAddrs(ps))
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

func Test_pstore_Delete(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, _ = rocksdb.NewRocksDB(dbpath, &o)
	defer db.Close()

	type fields struct {
		t storage.Table
	}
	type args struct {
		key datastore.Key
	}

	table := func(name string) storage.Table {
		if t, err := db.Table(name); err == nil {
			return t
		}
		return nil
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "t1",
			fields:  fields{t: table("t1")},
			args:    args{key: datastore.NewKey("k1")},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &pstore{
				t: tt.fields.t,
			}
			s.Put(tt.args.key, []byte("value"))
			if err := s.Delete(tt.args.key); (err != nil) != tt.wantErr {
				t.Errorf("pstore.Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
			has, err := s.Has(tt.args.key)
			ensure.Nil(t, err)
			ensure.False(t, has)
		})
	}
}

func Test_pstore_GetPutHas(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, _ = rocksdb.NewRocksDB(dbpath, &o)
	defer db.Close()

	ps := func(name string) *pstore {
		table, _ := db.Table(name)
		return &pstore{t: table}
	}
	ps1 := ps("p1")
	k1 := datastore.NewKey("k1")
	v1 := []byte("value")
	ensure.Nil(t, ps1.Put(k1, v1))
	has, err := ps1.Has(k1)
	ensure.Nil(t, err)
	ensure.True(t, has)
	value, err := ps1.Get(k1)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, v1, value)

	ps2 := ps("p2")
	k2 := datastore.NewKey("k2")
	has, err = ps2.Has(k2)
	ensure.Nil(t, err)
	ensure.False(t, has)
	value, err = ps2.Get(k2)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, 0, len(value))
}

type filter struct{}

func (f filter) Filter(e query.Entry) bool {
	return bytes.Contains(e.Value, []byte("v-02"))
}

func setupDatestore(t *testing.T, ps datastore.Batching) {
	for i := 0; i < 100; i++ {
		k1 := datastore.NewKey(fmt.Sprintf("k1-%03d", i))
		v1 := []byte(fmt.Sprintf("v-%03d", i))
		ensure.Nil(t, ps.Put(k1, v1))

		k2 := datastore.NewKey(fmt.Sprintf("k2-%03d", i))
		v2 := []byte(fmt.Sprintf("v-%03d", i))
		ensure.Nil(t, ps.Put(k2, v2))
	}
}

func Test_pstore_QueryFilter(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, _ = rocksdb.NewRocksDB(dbpath, &o)
	defer db.Close()

	newps := func(name string) *pstore {
		table, _ := db.Table(name)
		return &pstore{t: table}
	}
	ps := newps("ps")
	setupDatestore(t, ps)

	var q = query.Query{
		Prefix:  "/k1",
		Filters: []query.Filter{filter{}},
	}
	results, err := ps.Query(q)
	ensure.Nil(t, err)
	var count = 0
	for r := range results.Next() {
		count++
		ensure.DeepEqual(t, r.Value[:4], []byte("v-02"))
	}
	ensure.DeepEqual(t, count, 10)

	q = query.Query{
		Prefix:  "/k2",
		Filters: []query.Filter{filter{}},
	}
	results, err = ps.Query(q)
	ensure.Nil(t, err)
	count = 0
	for r := range results.Next() {
		count++
		ensure.DeepEqual(t, r.Value[:4], []byte("v-02"))
	}
	ensure.DeepEqual(t, count, 10)

	q = query.Query{
		Filters: []query.Filter{filter{}},
	}
	results, err = ps.Query(q)
	ensure.Nil(t, err)
	count = 0
	for r := range results.Next() {
		count++
		ensure.DeepEqual(t, r.Value[:4], []byte("v-02"))
	}
	ensure.DeepEqual(t, count, 20)
}

func Test_pstore_LimitOffset(t *testing.T) {
	var dbpath = randomPath(t)
	defer os.RemoveAll(dbpath)

	var o storage.Options
	var db, _ = rocksdb.NewRocksDB(dbpath, &o)
	defer db.Close()

	newps := func(name string) *pstore {
		table, _ := db.Table(name)
		return &pstore{t: table}
	}
	ps := newps("ps")
	setupDatestore(t, ps)

	var q = query.Query{
		Prefix: "/k1",
		Limit:  20,
		Offset: 87,
	}
	results, err := ps.Query(q)
	ensure.Nil(t, err)
	var count = 0
	for r := range results.Next() {
		count++
		ensure.True(t, strings.HasPrefix(r.Key, "/k1"))
	}
	ensure.DeepEqual(t, count, 13)

	for i := range []int{3, 27, 45, 67, 87, 90, 99, 97, 100, 103} {
		q = query.Query{
			Prefix: "/k2",
			Limit:  10,
			Offset: i,
		}
		results, err = ps.Query(q)
		ensure.Nil(t, err)
		count = 0
		for r := range results.Next() {
			count++
			ensure.True(t, strings.HasPrefix(r.Key, "/k2"))
		}
		if i < 90 {
			ensure.DeepEqual(t, count, 10)
		} else if i < 100 {
			ensure.DeepEqual(t, count, 100-i)
		} else {
			ensure.DeepEqual(t, count, 0)
		}
	}
}
