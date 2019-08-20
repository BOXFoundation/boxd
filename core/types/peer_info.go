// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

// PeerInfo is a small struct used to pass around a peer with
// a set of addresses.
type PeerInfo struct {
	ID     string
	Addr   string
	Iplist []string
}
