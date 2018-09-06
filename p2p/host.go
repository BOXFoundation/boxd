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
	priv, pub, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", listenAddress, listenPort)),
		libp2p.Identity(priv),
		libp2p.ConnectionManager(NewConnManager()),
		libp2p.Peerstore(ps), // TODO NAT/Relay/...
	}

	host, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/%s/%s", ma.ProtocolWithCode(ma.P_P2P).Name, basicHost.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	addr := basicHost.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	fmt.Printf("Now listening on %s", fullAddr) //TODO change to logger

	return &Host{Host: host}, nil
}
