// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/storage"
	key "github.com/BOXFoundation/boxd/storage/key"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	b58 "github.com/mr-tron/base58/base58"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
)

// Peer addresses are stored under the following db key pattern:
// /peers/addr/<b58 peer id no padding>/<hash of maddr>
var abBase = key.NewKey("/peers/addrs")
var ttlBase = key.NewKey("/ttl")

// The maximum representable value in time.Time is time.Unix(1<<63-62135596801, 999999999).
// But it's too brittle and implementation-dependent, so we prefer to use 1<<62, which is in the
// year 146138514283. We're safe.
var maxTime = time.Unix(1<<62, 0)

var defaultTTLInterval = time.Minute * 1

type cacheEntry struct {
	expiration time.Time
	addrs      []ma.Multiaddr
}

var _ peerstore.AddrBook = (*addrBook)(nil)

// addrBook is an address book backed by a storage.Table with both an
// in-memory TTL manager and an in-memory address stream manager.
type addrBook struct {
	cache    cache
	bus      eventbus.Bus
	proc     goprocess.Process
	store    storage.Table
	interval time.Duration
}

// NodeInfo contains status info about a peer, including peer id, protocol, ip
// addresses and ttl
type NodeInfo struct {
	TTL    time.Duration
	PeerID peer.ID
	Addr   []string
	Valid  bool
}

type ttlWriteMode int

const (
	ttlOverride ttlWriteMode = iota
	ttlExtend
)

func keysAndAddrs(p peer.ID, addrs []ma.Multiaddr) ([]key.Key, []ma.Multiaddr, error) {
	var (
		keys      = make([]key.Key, len(addrs))
		clean     = make([]ma.Multiaddr, len(addrs))
		parentKey = abBase.ChildString(p.Pretty())
		i         = 0
	)

	for _, addr := range addrs {
		if addr == nil {
			continue
		}

		hash, err := mh.Sum((addr).Bytes(), mh.MURMUR3, -1)
		if err != nil {
			return nil, nil, err
		}
		keys[i] = parentKey.ChildString(b58.Encode(hash))
		clean[i] = addr
		i++
	}

	return keys[:i], clean[:i], nil
}

func ttlKey(k key.Key) key.Key {
	if ttlBase.IsAncestorOf(k) {
		return k
	}
	return ttlBase.Child(k)
}

// NewAddrBook creates a new instance of AddrBook
func NewAddrBook(parent goprocess.Process, s storage.Table, bus eventbus.Bus, cacheSize int) peerstore.AddrBook {
	ab := &addrBook{
		proc:     goprocess.WithParent(parent),
		store:    s,
		bus:      bus,
		interval: defaultTTLInterval,
	}
	if cacheSize > 0 {
		ab.cache, _ = lru.NewARC(cacheSize)
	}
	ab.initBusListener()

	return ab
}

func (ab *addrBook) initBusListener() {
	ab.bus.Reply(eventbus.TopicGetAddressBook, func(out chan<- []NodeInfo) {
		var infos []NodeInfo
		peers := ab.PeersWithAddrs()
		for _, p := range peers {
			info := NodeInfo{
				TTL:    0,
				PeerID: p,
				Addr:   []string{},
			}
			addrs := ab.Addrs(p)
			for _, addr := range addrs {
				info.Addr = append(info.Addr, addr.String())
			}
			ttl, err := ab.dbGetTTL(p)
			info.Valid = err == nil
			if info.Valid {
				info.TTL = ttl
			}
			infos = append(infos, info)
		}
		out <- infos
	}, false)
}

func (ab *addrBook) Run() error {
	ab.proc.Go(func(p goprocess.Process) {
		ticker := time.NewTicker(ab.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				logger.Debug("checking expired peers......")
				if err := ab.dbRemoveExpired(); err != nil {
					logger.Errorf("failed to remove expired addr.")
				}
			case <-p.Closing():
				logger.Info("quit loop of address book")
				return
			}
		}
	})
	return nil
}

func (ab *addrBook) Stop() {
	ab.proc.Close()
}

func (ab *addrBook) Proc() goprocess.Process {
	return ab.proc
}

//////////////////////// impl peerstore.AddrBook ////////////////////////

// AddAddr will add a new address if it's not already in the AddrBook.
func (ab *addrBook) AddAddr(p peer.ID, addr ma.Multiaddr, ttl time.Duration) {
	ab.AddAddrs(p, []ma.Multiaddr{addr}, ttl)
}

// AddAddrs will add many new addresses if they're not already in the AddrBook.
func (ab *addrBook) AddAddrs(p peer.ID, addrs []ma.Multiaddr, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	ab.setAddrs(p, addrs, ttl, ttlExtend)
}

