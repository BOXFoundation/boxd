// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"context"
	"sync"
	"time"

	ifconnmgr "github.com/libp2p/go-libp2p-interface-connmgr"
	inet "github.com/libp2p/go-libp2p-net"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
)

// ConnManager is an object to maitian all connections
type ConnManager struct {
	mutex               sync.Mutex
	tagInfos            map[peer.ID]*ifconnmgr.TagInfo
	notifiee            inet.Notifiee
	context             context.Context
	cancel              context.CancelFunc
	durationToTrimConns time.Duration
}

const (
	// TODO define some TAGs for TagInfo

	// TagLastActive is timestamp of the last message received from the peer.
	TagLastActive = "LastActive"

	// TagConnStatus is the connection status
	TagConnStatus = "ConnStatus"
)

const (
	// NetConnected means net connected
	NetConnected = iota
	// NetDisconnected means net disconnected
	NetDisconnected
)

const secondsToTrimConnections = 30

// NewConnManager creates a ConnManager object, which is used on Host object.
func NewConnManager() *ConnManager {
	cmgr := ConnManager{
		tagInfos:            make(map[peer.ID]*ifconnmgr.TagInfo),
		durationToTrimConns: time.Second * 30,
	}
	cmgr.notifiee = newNotifiee(&cmgr)
	return &cmgr
}

// Start function starts a go thread to loop removing unnecessary connections.
func (cm *ConnManager) Start(ctx context.Context) {
	go cm.run(ctx)
}

// loop removing unnecessary connections
func (cm *ConnManager) run(ctx context.Context) {
	cm.mutex.Lock()
	if cm.context != nil {
		cm.mutex.Unlock()
		logger.Warn("Connection Manager has already been running.\n")
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cm.context, cm.cancel = context.WithCancel(ctx)
	cm.mutex.Unlock()

	ticker := time.NewTicker(cm.durationToTrimConns)
	for {
		select {
		case <-ticker.C:
			go func(ctx context.Context) {
				cm.TrimOpenConns(ctx)
			}(cm.context)
		case <-cm.context.Done():
			logger.Info("Quit connection manager.")
			return
		}
	}
}

// Stop function stops the ConnManager
func (cm *ConnManager) Stop() {
	cm.mutex.Lock()
	if cm.cancel != nil {
		cm.cancel()
	}
	cm.mutex.Unlock()
}

// notifiee implements interface inet.Notifiee, which is called when network connection changes.
type notifiee struct {
	cmgr *ConnManager
}

// newNotifiee creates a notifiee object, which implements interface inet.Notifiee.
func newNotifiee(cmgr *ConnManager) inet.Notifiee {
	return &notifiee{cmgr: cmgr}
}

// implement interface inet.Notifiee
// TODO not implemented

// called when network starts listening on an addr
func (n *notifiee) Listen(network net.Network, multiaddr ma.Multiaddr) {
}

// called when network starts listening on an addr
func (n *notifiee) ListenClose(network net.Network, multiaddr ma.Multiaddr) {
}

// called when a connection opened
func (n *notifiee) Connected(network net.Network, conn net.Conn) {
	// update conn status to NetConnected
	n.cmgr.TagPeer(conn.RemotePeer(), TagConnStatus, NetConnected)
	n.cmgr.TagPeer(conn.RemotePeer(), TagLastActive, int(time.Now().Unix()))
}

// called when a connection closed
func (n *notifiee) Disconnected(network net.Network, conn net.Conn) {
	n.cmgr.TagPeer(conn.RemotePeer(), TagConnStatus, NetDisconnected)
}

// called when a stream opened
func (n *notifiee) OpenedStream(network net.Network, stream net.Stream) {
	n.cmgr.TagPeer(stream.Conn().RemotePeer(), TagLastActive, int(time.Now().Unix()))
}

// called when a stream closed
func (n *notifiee) ClosedStream(network net.Network, stream net.Stream) {
}

// implement interface ifconnmgr.ConnManager

// TagPeer tags a peer with a string, associating a weight with the tag.
func (cm *ConnManager) TagPeer(p peer.ID, tag string, value int) {
	cm.mutex.Lock()

	tagInfo, ok := cm.tagInfos[p]
	if !ok {
		tagInfo = &ifconnmgr.TagInfo{
			FirstSeen: time.Now(),
			Value:     0, // TODO what does it mean??
			Tags:      make(map[string]int),
			Conns:     make(map[string]time.Time),
		}
		cm.tagInfos[p] = tagInfo
	}
	tagInfo.Tags[tag] = value

	cm.mutex.Unlock()
}

// UntagPeer removes the tagged value from the peer.
func (cm *ConnManager) UntagPeer(p peer.ID, tag string) {
	cm.mutex.Lock()

	tagInfo, ok := cm.tagInfos[p]
	if ok {
		delete(tagInfo.Tags, tag)
	}

	cm.mutex.Unlock()
}

// GetTagInfo returns the metadata associated with the peer,
// or nil if no metadata has been recorded for the peer.
func (cm *ConnManager) GetTagInfo(p peer.ID) *ifconnmgr.TagInfo {
	if ti, ok := cm.tagInfos[p]; ok {
		return ti
	}

	return nil
}

// TrimOpenConns terminates open connections based on an implementation-defined
// heuristic.
func (cm *ConnManager) TrimOpenConns(ctx context.Context) {
	//TODO close unnecessary connections...
}

// Notifee returns an implementation that can be called back to inform of
// opened and closed connections.
func (cm *ConnManager) Notifee() inet.Notifiee {
	return cm.notifiee
}
