// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pstore

import (
	"strings"
)

// PeerType is used to distinguish peer types in the p2p network.
type PeerType uint8

const (
	// UnkownPeer identifies an unkown peer.
	UnkownPeer PeerType = iota
	// ServerPeer provides RPC services. It is recommended to configure it as a seed node.
	ServerPeer
	// MinerPeer acts as the mining node.
	MinerPeer
	// CandidatePeer identifies the MinerPeer of the next dynasty.
	CandidatePeer
	// LayfolkPeer identifies a common full node.
	LayfolkPeer
)

// ParsePeerType takes a string level and returns the Logrus log level constant.
func ParsePeerType(pt string) PeerType {
	switch strings.ToLower(pt) {
	case "server":
		return ServerPeer
	case "miner":
		return MinerPeer
	case "candidate":
		return CandidatePeer
	case "layfolk":
		return LayfolkPeer
	default:
		return UnkownPeer
	}
}

// Mask coverts PeerType to uint8. Each bit refers to a PeerType.
func (pt PeerType) Mask() uint8 {
	if pt == UnkownPeer {
		return 0
	}
	return 1 << (pt - 1)
}
