// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"context"
	"crypto/rand"
	"fmt"

	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

const defaultPort = 9199

// Host is host.Host wrapper
type Host struct {
	host.Host
}

// init function
func init() {
	ma.SwapToP2pMultiaddrs() // change ma.P_P2P from 'ipfs' to 'p2p'
}

// NewHost creates a wrapper of host.Host
func NewHost() (*Host, error) {
	return NewHostWithAddress(defaultPort, "0.0.0.0", pstore.NewPeerstore())
}

// NewHostWithAddress creates a wrapper of host.Host, listening on given port/address
func NewHostWithAddress(listenPort int, listenAddress string, ps pstore.Peerstore) (*Host, error) {
	var r = rand.Reader

	// Generate a key pair for this host. We will use it at least
	// to obtain a valid host ID.
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, err
	}

	cm := NewConnManager()
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", listenAddress, listenPort)),
		libp2p.Identity(priv),
		libp2p.ConnectionManager(&cm),
		libp2p.Peerstore(ps), // TODO NAT/Relay/...
	}

	host, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/%s/%s", ma.ProtocolWithCode(ma.P_P2P).Name, host.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	addr := host.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	fmt.Printf("Now listening on %s", fullAddr) //TODO change to logger

	return &Host{Host: host}, nil
}