// SetAddr will add or update the TTL of an address in the AddrBook.
func (ab *addrBook) SetAddr(p peer.ID, addr ma.Multiaddr, ttl time.Duration) {
	ab.SetAddrs(p, []ma.Multiaddr{addr}, ttl)
}

// SetAddrs will add or update the TTLs of addresses in the AddrBook.
func (ab *addrBook) SetAddrs(p peer.ID, addrs []ma.Multiaddr, ttl time.Duration) {
	if ttl <= 0 {
		ab.deleteAddrs(p, addrs)
		return
	}
	ab.setAddrs(p, addrs, ttl, ttlOverride)
}

// UpdateAddrs will update any addresses for a given peer and TTL combination to
// have a new TTL.
func (ab *addrBook) UpdateAddrs(p peer.ID, oldTTL time.Duration, newTTL time.Duration) {
	if ab.cache != nil {
		ab.cache.Remove(p)
	}

	if err := ab.dbUpdateTTL(p, oldTTL, newTTL); err != nil {
		logger.Errorf("failed to update ttlsfor peer %s: %s", p.Pretty(), err)
	}
}

// Addrs returns all of the non-expired addresses for a given peer.
func (ab *addrBook) Addrs(p peer.ID) []ma.Multiaddr {
	var (
		prefix = abBase.ChildString(p.Pretty())
		err    error
	)

	// Check the cache and return the entry only if it hasn't expired; if expired, remove.
	if ab.cache != nil {
		if e, ok := ab.cache.Get(p); ok {
			entry := e.(cacheEntry)
			if entry.expiration.After(time.Now()) {
				addrs := make([]ma.Multiaddr, len(entry.addrs))
				copy(addrs, entry.addrs)
				return addrs
			}
			ab.cache.Remove(p)
		}
	}

	txn, err := ab.store.NewTransaction()
	if err != nil {
		return nil
	}
	defer txn.Discard()

	var addrs []ma.Multiaddr
	var buf []byte
	// used to get the multiaddr with the prefix of the peer
	earliestExp := maxTime
	for _, k := range txn.KeysWithPrefix(prefix.Bytes()) {
		if buf, err = txn.Get(k); err != nil {
			logger.Errorf("failed to get value of key %s: %v", string(k), err)
			return nil
		}
		addr, err := ma.NewMultiaddrBytes(buf)
		if err != nil {
			logger.Errorf("failed to unmarshal addr: %v", err)
			return nil
		}
		addrs = append(addrs, addr)

		// get ttl
		ttlKey := ttlBase.Child(key.NewKeyFromBytes(k))
		if buf, err = txn.Get(ttlKey.Bytes()); err == nil && len(buf) > 0 {
			var exp time.Time
			if err := exp.UnmarshalBinary(buf); err == nil {
				if !exp.IsZero() && exp.Before(earliestExp) {
					earliestExp = exp
				}
			}
		}
	}

	// Store a copy in the cache.
	if ab.cache != nil {
		addrsCpy := make([]ma.Multiaddr, len(addrs))
		copy(addrsCpy, addrs)
		entry := cacheEntry{addrs: addrsCpy, expiration: earliestExp}
		ab.cache.Add(p, entry)
	}

	return addrs
}

// AddrStream returns a channel on which all new addresses discovered for a
// given peer ID will be published.
func (ab *addrBook) AddrStream(ctx context.Context, p peer.ID) <-chan ma.Multiaddr {
	initial := ab.Addrs(p)
	out := make(chan ma.Multiaddr)

	go func(buffer []ma.Multiaddr) {
		defer close(out)

		// record sent addr
		sent := make(map[string]bool, len(buffer))
		for _, a := range buffer {
			sent[string(a.Bytes())] = true
		}

		// next to be sent
		var outch chan ma.Multiaddr
		var next ma.Multiaddr
		if len(buffer) > 0 {
			next = buffer[0]
			buffer = buffer[1:]
			outch = out
		}

		// chan to be sent by bus subscriber
		subch := make(chan ma.Multiaddr)
		defer close(subch)

		subf := func(pid peer.ID, addr ma.Multiaddr) {
			if pid == p {
				subch <- addr
			}
		}
		ab.bus.SubscribeAsync(eventbus.TopicP2PPeerAddr, subf, false)
		defer ab.bus.Unsubscribe(eventbus.TopicP2PPeerAddr, subf)

		for {
			select {
			case outch <- next:
				if len(buffer) > 0 {
					next = buffer[0]
					buffer = buffer[1:]
				} else {
					outch = nil
					next = nil
				}
			case naddr := <-subch:
				if sent[string(naddr.Bytes())] {
					continue
				}

				sent[string(naddr.Bytes())] = true
				if next == nil {
					next = naddr
					outch = out
				} else {
					buffer = append(buffer, naddr)
				}
			case <-ctx.Done():
				return
			}
		}

	}(initial)

	return out
}

