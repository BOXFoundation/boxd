// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"

	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

const defaultPort = 9199

// Host is host.Host wrapper
type Host struct {
	host.Host
	cmgr    *ConnManager
	routing *dht.IpfsDHT
	ctx     context.Context
	cancel  context.CancelFunc
	mutex   sync.Mutex
}

// init function
func init() {
	ma.SwapToP2pMultiaddrs() // change ma.P_P2P from 'ipfs' to 'p2p'
}

// NewDefaultHost creates a wrapper of host.Host
func NewDefaultHost(ctx context.Context) (*Host, error) {
	return NewHost(ctx, defaultPort, "0.0.0.0", pstore.NewPeerstore())
}

// NewHost creates a wrapper of host.Host, with given peerstore & notifiee, and listening on given port/address
func NewHost(ctx context.Context, listenPort int, listenAddress string, ps pstore.Peerstore) (*Host, error) {
	var r = rand.Reader

	// Generate a key pair for this host. We will use it at least
	// to obtain a valid host ID.
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, err
	}

	hostContext, cancel := context.WithCancel(ctx)

	cmgr := NewConnManager()
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", listenAddress, listenPort)),
		libp2p.Identity(priv),
		libp2p.ConnectionManager(cmgr),
		libp2p.Peerstore(ps), // TODO NAT/Relay/...
	}

	localhost, err := libp2p.New(hostContext, opts...)
	if err != nil {
		defer cancel()
		return nil, err
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/%s/%s", ma.ProtocolWithCode(ma.P_P2P).Name, localhost.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	addr := localhost.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	fmt.Printf("Now listening on %s", fullAddr) //TODO change to logger

	// create dht routing table
	routing, err := dht.New(hostContext, localhost)
	if err != nil {
		defer cancel()
		return nil, err
	}

	h := &Host{Host: localhost, cmgr: cmgr, routing: routing, ctx: hostContext, cancel: cancel}

	// start connmanager
	h.cmgr.Start(h.ctx)
	//  bootstrap dht routing table
	h.routing.Bootstrap(h.ctx)

	return h, nil
}

// Context returns the running context of the Host object
func (h *Host) Context() context.Context {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.ctx == nil {
		return context.Background()
	}
	return h.ctx
}

// Stop function stops the Host
func (h *Host) Stop() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.cancel != nil {
		h.cancel()
	}
}
