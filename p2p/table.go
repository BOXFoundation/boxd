// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"math"
	"math/rand"
	"time"

	p2ppb "github.com/BOXFoundation/boxd/p2p/pb"
	"github.com/BOXFoundation/boxd/p2p/pstore"
	"github.com/BOXFoundation/boxd/util"
	"github.com/jbenet/goprocess"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

// const
const (
	PeerDiscoverLoopInterval        = 120 * 1000
	MaxPeerCountToSyncRouteTable    = 16
	MaxPeerCountToReplyPeerDiscover = 16
	DefaultUnestablishRatio         = 0.25
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
	var cnt float64
	t.peerDiscover()
	t.proc = parent.Go(func(p goprocess.Process) {
		interval := time.Duration(calcTimeInterval(cnt) * 1000)
		timer := time.NewTimer(interval * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				t.peerDiscover()
				if cnt < 100 {
					cnt++
					interval = time.Duration(calcTimeInterval(float64(cnt)) * 1000)
				} else {
					interval = PeerDiscoverLoopInterval
				}
				timer.Reset(interval * time.Millisecond)
			case <-p.Closing():
				logger.Info("Quit route table loop.")
				return
			}
		}
	})
}

func calcTimeInterval(val float64) float64 {
	temp := float64(-(val) / 3.0)
	return math.Trunc(math.Pow(2-math.Exp2(temp), math.Log2(120))*1e2+0.5) * 1e-2
}

func (t *Table) peerDiscover() {
	logger.Info("do peer discover")
	var all peer.IDSlice
	for _, p := range t.peerStore.Peers() {
		// skip peer without address
		if len(t.peerStore.Addrs(p)) > 0 {
			all = append(all, p)
		}
	}
	// TODO check peer score
	if len(all) <= MaxPeerCountToSyncRouteTable {
		// TODO sort by peer score
		for _, v := range all {
			go t.lookup(v)
		}
		return
	}

	var peerIDs []peer.ID

	switch t.peer.peertype {
	case pstore.MinerPeer:
		peerIDs = t.minerDiscover(all)
	case pstore.CandidatePeer:
		peerIDs = t.candidateDiscover(all)
	case pstore.ServerPeer:
		peerIDs = t.serverDiscover(all)
	default:
		peerIDs = t.defaultDiscover(all)
	}

	for _, v := range peerIDs {
		go t.lookup(v)
	}
}

func (t *Table) selectTypedPeers(pt pstore.PeerType, num int) []peer.ID {
	peerIDs, err := pstore.ListPeerIDByType(pt)
	if err != nil {
		logger.Errorf("selectTypedPeers failed, Err: %v", err)
		return nil
	}
	peerIDs = shufflePeerID(peerIDs)
	if len(peerIDs) > num {
		return peerIDs[:num]
	}
	return peerIDs
}

// FIXME: 加对节点类型的判断
// param: std standard deviation
func (t *Table) selectRandomPeers(all peer.IDSlice, num uint32, std float32, layfolk bool) (peerIDs []peer.ID) {
	// Randomly select some peer to do sync routes from the established and unconnected peers
	// 1-<std> from established peers, and <std> from unconnected peers
	var establishedID []peer.ID
	t.peer.conns.Range(func(k, v interface{}) bool {
		establishedID = append(establishedID, k.(peer.ID))
		return true
	})

	var unestablishedID []peer.ID
	for _, v := range all {
		if !util.InArray(v, establishedID) {
			unestablishedID = append(unestablishedID, v)
		}
	}

	cap := int(num)
	if len(unestablishedID) < int(float32(cap)*std) {
		peerIDs = append(peerIDs, unestablishedID...)
		peerIDs = append(peerIDs, establishedID[:cap-len(unestablishedID)]...)
	} else if len(establishedID) > cap {
		peerIDs = append(peerIDs, unestablishedID[:int(float32(cap)*std)]...)
		peerIDs = append(peerIDs, establishedID[:cap-len(peerIDs)]...)
	} else {
		peerIDs = append(peerIDs, establishedID...)
		peerIDs = append(peerIDs, unestablishedID[:cap-len(establishedID)]...)
	}
	return
}

