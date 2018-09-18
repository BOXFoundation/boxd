// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/jbenet/goprocess"
	goprocessctx "github.com/jbenet/goprocess/context"
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
	conns           map[peer.ID]interface{}
	config          *Config
	host            host.Host
	proc            goprocess.Process
	id              peer.ID
	table           *Table
	networkIdentity crypto.PrivKey
	notifier        *Notifier
	mu              sync.Mutex
}

// NewBoxPeer create a BoxPeer
func NewBoxPeer(config *Config, parent goprocess.Process) (*BoxPeer, error) {
	// ctx := context.Background()
	proc := goprocess.WithParent(parent) // p2p proc
	ctx := goprocessctx.OnClosingContext(proc)
	boxPeer := &BoxPeer{conns: make(map[peer.ID]interface{}), config: config, notifier: NewNotifier(), proc: proc}
	networkIdentity, err := loadNetworkIdentity(config.KeyPath)
	if err != nil {
		return nil, err
	}
	boxPeer.networkIdentity = networkIdentity
	boxPeer.id, err = peer.IDFromPublicKey(networkIdentity.GetPublic())
	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", config.Address, config.Port)),
		libp2p.Identity(networkIdentity),
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		libp2p.NATPortMap(),
	}

	boxPeer.host, err = libp2p.New(ctx, opts...)
	boxPeer.host.SetStreamHandler(ProtocolID, boxPeer.handleStream)
	boxPeer.table = NewTable(boxPeer)
	logger.Infof("BoxPeer starting...ID: %s listen: %s", boxPeer.id.Pretty(), fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", config.Port))
	return boxPeer, nil
}

// Bootstrap schedules lookup and discover new peer
func (p *BoxPeer) Bootstrap() {
	if len(p.config.Seeds) > 0 {
		p.ConnectSeeds()
		p.table.Loop(p.proc)
	}
	p.notifier.Loop(p.proc)
}

func loadNetworkIdentity(path string) (crypto.PrivKey, error) {
	var key crypto.PrivKey
	if path == "" {
		key, _, err := crypto.GenerateEd25519Key(rand.Reader)
		return key, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) { // file does not exist.
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
	conn := NewConn(s, p, s.Conn().RemotePeer())
	go conn.loop()
}

// ConnectSeeds connect the seeds in config
func (p *BoxPeer) ConnectSeeds() {
	host := p.host
	for _, v := range p.config.Seeds {
		if err := p.addAddrToPeerstore(host, v); err != nil {
			logger.Warn("Failed to add seed to peerstore.", err)
		}
		// conn := NewConn(nil, p, peerID)
		// go conn.loop()
	}
}

func (p *BoxPeer) addAddrToPeerstore(h host.Host, addr string) error {
	ipfsaddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return err
	}
	pid, err := ipfsaddr.ValueForProtocol(multiaddr.P_IPFS)
	if err != nil {
		return err
	}

	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		return err
	}
	targetPeerAddr, _ := multiaddr.NewMultiaddr(
		fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid)))
	targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)

	h.Peerstore().AddAddr(peerid, targetAddr, peerstore.PermanentAddrTTL)
	p.table.routeTable.Update(peerid)
	return nil
}
