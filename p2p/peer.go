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

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/log"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	"github.com/BOXFoundation/boxd/p2p/pstore"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
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

var (
	logger = log.NewLogger("p2p")

	isSynced = false
)

// BoxPeer represents a connected remote node.
type BoxPeer struct {
	conns           *sync.Map
	config          *Config
	host            host.Host
	proc            goprocess.Process
	id              peer.ID
	table           *Table
	networkIdentity crypto.PrivKey
	notifier        *Notifier
	connmgr         *ConnManager
	scoremgr        *ScoreManager
	addrbook        service.Server
	bus             eventbus.Bus
}

var _ Net = (*BoxPeer)(nil) // BoxPeer implements Net interface

// NewBoxPeer create a BoxPeer
func NewBoxPeer(parent goprocess.Process, config *Config, s storage.Storage, bus eventbus.Bus) (*BoxPeer, error) {

	proc := goprocess.WithParent(parent) // p2p proc
	ctx := goprocessctx.OnClosingContext(proc)
	boxPeer := &BoxPeer{conns: new(sync.Map), config: config, notifier: NewNotifier(), proc: proc, bus: bus}
	networkIdentity, err := loadNetworkIdentity(config.KeyPath)
	if err != nil {
		return nil, err
	}
	boxPeer.networkIdentity = networkIdentity
	boxPeer.id, err = peer.IDFromPublicKey(networkIdentity.GetPublic())
	if err != nil {
		return nil, err
	}

	addrbook, err := pstore.NewDefaultAddrBook(proc, s, bus)
	if err != nil {
		return nil, err
	}
	boxPeer.addrbook = addrbook.(service.Server)

	ps, err := pstore.NewDefaultPeerstoreWithAddrBook(proc, s, addrbook)
	if err != nil {
		return nil, err
	}
	boxPeer.connmgr = NewConnManager(ps)
	boxPeer.scoremgr = NewScoreManager(proc, bus, boxPeer)

	// seed peer never sync
	isSynced = len(config.Seeds) == 0

	opts := []libp2p.Option{
		// TODO: to support ipv6
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

// load network identity from local filesystem or create a new one.
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

// save network identity
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
	conn.Loop(p.proc)
}

// implement interface service.Server
var _ service.Server = (*BoxPeer)(nil)

// Run schedules lookup and discover new peer
func (p *BoxPeer) Run() error {
	// libp2p conn manager
	p.connmgr.Loop(p.proc)
	p.addrbook.Run()

	if len(p.config.Seeds) > 0 {
		p.connectSeeds()
		p.table.Loop(p.proc)
	}
	p.notifier.Loop(p.proc)

	return nil
}

// Proc returns the gopreocess of database
func (p *BoxPeer) Proc() goprocess.Process {
	return p.proc
}

// Stop box peer service
func (p *BoxPeer) Stop() {
	p.proc.Close()
}

func (p *BoxPeer) connectSeeds() {
	for _, v := range p.config.Seeds {
		if err := p.AddAddrToPeerstore(v); err != nil {
			logger.Warn("Failed to add seed to peerstore.", err)
		}
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

	p.conns.Range(func(k, v interface{}) bool {
		conn := v.(*Conn)
		if p.id.Pretty() == conn.remotePeer.Pretty() {
			return true
		}
		go conn.Write(code, body)
		return true
	})
	return nil
}

// BroadcastToMiners business message to miners.
func (p *BoxPeer) BroadcastToMiners(code uint32, msg conv.Convertible, miners []string) error {

	body, err := conv.MarshalConvertible(msg)
	if err != nil {
		return err
	}
	for _, v := range miners {
		if p.id.Pretty() == v {
			continue
		}
		pid, err := peer.IDB58Decode(v)
		if err != nil {
			return err
		}
		if c, ok := p.conns.Load(pid); ok {
			conn := c.(*Conn)
			go conn.Write(code, body)
		}
	}
	return nil
}

// SendMessageToPeer sends message to a peer.
func (p *BoxPeer) SendMessageToPeer(code uint32, msg conv.Convertible, pid peer.ID) error {

	body, err := conv.MarshalConvertible(msg)
	if err != nil {
		return err
	}
	if c, ok := p.conns.Load(pid); ok {
		conn := c.(*Conn)
		if p.id.Pretty() == conn.remotePeer.Pretty() {
			return ErrFailedToSendMessageToPeer
		}
		go conn.Write(code, body)
		return nil
	}
	return ErrFailedToSendMessageToPeer
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

// Conns return peer connections.
func (p *BoxPeer) Conns() *sync.Map {
	return p.conns
}

// PickOnePeer picks a peer not in peersExclusive and return its id
func (p *BoxPeer) PickOnePeer(peersExclusive ...peer.ID) peer.ID {
	var pid peer.ID
	p.conns.Range(func(k, v interface{}) bool {
		if !util.InArray(k, peersExclusive) {
			pid = k.(peer.ID)
			return false
		}
		return true
	})
	return pid
}

// PeerSynced get sync states of remote peers
func (p *BoxPeer) PeerSynced(peerID peer.ID) (bool, bool) {
	val, ok := p.conns.Load(peerID)
	return val.(*Conn).isSynced, ok
}

// UpdateSynced update peers' isSynced
func UpdateSynced(synced bool) {
	isSynced = synced
}