// ClearAddrs will delete all known addresses for a peer ID.
func (ab *addrBook) ClearAddrs(p peer.ID) {
	var prefix = abBase.ChildString(p.Pretty())

	if ab.cache != nil {
		// clear addrs found in cache
		if e, ok := ab.cache.Peek(p); ok {
			ab.cache.Remove(p)
			keys, _, _ := keysAndAddrs(p, e.(cacheEntry).addrs)
			if err := ab.dbDelete(keys); err != nil {
				logger.Errorf("failed to clear addresses for peer %s: %s", p.Pretty(), err)
			}
			return
		}
	}
	if err := ab.dbDeleteIter(prefix); err != nil {
		logger.Errorf("failed to clear addresses for peer %s: %s", p.Pretty(), err)
	}
}

// Peers returns all of the peer IDs stored in the AddrBook
func (ab *addrBook) PeersWithAddrs() peer.IDSlice {
	pids, err := uniquePeerIDs(ab.store, abBase.Bytes(), func(k key.Key) string {
		return k.Parent().BaseName()
	})
	if err != nil {
		logger.Errorf("failed to get peers: %v", err)
	}
	return pids
}

////////////////////////////////////////////////////////////////////////////////

func (ab *addrBook) deleteAddrs(p peer.ID, addrs []ma.Multiaddr) error {
	// Keys and cleaned up addresses.
	keys, addrs, err := keysAndAddrs(p, addrs)
	if err != nil {
		return err
	}

	if ab.cache != nil {
		ab.cache.Remove(p)
	}
	// Attempt transactional KV deletion.
	if err = ab.dbDelete(keys); err != nil {
		logger.Errorf("failed to delete addrs of peer %s: %v", p.Pretty(), err)
		return err
	}

	return nil
}

func (ab *addrBook) setAddrs(p peer.ID, addrs []ma.Multiaddr, ttl time.Duration, mode ttlWriteMode) error {
	// Keys and cleaned up addresses.
	keys, addrs, err := keysAndAddrs(p, addrs)
	if err != nil {
		return err
	}

	if ab.cache != nil {
		ab.cache.Remove(p)
	}
	// Attempt transactional KV insertion.
	var existed []bool
	if existed, err = ab.dbInsert(keys, addrs, ttl, mode); err != nil {
		logger.Errorf("failed to avoid write conflict for peer %s : %v", p.Pretty(), err)
		return err
	}

	// Update was successful, so publish event only for new addresses.
	for i, addr := range addrs {
		if !existed[i] {
			ab.bus.Publish(eventbus.TopicP2PPeerAddr, p, addr)
		}
	}
	return nil
}

/////////////////////////////////// db operations //////////////////////////////

// dbInsert performs a transactional insert of the provided keys and values.
func (ab *addrBook) dbInsert(keys []key.Key, addrs []ma.Multiaddr, ttl time.Duration, mode ttlWriteMode) ([]bool, error) {
	var (
		err     error
		existed = make([]bool, len(keys))
		exp     = time.Now().Add(ttl)
	)

	txn, err := ab.store.NewTransaction()
	if err != nil {
		return nil, err
	}
	defer txn.Discard()

	for i, key := range keys {
		// Check if the key existed previously.
		if existed[i], err = txn.Has(key.Bytes()); err != nil {
			logger.Errorf("transaction failed and aborted while checking key existence: %s, cause: %v", key.String(), err)
			return nil, err
		}

		// update ttl
		var ttlkey = ttlKey(key)
		var buf []byte
		switch mode {
		case ttlOverride:
			if buf, err = exp.MarshalBinary(); err == nil {
				err = txn.Put(ttlkey.Bytes(), buf)
			}
		case ttlExtend:
			var curr time.Time
			if buf, err = txn.Get(ttlkey.Bytes()); err == nil && len(buf) != 0 {
				if err = curr.UnmarshalBinary(buf); err != nil {
					break
				}
			}
			if exp.After(curr) {
				if buf, err = exp.MarshalBinary(); err == nil {
					err = txn.Put(ttlkey.Bytes(), buf)
				}
			}
		}
		if err != nil {
			// mode will be printed as an int
			logger.Errorf("failed while updating the ttl for key: %s, mode: %v, cause: %v", key.String(), mode, err)
			return nil, err
		}

		// The key embeds a hash of the value, so if it existed, we can safely skip the insert
		if existed[i] {
			continue
		}

		// put addr
		if err = txn.Put(key.Bytes(), addrs[i].Bytes()); err != nil {
			logger.Errorf("transaction failed and aborted while setting key: %s, cause: %v", key.String(), err)
			return nil, err
		}
	}

	if err = txn.Commit(); err != nil {
		logger.Errorf("failed to commit transaction when setting keys, cause: %v", err)
		return nil, err
	}

	return existed, nil
}

