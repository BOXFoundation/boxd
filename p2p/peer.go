/*
 * Copyright (C) 2018 The go-contentbox Authors
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
)

// BoxPeer represents a connected remote node.
type BoxPeer struct {
	conns   *sync.Map
	config  *Config
	host    host.Host
	context context.Context
	id      peer.ID
	table   *Table
}

// New create a BoxPeer
func New(config *Config) (*BoxPeer, error) {
	ctx := context.Background()
	boxPeer := &BoxPeer{conns: new(sync.Map), config: config, context: ctx, table: NewTable()}
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
	conn := NewConn(s)
	if v, ok := p.conns.Load(s.Conn().RemotePeer()); ok {
		old, _ := v.(Conn)
		p.conns.Delete(s.Conn().RemotePeer())
		old.stream.Close()
	}
	p.conns.Store(s.Conn().RemotePeer(), conn)
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
	go conn.readData(rw)
	go conn.writeData(rw)
}
