// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	se "github.com/BOXFoundation/Quicksilver/p2p/serialize"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Message Define message interface
type Message interface {
	Code() uint32
	Body() []byte
}

// Net Define Net interface
type Net interface {
	Broadcast(uint32, se.Serializable) error
	SendMessageToPeer(uint32, se.Serializable, peer.ID)
	Subscribe(*Notifiee)
	UnSubscribe(*Notifiee)
}
