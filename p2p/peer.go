/*
 * Copyright (C) 2018 ContentBox Authors
 * This file is part of The go-contentbox library.
 *
 * The go-contentbox library is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The go-contentbox library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The go-contentbox library.  If not, see <http://www.gnu.org/licenses/>.
 */

package p2p

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"sync"

	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	libp2pnet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	multiaddr "github.com/multiformats/go-multiaddr"
)

// BoxPeer represents a connected remote node.
type BoxPeer struct {
	conns   map[string]interface{}
	config  *Config
	host    host.Host
	context context.Context
	id      peer.ID
	table   *Table
	mu      sync.Mutex
}

// New create a BoxPeer
func New(config *Config) (*BoxPeer, error) {
	ctx := context.Background()
	boxPeer := &BoxPeer{conns: make(map[string]interface{}), config: config, context: ctx, table: NewTable()}
	networkIdentity, err := loadNetworkIdentity(config.KeyPath)
	if err != nil {
		return nil, err
	}
	boxPeer.id, err = peer.IDFromPublicKey(networkIdentity.GetPublic())
	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", config.Port)),
		libp2p.Identity(networkIdentity),
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		libp2p.NATPortMap(),
	}

	boxPeer.host, err = libp2p.New(ctx, opts...)
	boxPeer.host.SetStreamHandler(ProtocolID, boxPeer.handleStream)

	return boxPeer, nil
}

// Bootstrap schedules lookup and discover new peer
func (p *BoxPeer) Bootstrap() {
	p.ConnectSeeds()
	p.table.Loop()
}

func loadNetworkIdentity(path string) (crypto.PrivKey, error) {
	var key crypto.PrivKey
	if path == "" {
		key, _, err := crypto.GenerateEd25519Key(rand.Reader)
		return key, err
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decodeData, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, err
	}
	key, err = crypto.UnmarshalPrivateKey(decodeData)

	return key, err
}

func (p *BoxPeer) handleStream(s libp2pnet.Stream) {
	p.mu.Lock()
	defer p.mu.Unlock()
	conn := NewConn(s, p)
	if p.conns[s.Conn().RemotePeer().String()] != nil {
		old, _ := p.conns[s.Conn().RemotePeer().String()].(Conn)
		old.stream.Close()
	}
	p.conns[s.Conn().RemotePeer().String()] = conn
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
	go conn.readData(rw)
	go conn.writeData(rw)
}

// ConnectSeeds connect the seeds in config
func (p *BoxPeer) ConnectSeeds() {
	host := p.host
	for _, v := range p.config.Seeds {
		peerID, err := addAddrToPeerstore(host, v)
		if err != nil {
			logger.Warn("Failed to add seed to peerstore.")
		}
		s, err := host.NewStream(context.Background(), peerID, ProtocolID)
		if err != nil {
			logger.Warn("Failed to new stream to seed.")
		}
		p.handleStream(s)
	}
	if len(p.conns) == 0 {
		logger.Fatal("Failed to connect seeds.")
	}
}

func addAddrToPeerstore(h host.Host, addr string) (peer.ID, error) {
	ipfsaddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return "", err
	}
	pid, err := ipfsaddr.ValueForProtocol(multiaddr.P_IPFS)
	if err != nil {
		return "", err
	}

	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		return "", err
	}
	targetPeerAddr, _ := multiaddr.NewMultiaddr(
		fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid)))
	targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)

	h.Peerstore().AddAddr(peerid, targetAddr, peerstore.PermanentAddrTTL)
	return peerid, nil
}
