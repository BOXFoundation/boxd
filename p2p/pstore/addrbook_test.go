// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/storage/rocksdb"
	"github.com/facebookgo/ensure"
	goprocessctx "github.com/jbenet/goprocess/context"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
)

func getDatabase() (string, storage.Storage, error) {
	dbpath, err := ioutil.TempDir("", fmt.Sprintf("%d", rand.Int()))
	if err != nil {
		return "", nil, err
	}

	db, err := rocksdb.NewRocksDB(dbpath, &storage.Options{})
	if err != nil {
		return dbpath, nil, err
	}
	return dbpath, db, nil
}

func releaseDatabase(dbpath string, db storage.Storage) {
	db.Close()
	os.RemoveAll(dbpath)
}

type tab struct {
	ab     *addrBook
	db     storage.Storage
	dbpath string
	ctx    context.Context
	cancel context.CancelFunc
}

func newAb() (*tab, error) {
	dbpath, db, err := getDatabase()
	if err != nil {
		return nil, err
	}

	t, err := db.Table("peer")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := goprocessctx.WithContext(ctx)
	ab := NewAddrBook(p, t, eventbus.New(), 1024)

	return &tab{
		ab:     ab.(*addrBook),
		db:     db,
		dbpath: dbpath,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func releaseAb(b *tab) {
	b.cancel()
	releaseDatabase(b.dbpath, b.db)
}

func genAddr(ctx context.Context) <-chan ma.Multiaddr {
	var addresses = make(map[string]struct{})
	var out = make(chan ma.Multiaddr)
	go func() {
		for {
			s := fmt.Sprintf("/ip4/%d.%d.%d.%d/tcp/%d", rand.Intn(250)+1, rand.Intn(250), rand.Intn(250), rand.Intn(250), rand.Intn(2000)+1024)
			if addr, err := ma.NewMultiaddr(s); err == nil {
				if _, ok := addresses[addr.String()]; !ok {
					addresses[addr.String()] = struct{}{}
					select {
					case out <- addr:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return out
}

func generagePeer(ctx context.Context, count int) map[peer.ID][]ma.Multiaddr {
	var pids = make(map[peer.ID][]ma.Multiaddr)
	addresses := genAddr(ctx)
	for i := 0; i < count; i++ {
		pri, _, _ := crypto.GenerateEd25519Key(crand.Reader)
		pid, _ := peer.IDFromPrivateKey(pri)

		var addrs []ma.Multiaddr
		var addrsCount = rand.Intn(4) + 1
		for j := 0; j < addrsCount; j++ {
			addrs = append(addrs, <-addresses)
		}
		pids[pid] = addrs
	}

	return pids
}

func genDataForAddrBook(tab *tab, count int) map[peer.ID][]ma.Multiaddr {
	pa := generagePeer(tab.ctx, count)
	for pid, addrs := range pa {
		tab.ab.AddAddrs(pid, addrs, time.Minute*10)
	}
	return pa
}

func ensureAddrsEqual(t *testing.T, addr1 []ma.Multiaddr, addr2 []ma.Multiaddr) {
	ensure.DeepEqual(t, len(addr1), len(addr2))
	for _, a := range addr1 {
		exists := false
		for _, b := range addr2 {
			if a.Equal(b) {
				exists = true
				break
			}
		}
		ensure.True(t, exists)
	}
}

func ensurePeersEqual(t *testing.T, p1 peer.IDSlice, p2 peer.IDSlice) {
	ensure.DeepEqual(t, len(p1), len(p2))
	for _, a := range p1 {
		exists := false
		for _, b := range p2 {
			if a == b {
				exists = true
				break
			}
		}
		ensure.True(t, exists)
	}
}

func containAddr(addrs []ma.Multiaddr, addr ma.Multiaddr) bool {
	exists := false
	for _, a := range addrs {
		if a.Equal(addr) {
			exists = true
			break
		}
	}
	return exists
}

func TestAddrBookCreate(t *testing.T) {
	ab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(ab)

	ensure.NotNil(t, ab.ab)
}

func TestAddrBook(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	ensure.NotNil(t, tab.ab)
}

func TestAddrBookAddAddrs(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := generagePeer(tab.ctx, 100)
	for pid, addrs := range pa {
		tab.ab.AddAddrs(pid, addrs, time.Minute*10)
	}

	for pid, addrs := range pa {
		mas := tab.ab.Addrs(pid)

		ensureAddrsEqual(t, mas, addrs)
	}
}

func TestAddrBookAddrsCache(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := genDataForAddrBook(tab, 100)

	for pid := range pa {
		var _, ok = tab.ab.cache.Peek(pid)
		ensure.False(t, ok)
		mas1 := tab.ab.Addrs(pid)

		_, ok = tab.ab.cache.Peek(pid)
		ensure.True(t, ok)
		mas2 := tab.ab.Addrs(pid)

		ensureAddrsEqual(t, mas1, mas2)
	}
}

func TestAddrBookUpdateCache(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := genDataForAddrBook(tab, 100)

	for pid := range pa {
		tab.ab.UpdateAddrs(pid, time.Minute*10, time.Minute)

		var _, ok = tab.ab.cache.Peek(pid)
		ensure.False(t, ok)
	}
}

func TestAddrBookUpdate(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := genDataForAddrBook(tab, 100)

	var start = time.Now() // start time
	for pid := range pa {
		tab.ab.UpdateAddrs(pid, time.Minute*10, time.Millisecond)
	}

	time.Sleep(2 * time.Millisecond) // wait until expired
	for pid := range pa {
		addr1 := tab.ab.Addrs(pid)

		e, ok := tab.ab.cache.Get(pid)
		ensure.True(t, ok)
		entry := e.(cacheEntry)
		// start < exp < now
		ensure.True(t, entry.expiration.After(start))
		ensure.True(t, entry.expiration.Before(time.Now()))

		addr2 := tab.ab.Addrs(pid)
		ensureAddrsEqual(t, addr1, addr2)
	}
}

func TestAddrBookClear(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := genDataForAddrBook(tab, 10)

	for pid := range pa {
		addr1 := tab.ab.Addrs(pid)
		ensure.True(t, len(addr1) > 0)

		// clear addrs
		tab.ab.ClearAddrs(pid)
		_, ok := tab.ab.cache.Get(pid)
		ensure.False(t, ok) // cache is clear

		addr2 := tab.ab.Addrs(pid)
		ensure.True(t, len(addr2) == 0)
	}
}

func TestAddrBookSetZeroTTL(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := genDataForAddrBook(tab, 10)

	for pid := range pa {
		addrs := tab.ab.Addrs(pid)
		ensure.True(t, len(addrs) > 0)

		for _, addr := range addrs {
			// set 0 ttl to delete the addr
			tab.ab.SetAddr(pid, addr, time.Duration(0))
			// get addrs again
			addrs2 := tab.ab.Addrs(pid)
			ensure.False(t, containAddr(addrs2, addr))
		}
	}
}

func TestAddrBookSetAddr(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := genDataForAddrBook(tab, 2)

	var peers []peer.ID
	for p := range pa {
		peers = append(peers, p)
	}

	tab.ab.ClearAddrs(peers[1]) // clear addrs of peers[1]
	addrs := append(pa[peers[0]], pa[peers[1]]...)
	for _, addr := range addrs {
		tab.ab.SetAddr(peers[0], addr, time.Minute)
	}
	actualaddrs := tab.ab.Addrs(peers[0])
	ensureAddrsEqual(t, actualaddrs, addrs)
}

func TestAddrBookSetAddrs(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := genDataForAddrBook(tab, 2)

	var peers []peer.ID
	for p := range pa {
		peers = append(peers, p)
	}

	tab.ab.ClearAddrs(peers[1]) // clear addrs of peers[1]
	addrs := append(pa[peers[0]], pa[peers[1]]...)
	tab.ab.SetAddrs(peers[0], addrs, time.Minute)

	actualaddrs := tab.ab.Addrs(peers[0])
	ensureAddrsEqual(t, actualaddrs, addrs)
}

func TestAddrBookAddAddr(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := genDataForAddrBook(tab, 2)

	var peers []peer.ID
	for p := range pa {
		peers = append(peers, p)
	}

	tab.ab.ClearAddrs(peers[1]) // clear addrs of peers[1]
	addrs := append(pa[peers[0]], pa[peers[1]]...)
	for _, addr := range addrs {
		tab.ab.AddAddr(peers[0], addr, time.Minute) // extend ttl
	}

	actualaddrs := tab.ab.Addrs(peers[0])
	ensureAddrsEqual(t, actualaddrs, addrs)

	// compare cache entry
	e, ok := tab.ab.cache.Get(peers[0])
	ensure.True(t, ok)
	entry := e.(cacheEntry)
	ensure.True(t, entry.expiration.Sub(time.Now()) < time.Minute)
	ensure.True(t, entry.expiration.Sub(time.Now()) > time.Second*58)
}

func TestAddrBookAddAddrsTTL(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := generagePeer(tab.ctx, 2)
	var peers []peer.ID
	for p := range pa {
		peers = append(peers, p)
	}

	tab.ab.AddAddrs(peers[0], pa[peers[0]], time.Minute)
	tab.ab.AddAddrs(peers[0], pa[peers[1]], time.Minute*10)
	tab.ab.AddAddrs(peers[0], pa[peers[0]], time.Minute*10)

	actualaddrs := tab.ab.Addrs(peers[0])
	ensureAddrsEqual(t, actualaddrs, append(pa[peers[0]], pa[peers[1]]...))

	// compare cache entry
	e, ok := tab.ab.cache.Get(peers[0])
	ensure.True(t, ok)
	entry := e.(cacheEntry)

	// the ttl should be same as before
	ensure.True(t, entry.expiration.Sub(time.Now()) < time.Minute*10)
	ensure.True(t, entry.expiration.Sub(time.Now()) > time.Second*(60*9+58))
}

func TestAddrBookAddrsStream(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := generagePeer(tab.ctx, 3)
	var peers []peer.ID
	for p := range pa {
		peers = append(peers, p)
	}

	var pid = peers[0]
	var addresses = pa[pid]
	var count = len(addresses) + 20

	var mutext sync.Mutex
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := genAddr(ctx)

	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer wg.Done()

		var i int
	ForLoop:
		for {
			time.Sleep(time.Millisecond)
			select {
			case <-ctx.Done():
				break ForLoop
			case addr := <-c:
				if i%3 == 0 {
					mutext.Lock()
					addresses = append(addresses, addr)
					mutext.Unlock()
				}
				tab.ab.AddAddr(peers[i%3], addr, time.Minute)
			}
			i++
		}
	}()

	var i = 0
	for addr := range tab.ab.AddrStream(ctx, pid) {
		mutext.Lock()
		ensure.True(t, containAddr(addresses, addr))
		mutext.Unlock()
		i++
		if i == count {
			break
		}
	}
	cancel()
	wg.Wait()
}

func TestAddrBookPeers(t *testing.T) {
	tab, err := newAb()
	ensure.Nil(t, err)
	defer releaseAb(tab)

	pa := genDataForAddrBook(tab, 100)

	var peers peer.IDSlice
	for pid := range pa {
		peers = append(peers, pid)
	}

	pids := tab.ab.PeersWithAddrs()

	ensurePeersEqual(t, peers, pids)
}
