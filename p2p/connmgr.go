// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/jbenet/goprocess"
	goprocessctx "github.com/jbenet/goprocess/context"
	ifconnmgr "github.com/libp2p/go-libp2p-interface-connmgr"
	inet "github.com/libp2p/go-libp2p-net"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

// ConnManager is an object to maitian all connections
type ConnManager struct {
	mutex sync.Mutex

	tagInfos  map[peer.ID]*ifconnmgr.TagInfo
	peerstore peerstore.Peerstore

	proc                goprocess.Process
	durationToTrimConns time.Duration
}

const secondsToTrimConnections = 30

// ConnStatus represents the connection status
type ConnStatus int32

const (
	// ConnStatusTagName is the tag name of ConnStatus
	ConnStatusTagName = "cmgr:status"

	// ConnStatusNotTried means there is no tries to connect with the peer
	ConnStatusNotTried ConnStatus = 0

	// ConnStatusConnected means the peer is connected.
	ConnStatusConnected ConnStatus = 1

	// ConnStatusDisconnected means the peer is disconnected.
	ConnStatusDisconnected ConnStatus = 2
)

const (
	// ConnMaxCapacity means the max capacity of the conn pool
	ConnMaxCapacity = 200

	// ConnLoadFactor means the threshold of gc
	ConnLoadFactor = 0.8
)

func (cs ConnStatus) String() string {
	switch cs {
	case ConnStatusConnected:
		return "Connected"
	case ConnStatusDisconnected:
		return "Disconnected"
	case ConnStatusNotTried:
		fallthrough
	default:
		return "NotTried"

	}
}

// NewConnManager creates a ConnManager object, which is used on Host object.
func NewConnManager(ps peerstore.Peerstore) *ConnManager {
	cmgr := ConnManager{
		tagInfos:            make(map[peer.ID]*ifconnmgr.TagInfo),
		durationToTrimConns: time.Second * 30,
		peerstore:           ps,
	}
	return &cmgr
}

// Loop starts a go thread to loop removing unnecessary connections.
func (cm *ConnManager) Loop(parent goprocess.Process) {
	cm.mutex.Lock()
	cm.loop(parent)
	cm.mutex.Unlock()
}

// loop removing unnecessary connections
func (cm *ConnManager) loop(parent goprocess.Process) {
	if cm.proc != nil {
		logger.Warn("Connection Manager has already been running.")
		return
	}

	logger.Info("Now starting connection manager.")
	cm.proc = parent.Go(func(p goprocess.Process) {
		ticker := time.NewTicker(cm.durationToTrimConns)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				<-p.Go(func(proc goprocess.Process) {
					cm.TrimOpenConns(goprocessctx.OnClosingContext(proc))
				}).Closed() // blocked on go function
			case <-p.Closing():
				logger.Info("Quit connection manager.")
				return
			}
		}
	})
}

// Stop function stops the ConnManager
func (cm *ConnManager) Stop() {
	cm.mutex.Lock()
	if cm.proc != nil {
		cm.proc.Close()
	}
	cm.mutex.Unlock()
}

// implement interface inet.Notifiee
var _ inet.Notifiee = (*ConnManager)(nil)

// Listen is called when network starts listening on an addr
func (cm *ConnManager) Listen(network net.Network, multiaddr ma.Multiaddr) {
}

// ListenClose is called when network starts listening on an addr
func (cm *ConnManager) ListenClose(network net.Network, multiaddr ma.Multiaddr) {
}

// Connected is called when a connection opened
func (cm *ConnManager) Connected(network net.Network, conn net.Conn) {
	// update conn status to NetConnected
	pid := conn.RemotePeer()

	cm.mutex.Lock()
	cm.tagPeer(pid, ConnStatusTagName, int(ConnStatusConnected))
	tagInfo, ok := cm.tagInfos[pid]
	if ok {
		tagInfo.Conns[conn.RemoteMultiaddr().String()] = time.Now()
	}
	cm.mutex.Unlock()
}

// Disconnected is called when a connection closed
func (cm *ConnManager) Disconnected(network net.Network, conn net.Conn) {
	pid := conn.RemotePeer()

	cm.mutex.Lock()
	cm.tagPeer(pid, ConnStatusTagName, int(ConnStatusDisconnected))
	tagInfo, ok := cm.tagInfos[pid]
	if ok {
		tagInfo.Value = 0
		delete(tagInfo.Conns, conn.RemoteMultiaddr().String())
	}
	cm.mutex.Unlock()
}

