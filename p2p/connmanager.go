// MIT License
//
// Copyright (c) 2018 ContentBox Foundation
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package p2p

import (
	"context"
	"time"

	ifconnmgr "github.com/libp2p/go-libp2p-interface-connmgr"
	inet "github.com/libp2p/go-libp2p-net"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
)

// ConnManager is an object to maitian all connections
type ConnManager struct {
	tagInfos map[peer.ID]*ifconnmgr.TagInfo
	notifiee inet.Notifiee
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

// NewConnManager creates a ConnManager object, which is used on Host object.
func NewConnManager() ConnManager {
	cmgr := ConnManager{
		tagInfos: make(map[peer.ID]*ifconnmgr.TagInfo),
	}
	cmgr.notifiee = newNotifiee(&cmgr)
	return cmgr
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
func (n *notifiee) Listen(net net.Network, multiaddr ma.Multiaddr) {

}

// called when network starts listening on an addr
func (n *notifiee) ListenClose(net net.Network, multiaddr ma.Multiaddr) {

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
}

// UntagPeer removes the tagged value from the peer.
func (cm *ConnManager) UntagPeer(p peer.ID, tag string) {
	tagInfo, ok := cm.tagInfos[p]
	if ok {
		delete(tagInfo.Tags, tag)
	}
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
