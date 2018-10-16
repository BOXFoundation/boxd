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

	"github.com/BOXFoundation/boxd/log"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	"github.com/BOXFoundation/boxd/p2p/pstore"
	"github.com/BOXFoundation/boxd/storage"
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

var logger = log.NewLogger("p2p") // logger

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
	connmgr         *ConnManager
	mu              sync.Mutex
}

var _ Net = (*BoxPeer)(nil) // BoxPeer implements Net interface

// NewBoxPeer create a BoxPeer
func NewBoxPeer(parent goprocess.Process, config *Config, s storage.Storage) (*BoxPeer, error) {
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

	ps, err := pstore.NewDefaultPeerstore(ctx, s)
	if err != nil {
		return nil, err
	}
	boxPeer.connmgr = NewConnManager(ps)

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", config.Address, config.Port)),
		libp2p.Identity(networkIdentity),
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		libp2p.Peerstore(ps),
		libp2p.ConnectionManager(boxPeer.connmgr),
		libp2p.NATPortMap(),
	}

	boxPeer.host, err = libp2p.New(ctx, opts...)
	boxPeer.host.SetStreamHandler(ProtocolID, boxPeer.handleStream)
	boxPeer.table = NewTable(boxPeer)

	fulladdr, _ := PeerMultiAddr(boxPeer.host)
	logger.Infof("BoxPeer is now starting at %s", fulladdr)

	return boxPeer, nil
}

// Run schedules lookup and discover new peer
func (p *BoxPeer) Run() {
	// libp2p conn manager
	p.connmgr.Loop(p.proc)

	if len(p.config.Seeds) > 0 {
		p.connectSeeds()
		p.table.Loop(p.proc)
	}
	p.notifier.Loop(p.proc)
}

func loadNetworkIdentity(filename string) (crypto.PrivKey, error) {
	var key crypto.PrivKey
	if filename == "" {
		key, _, err := crypto.GenerateEd25519Key(rand.Reader)
		return key, err
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) { // file does not exist.
		key, _, err := crypto.GenerateEd25519Key(rand.Reader)
		if err == nil {
			// save privKey to file
			go saveNetworkIdentity(filename, key)
		}
		return key, err
	}

	data, err := ioutil.ReadFile(filename)
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

func saveNetworkIdentity(path string, key crypto.PrivKey) error {
	data, err := crypto.MarshalPrivateKey(key)
	if err != nil {
		return err
	}
	b64data := base64.StdEncoding.EncodeToString(data)
	return ioutil.WriteFile(path, []byte(b64data), 0400)
}

func (p *BoxPeer) handleStream(s libp2pnet.Stream) {
	conn := NewConn(s, p, s.Conn().RemotePeer())
	go conn.loop()
}

func (p *BoxPeer) connectSeeds() {
	for _, v := range p.config.Seeds {
		if err := p.AddAddrToPeerstore(v); err != nil {
			logger.Warn("Failed to add seed to peerstore.", err)
		}
		// conn := NewConn(nil, p, peerID)
		// go conn.loop()
	}
}

// AddAddrToPeerstore adds specified address to peerstore
func (p *BoxPeer) AddAddrToPeerstore(addr string) error {
	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return err
	}
	return p.AddToPeerstore(maddr)
}

// AddToPeerstore adds specified multiaddr to peerstore
func (p *BoxPeer) AddToPeerstore(maddr multiaddr.Multiaddr) error {
	haddr, pid, err := DecapsulatePeerMultiAddr(maddr)
	if err != nil {
		return err
	}

	// TODO, we must consider how long the peer should be in the peerstore,
	// PermanentAddrTTL should only be for peer configured by user.
	// Peer that is connected or observed from other peers should have different TTL.
	p.host.Peerstore().AddAddr(pid, haddr, peerstore.PermanentAddrTTL)
	p.table.routeTable.Update(pid)
	return nil
}

////////// implements Net interface //////////

// Broadcast business message.
func (p *BoxPeer) Broadcast(code uint32, msg conv.Convertible) error {
	body, err := conv.MarshalConvertible(msg)
	if err != nil {
		return err
	}

	for _, v := range p.conns {
		conn := v.(*Conn)
		if p.id.Pretty() == conn.remotePeer.Pretty() {
			continue
		}
		go conn.Write(code, body)
	}
	return nil
}

// SendMessageToPeer send message to a peer.
func (p *BoxPeer) SendMessageToPeer(code uint32, msg conv.Convertible, pid peer.ID) {

}

// Subscribe a message notification.
func (p *BoxPeer) Subscribe(notifiee *Notifiee) {
	p.notifier.Subscribe(notifiee)
}

// UnSubscribe cancel subcribe.
func (p *BoxPeer) UnSubscribe(notifiee *Notifiee) {
	p.notifier.UnSubscribe(notifiee)
}

// Notify publishes a message notification.
func (p *BoxPeer) Notify(msg Message) {
	p.notifier.Notify(msg)
}
