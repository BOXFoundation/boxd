// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	proto "github.com/gogo/protobuf/proto"
	peer "github.com/libp2p/go-libp2p-peer"
)

// Serializable Define serialize interface
type Serializable interface {
	Serialize() (proto.Message, error)
	Deserialize(proto.Message) error
}

// Message Define message interface
type Message interface {
	Code() uint32
	Body() []byte
}

// Net Define Net interface
type Net interface {
	Bootstrap()
	Broadcast(uint32, Serializable)
	SendMessageToPeer(uint32, Serializable, peer.ID)
	Subscribe(*Notifiee)
	UnSubscribe(*Notifiee)
}