// dbDelete transactionally deletes the provided keys.
func (ab *addrBook) dbDelete(keys []key.Key) error {
	txn, err := ab.store.NewTransaction()
	if err != nil {
		return err
	}
	defer txn.Discard()

	for _, key := range keys {
		if err = txn.Del(key.Bytes()); err != nil {
			return err
		}
		ttlkey := ttlKey(key)
		if err = txn.Del(ttlkey.Bytes()); err != nil {
			return err
		}
	}

	return txn.Commit()
}

func (ab *addrBook) dbUpdateTTL(p peer.ID, oldTTL time.Duration, newTTL time.Duration) error {
	var (
		prefix = abBase.ChildString(p.Pretty())
		exp    = time.Now().Add(newTTL)
	)

	txn, err := ab.store.NewTransaction()
	if err != nil {
		return err
	}
	defer txn.Discard()

	ttlbuf, err := exp.MarshalBinary()
	if err != nil {
		logger.Errorf("failed to marshal time %v: %v", exp, err)
		return err
	}

	for _, k := range txn.KeysWithPrefix(prefix.Bytes()) {
		ttlkey := ttlBase.Child(key.NewKeyFromBytes(k))
		if err = txn.Put(ttlkey.Bytes(), ttlbuf); err != nil {
			return err
		}
	}

	return txn.Commit()
}

func (ab *addrBook) dbGetTTL(p peer.ID) (time.Duration, error) {
	var (
		prefix = abBase.ChildString(p.Pretty())
	)
	keys := ab.store.KeysWithPrefix(prefix.Bytes())
	if len(keys) == 0 {
		return time.Duration(0), fmt.Errorf("no db record found for peer ttl")
	}
	var exps []*time.Time
	for _, k := range keys {
		ttlKey := ttlBase.Child(key.NewKeyFromBytes(k))
		buf, err := ab.store.Get(ttlKey.Bytes())
		if err != nil {
			return time.Duration(0), err
		}
		exp := new(time.Time)
		if err := exp.UnmarshalBinary(buf); err != nil {
			return time.Duration(0), err
		}
		exps = append(exps, exp)
	}
	sort.Slice(exps, func(i, j int) bool {
		return exps[i].After(*exps[j])
	})
	now := time.Now()
	if exps[0].After(now) {
		return exps[0].Sub(now), nil
	}
	return time.Duration(0), fmt.Errorf("peer expired")
}

// dbDeleteIter removes all entries whose keys are prefixed with the argument,
// or '/ttl' + artument.
func (ab *addrBook) dbDeleteIter(prefix key.Key) error {
	txn, err := ab.store.NewTransaction()
	if err != nil {
		return err
	}
	defer txn.Discard()

	for _, k := range txn.KeysWithPrefix(prefix.Bytes()) {
		if err := txn.Del(k); err != nil {
			return err
		}

		ttlkey := ttlBase.Child(key.NewKeyFromBytes(k))
		if err := txn.Del(ttlkey.Bytes()); err != nil {
			return err
		}
	}

	return txn.Commit()
}

// dbRemoveExpired removes expired peers
func (ab *addrBook) dbRemoveExpired() error {
	var err error

	txn, err := ab.store.NewTransaction()
	if err != nil {
		return err
	}
	defer txn.Discard()

	now := time.Now()
	for _, k := range txn.KeysWithPrefix(ttlBase.Bytes()) {
		if buf, err := txn.Get(k); err == nil && len(buf) > 0 {
			var exp time.Time
			if err := exp.UnmarshalBinary(buf); err == nil {
				if !exp.IsZero() && exp.Before(now) {
					if err = txn.Del(k); err != nil {
						return err
					}

					// del key of multiaddr
					ttlKey := key.NewKeyFromBytes(k)
					n := ttlKey.List()
					if len(n) > 1 {
						addKey := key.NewKey(strings.Join(n[1:], "/"))
						logger.Debug("removing key of addrbook %s", addKey)
						if err = txn.Del(addKey.Bytes()); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return txn.Commit()
}
