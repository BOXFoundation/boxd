// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"math/rand"
	"time"

	"github.com/BOXFoundation/boxd/p2p/pb"
	"github.com/BOXFoundation/boxd/util"
	"github.com/jbenet/goprocess"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

// const
const (
	PeerDiscoverLoopInterval        = 10 * time.Second
	MaxPeerCountToSyncRouteTable    = 16
	MaxPeerCountToReplyPeerDiscover = 16
)

// Table peer route table struct.
type Table struct {
	peerStore  peerstore.Peerstore
	routeTable *kbucket.RoutingTable
	peer       *BoxPeer
	proc       goprocess.Process
}

// NewTable return a new route table.
func NewTable(peer *BoxPeer) *Table {

	table := &Table{
		peerStore: peer.host.Peerstore(),
		peer:      peer,
	}
	table.routeTable = kbucket.NewRoutingTable(
		peer.config.Bucketsize,
		kbucket.ConvertPeerID(peer.id),
		peer.config.Latency,
		table.peerStore,
	)
	table.routeTable.Update(peer.id)
	table.peerStore.AddPubKey(peer.id, peer.networkIdentity.GetPublic())
	table.peerStore.AddPrivKey(peer.id, peer.networkIdentity)

	return table
}

// Loop for discover new peer.
func (t *Table) Loop(parent goprocess.Process) {
	t.peerDiscover()

	t.proc = parent.Go(func(p goprocess.Process) {
		loopTicker := time.NewTicker(PeerDiscoverLoopInterval)
		defer loopTicker.Stop()
		for {
			select {
			case <-loopTicker.C:
				t.peerDiscover()
			case <-p.Closing():
				logger.Info("Quit route table loop.")
				return
			}
		}
	})
}

func (t *Table) peerDiscover() {
	logger.Info("do peer discover")
	all := t.peerStore.Peers()
	if len(all) <= MaxPeerCountToSyncRouteTable {
		for _, v := range all {
			if v.Pretty() == t.peer.id.Pretty() {
				continue
			}
			go t.lookup(v)
		}
		return
	}

	// Randomly select some peer to do sync routes from the established and unconnected peers
	// 3/4 from established peers, and 1/4 from unconnected peers
	var establishedID []peer.ID
	for k := range t.peer.conns {
		establishedID = append(establishedID, k)
	}

	var unestablishedID []peer.ID
	for _, v := range all {
		if !util.InArray(v, establishedID) {
			unestablishedID = append(unestablishedID, v)
		}
	}

	var peerIDs []peer.ID
	if len(unestablishedID) < MaxPeerCountToSyncRouteTable/4 {
		peerIDs = append(peerIDs, unestablishedID...)
		peerIDs = append(peerIDs, establishedID[:MaxPeerCountToSyncRouteTable-len(unestablishedID)]...)
	} else {
		peerIDs = append(peerIDs, establishedID...)
		peerIDs = append(peerIDs, unestablishedID[:MaxPeerCountToSyncRouteTable-len(establishedID)]...)
	}

	for _, v := range peerIDs {
		go t.lookup(v)
	}
}

func (t *Table) lookup(pid peer.ID) {
	var conn *Conn
	if _, ok := t.peer.conns[pid]; ok {
		// established peer
		conn = t.peer.conns[pid].(*Conn)
	} else {
		// unestablished peer
		conn = NewConn(nil, t.peer, pid)
		conn.Loop(t.peer.proc)
	}
	if err := conn.PeerDiscover(); err != nil {
		logger.Errorf("Failed to sync route table from peer: %s err: %s", pid.Pretty(), err.Error())
	}
}

// GetRandomPeers get random peers
func (t *Table) GetRandomPeers(pid peer.ID) []peerstore.PeerInfo {

	peers := shufflePeerID(t.routeTable.ListPeers())
	if len(peers) > MaxPeerCountToReplyPeerDiscover {
		peers = peers[:MaxPeerCountToReplyPeerDiscover]
	}
	ret := make([]peerstore.PeerInfo, len(peers))
	for i, v := range peers {
		ret[i] = t.peerStore.PeerInfo(v)
	}
	return ret
}

// AddPeerToTable add peer route table.
func (t *Table) AddPeerToTable(conn *Conn) {

	peerID := conn.stream.Conn().RemotePeer()
	t.peerStore.AddAddr(
		peerID,
		conn.stream.Conn().RemoteMultiaddr(),
		peerstore.PermanentAddrTTL,
	)
	t.routeTable.Update(peerID)
}

// AddPeers add peers to route table
func (t *Table) AddPeers(conn *Conn, peers *p2ppb.Peers) {
	if len(peers.Peers) > MaxPeerCountToReplyPeerDiscover {
		conn.Close()
	}
	for _, v := range peers.Peers {
		t.addPeerInfo(v.Id, v.Addrs)
	}
}

func (t *Table) addPeerInfo(prettyID string, addrStr []string) error {
	pid, err := peer.IDB58Decode(prettyID)
	if err != nil {
		return nil
	}

	addrs := make([]ma.Multiaddr, len(addrStr))
	for i, v := range addrStr {
		addrs[i], err = ma.NewMultiaddr(v)
		if err != nil {
			return err
		}
	}
	if t.routeTable.Find(pid) != "" {
		t.peerStore.SetAddrs(pid, addrs, peerstore.PermanentAddrTTL)
	} else {
		t.peerStore.AddAddrs(pid, addrs, peerstore.PermanentAddrTTL)

	}
	t.routeTable.Update(pid)

	return nil
}

func shufflePeerID(pids []peer.ID) []peer.ID {

	r := rand.New(rand.NewSource(time.Now().Unix()))
	ret := make([]peer.ID, len(pids))
	perm := r.Perm(len(pids))
	for i, randIndex := range perm {
		ret[i] = pids[randIndex]
	}
	return ret
}