// OpenedStream is called when a stream opened
func (cm *ConnManager) OpenedStream(network net.Network, stream net.Stream) {
	pid := stream.Conn().RemotePeer()

	cm.mutex.Lock()
	cm.tagPeer(pid, ConnStatusTagName, int(ConnStatusConnected))
	tagInfo, ok := cm.tagInfos[pid]
	if ok {
		tagInfo.Value++
	}
	cm.mutex.Unlock()
}

// ClosedStream is called when a stream closed
func (cm *ConnManager) ClosedStream(network net.Network, stream net.Stream) {
	pid := stream.Conn().RemotePeer()

	cm.mutex.Lock()
	tagInfo, ok := cm.tagInfos[pid]
	if ok {
		if tagInfo.Value > 0 {
			tagInfo.Value--
		}
	}
	cm.mutex.Unlock()
}

// implement interface ifconnmgr.ConnManager
var _ ifconnmgr.ConnManager = (*ConnManager)(nil)

// TagPeer tags a peer with a string, associating a weight with the tag.
func (cm *ConnManager) TagPeer(p peer.ID, tag string, value int) {
	cm.mutex.Lock()
	cm.tagPeer(p, tag, value)
	cm.mutex.Unlock()
}

func (cm *ConnManager) tagPeer(p peer.ID, tag string, value int) {
	tagInfo, ok := cm.tagInfos[p]
	if !ok {
		tagInfo = &ifconnmgr.TagInfo{
			FirstSeen: time.Now(),
			Value:     0,
			Tags:      make(map[string]int),
			Conns:     make(map[string]time.Time),
		}
		cm.tagInfos[p] = tagInfo
	}
	tagInfo.Tags[tag] = value
}

// UntagPeer removes the tagged value from the peer.
func (cm *ConnManager) UntagPeer(p peer.ID, tag string) {
	cm.mutex.Lock()
	cm.untagPeer(p, tag)
	cm.mutex.Unlock()
}

func (cm *ConnManager) untagPeer(p peer.ID, tag string) {
	tagInfo, ok := cm.tagInfos[p]
	if ok {
		delete(tagInfo.Tags, tag)
	}
}

// GetTagInfo returns the metadata associated with the peer,
// or nil if no metadata has been recorded for the peer.
func (cm *ConnManager) GetTagInfo(p peer.ID) *ifconnmgr.TagInfo {
	cm.mutex.Lock()
	ti := cm.getTagInfo(p)
	if ti != nil {
		// deep copy TagInfo
		tagInfo := &ifconnmgr.TagInfo{
			FirstSeen: ti.FirstSeen,
			Value:     ti.Value,
			Tags:      make(map[string]int),
			Conns:     make(map[string]time.Time),
		}
		for k, v := range ti.Tags {
			tagInfo.Tags[k] = v
		}
		for k, v := range ti.Conns {
			tagInfo.Conns[k] = v
		}
		ti = tagInfo
	}
	cm.mutex.Unlock()
	return ti
}

func (cm *ConnManager) getTagInfo(p peer.ID) *ifconnmgr.TagInfo {
	if ti, ok := cm.tagInfos[p]; ok {
		return ti
	}

	return nil
}

// TrimOpenConns terminates open connections based on an implementation-defined
// heuristic.
func (cm *ConnManager) TrimOpenConns(ctx context.Context) {
	//TODO: close unnecessary connections...
	logger.Warn("TrimOpenConns was called, but not implemented.")
	cm.mutex.Lock()
	var (
		total     = 0
		connected = 0
		opened    = 0
	)
	for _, i := range cm.tagInfos {
		total++
		if ConnStatus(i.Tags[ConnStatusTagName]) == ConnStatusConnected {
			connected++
			opened += i.Value
		}
		//logger.Warnf("Peer %s: first seen at: %v, conn status %s, open streams %d", p.Pretty(), i.FirstSeen, ConnStatus(i.Tags[ConnStatusTagName]), i.Value)
	}
	logger.Infof("Peers: %d, connected: %d, opened: %d", total, connected, opened)
	cm.mutex.Unlock()
}

// Notifee returns an implementation that can be called back to inform of
// opened and closed connections.
func (cm *ConnManager) Notifee() inet.Notifiee {
	return cm
}
