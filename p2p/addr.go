// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"fmt"

	host "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
)

// init function
func init() {
	ma.SwapToP2pMultiaddrs() // change ma.P_P2P from 'ipfs' to 'p2p'
}

// PeerMultiAddr returns the full p2p multiaddr of specified host.
// e.g. /ip4/192.168.10.34/tcp/19199/p2p/12D3KooWLornEge5BiVbL92o8wdFivY4c7GV3QdfmjkFk7Vm48Uk
func PeerMultiAddr(h host.Host) (ma.Multiaddr, error) {
	// Build host multiaddress
	hostAddr, err := ma.NewMultiaddr(fmt.Sprintf("/%s/%s", ma.ProtocolWithCode(ma.P_P2P).Name, h.ID().Pretty()))
	if err != nil {
		return nil, err
	}

	// full multiaddress to reach this host
	return h.Addrs()[0].Encapsulate(hostAddr), nil
}

// DecapsulatePeerMultiAddr decapsulates a p2p multiaddr into host multiaddr and peer id
// e.g. /ip4/192.168.10.34/tcp/19199/p2p/12D3KooWLornEge5BiVbL92o8wdFivY4c7GV3QdfmjkFk7Vm48Uk
// decapsuated to /ip4/192.168.10.34/tcp/19199, 12D3KooWLornEge5BiVbL92o8wdFivY4c7GV3QdfmjkFk7Vm48Uk
func DecapsulatePeerMultiAddr(addr ma.Multiaddr) (ma.Multiaddr, peer.ID, error) {
	pid, err := addr.ValueForProtocol(ma.P_P2P)
	if err != nil {
		return nil, "", err
	}

	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		return nil, "", err
	}

	pma, _ := ma.NewMultiaddr(
		fmt.Sprintf("/%s/%s", ma.ProtocolWithCode(ma.P_P2P).Name, pid),
	)
	lma := addr.Decapsulate(pma)

	return lma, peerid, nil
}

// EncapsulatePeerMultiAddr encapsulates a host multiaddr and peer id into p2p multiaddr
// e.g. /ip4/192.168.10.34/tcp/19199 and 12D3KooWLornEge5BiVbL92o8wdFivY4c7GV3QdfmjkFk7Vm48Uk
// encapsuated to /ip4/192.168.10.34/tcp/19199/p2p/12D3KooWLornEge5BiVbL92o8wdFivY4c7GV3QdfmjkFk7Vm48Uk
func EncapsulatePeerMultiAddr(addr ma.Multiaddr, peerid peer.ID) (ma.Multiaddr, error) {
	pma, err := ma.NewMultiaddr(
		fmt.Sprintf("/%s/%s", ma.ProtocolWithCode(ma.P_P2P).Name, peer.IDB58Encode(peerid)),
	)
	if err != nil {
		return nil, err
	}

	return addr.Encapsulate(pma), nil
}
