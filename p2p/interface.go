// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Message Define message interface
type Message interface {
	Code() uint32
	Body() []byte
	From() peer.ID
}

// Net Define Net interface
type Net interface {
	Broadcast(uint32, conv.Convertible) error
	SendMessageToPeer(uint32, conv.Convertible, peer.ID) error
	Subscribe(*Notifiee)
	UnSubscribe(*Notifiee)
	Notify(Message)
	PickOnePeer(peersExclusive ...peer.ID) peer.ID
	BroadcastToMiners(uint32, conv.Convertible, []string) error
	// PeerSynced(peers ...peer.ID) map[peer.ID]bool
}
