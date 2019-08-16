// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// TableReader defines basic operations routing table exposes
type TableReader interface {
	ConnectingPeers() []string
	PeerID() string
}