// Returns a list of IDs configured with the current node.
func (t *Table) selectConfigedPeers() (peerIDs []peer.ID, seeds []peer.ID) {

	// for _, seed := range t.peer.config.Seeds {
	// 	seedEle := strings.Split(seed, "/")
	// 	pid, err := peer.IDFromString(seedEle[len(seedEle)-1])
	// 	seeds = append(seeds, pid)
	// }

	// return peerIDs, t.peer.config.Seeds
	return
}

func (t *Table) minerDiscover(all peer.IDSlice) (peerIDs []peer.ID) {
	servers, _ := t.selectConfigedPeers()

	if len(servers) == 0 {
		return t.defaultDiscover(all)
	}
	peerIDs = append(peerIDs, servers...)

	candidates := t.selectTypedPeers(pstore.CandidatePeer, MaxPeerCountToSyncRouteTable/2)
	peerIDs = append(peerIDs, candidates...)

	miners := t.selectTypedPeers(pstore.MinerPeer, MaxPeerCountToSyncRouteTable-len(peerIDs))
	peerIDs = append(peerIDs, miners...)

	return
}

func (t *Table) candidateDiscover(all peer.IDSlice) (peerIDs []peer.ID) {
	servers, _ := t.selectConfigedPeers()

	if len(servers) == 0 {
		return t.defaultDiscover(all)
	}
	peerIDs = append(peerIDs, servers...)

	miners := t.selectTypedPeers(pstore.MinerPeer, MaxPeerCountToSyncRouteTable/2)
	peerIDs = append(peerIDs, miners...)

	candidates := t.selectTypedPeers(pstore.CandidatePeer, MaxPeerCountToSyncRouteTable-len(peerIDs))
	peerIDs = append(peerIDs, candidates...)
	return
}

func (t *Table) serverDiscover(all peer.IDSlice) (peerIDs []peer.ID) {
	configedPeers, _ := t.selectConfigedPeers()
	if len(configedPeers) != 0 {
		peerIDs = append(peerIDs, configedPeers...)
	}
	peerIDs = append(peerIDs, t.selectRandomPeers(all, uint32(MaxPeerCountToSyncRouteTable-len(peerIDs)), DefaultUnestablishRatio, true)...)
	return
}

func (t *Table) defaultDiscover(all peer.IDSlice) (peerIDs []peer.ID) {
	return t.selectRandomPeers(all, MaxPeerCountToSyncRouteTable, DefaultUnestablishRatio, false)
}

func (t *Table) lookup(pid peer.ID) {
	if pid.Pretty() == t.peer.id.Pretty() {
		return
	}
	var conn *Conn
	if c, ok := t.peer.conns.Load(pid); ok {
		// established peer
		conn = c.(*Conn)
	} else {
		// unestablished peer
		conn = NewConn(nil, t.peer, pid)
		conn.Loop(t.peer.proc)
	}

	if err := conn.PeerDiscover(); err != nil {
		logger.Warnf("Failed to sync route table from peer: %s err: %s", pid.Pretty(), err)
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
	// FIXME: 判断如果有就不put，如果是超级节点或者是备选节点或者自己配好的服务节点，则附相应的值
	t.peerStore.Put(peerID, pstore.PTypeSuf, uint8(pstore.UnkownPeer))
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
		logger.Errorf("Add too many peers. peers num: %d", len(peers.Peers))
		conn.Close()
	}
	for _, v := range peers.Peers {
		pid, err := peer.IDB58Decode(v.Id)
		if err != nil {
			logger.Errorf("get pid failed. Err: %v", err)
			continue
		}
		t.peerStore.Put(pid, pstore.PTypeSuf, uint8(v.Type))
		pstore.PutType(pid, v.Type, time.Now().Unix())
		t.addPeerInfo(pid, v.Addrs)
	}
}

func (t *Table) addPeerInfo(pid peer.ID, addrStr []string) (err error) {

	addrs := make([]ma.Multiaddr, len(addrStr))
	for i, v := range addrStr {
		addrs[i], err = ma.NewMultiaddr(v)
		if err != nil {
			return
		}
	}
	if t.routeTable.Find(pid) != "" {
		t.peerStore.SetAddrs(pid, addrs, peerstore.OwnObservedAddrTTL)
	} else {
		t.peerStore.AddAddrs(pid, addrs, peerstore.OwnObservedAddrTTL)

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
