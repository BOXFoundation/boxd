// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"hash/crc64"
	"io/ioutil"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core/types"
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
	logger     = log.NewLogger("p2p")
	crc64Table = crc64.MakeTable(crc64.ECMA)

	isSynced = false

	ipRegex = "(2(5[0-5]{1}|[0-4]\\d{1})|[0-1]?\\d{1,2})(\\.(2(5[0-5]{1}|[0-4]\\d{1})|[0-1]?\\d{1,2})){3}"
)

// BoxPeer represents a connected remote node.
type BoxPeer struct {
	peertype        pstore.PeerType
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
	minerreader     service.MinerReader
}

var _ Net = (*BoxPeer)(nil) // BoxPeer implements Net interface

// NewBoxPeer create a BoxPeer
func NewBoxPeer(parent goprocess.Process, config *Config, s storage.Storage, bus eventbus.Bus) (*BoxPeer, error) {

	proc := goprocess.WithParent(parent) // p2p proc
	ctx := goprocessctx.OnClosingContext(proc)
	boxPeer := &BoxPeer{
		conns:    new(sync.Map),
		config:   config,
		notifier: NewNotifier(),
		proc:     proc,
		bus:      bus,
	}
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
	if !conn.KeepConn() {
		s.Close()
		logger.Warnf("%s, peer.ID: %s", ErrNoConnectPermission.Error(), conn.remotePeer.Pretty())
		return
	}
	ptype, _ := p.Type(conn.remotePeer)
	err := pstore.UpdateType(conn.remotePeer, uint32(ptype))
	if err != nil {
		logger.Error(err)
	}
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
	go p.Metrics(p.proc)
	p.notifier.Loop(p.proc)

	return nil
}

// Metrics is used to measure data
func (p *BoxPeer) Metrics(proc goprocess.Process) {

	ticker := time.NewTicker(metricsLoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-proc.Closing():
			logger.Info("Quit peer metrics.")
			return
		case <-ticker.C:
			var len int64
			p.conns.Range(func(k, v interface{}) bool {
				len++
				return true
			})
			metricsConnGauge.Update(len)
		}
	}

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

	ptype, _ := p.Type(pid)
	p.table.peerStore.Put(pid, pstore.PTypeSuf, uint8(ptype))
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
		go func(conn *Conn) {
			if err := conn.Write(code, body); err != nil {
				logger.Errorf("Failed to broadcast message to remote peer.Code: %X, Err: %v", code, err)
			}
		}(conn)
		return true
	})
	return nil
}

// BroadcastToBookkeepers business message to bookkeepers.
func (p *BoxPeer) BroadcastToBookkeepers(code uint32, msg conv.Convertible, bookkeepers []string) error {

	body, err := conv.MarshalConvertible(msg)
	if err != nil {
		return err
	}
	for _, v := range bookkeepers {
		if p.id.Pretty() == v {
			continue
		}
		pid, err := peer.IDB58Decode(v)
		if err != nil {
			return err
		}
		if c, ok := p.conns.Load(pid); ok {
			conn := c.(*Conn)
			// go conn.Write(code, body)
			go func(conn *Conn) {
				if err := conn.Write(code, body); err != nil {
					logger.Errorf("Failed to broadcast message to remote bookkeepers peer.Code: %X, Err: %v", code, err)
				}
			}(conn)
		}
	}
	return nil
}

// Relay business message.
func (p *BoxPeer) Relay(code uint32, msg conv.Convertible) error {

	body, err := conv.MarshalConvertible(msg)
	if err != nil {
		return err
	}

	cnt := 0
	p.conns.Range(func(k, v interface{}) bool {
		conn := v.(*Conn)
		if uint32(cnt) >= p.config.RelaySize {
			return false
		}
		// go connTmp.Write(code, body)
		go func(conn *Conn) {
			if err := conn.Write(code, body); err != nil {
				logger.Errorf("Failed to relay message to remote peer.Code: %X, Err: %v", code, err)
			}
		}(conn)
		cnt++
		return true
	})
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
		// go conn.Write(code, body)
		go func(conn *Conn) {
			if err := conn.Write(code, body); err != nil {
				logger.Errorf("Failed to send message to remote peer.Code: %X, Err: %v", code, err)
			}
		}(conn)
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
		for _, p := range peersExclusive {
			if k.(peer.ID) == p {
				return true
			}
		}
		pid = k.(peer.ID)
		return false

	})
	return pid
}

// PeerSynced get sync states of remote peers.
func (p *BoxPeer) PeerSynced(peerID peer.ID) (bool, bool) {
	val, ok := p.conns.Load(peerID)
	return val.(*Conn).isSynced, ok
}

// SetMinerReader set minerreader.
func (p *BoxPeer) SetMinerReader(mr service.MinerReader) {
	p.minerreader = mr
}

// Type return returns the type corresponding to the peer id.
// The second return value indicates skepticism about the result.
func (p *BoxPeer) Type(pid peer.ID) (pstore.PeerType, bool) {
	for _, p := range agents {
		if pid == p {
			return pstore.ServerPeer, true
		}
	}
	// Need to init minerreader.
	if p.minerreader == nil {
		return pstore.UnknownPeer, false
	}
	miners, canmint := p.minerreader.Miners()

	pretty := pid.Pretty()
	if util.InStrings(pretty, miners) {
		return pstore.MinerPeer, true
	}

	candidates, _ := p.minerreader.Candidates()
	if util.InStrings(pretty, candidates) {
		return pstore.CandidatePeer, true
	}

	if len(miners) != 0 || (!canmint && len(miners) == 0) {
		if pretty == p.id.Pretty() {
			if len(principals) == 0 {
				return pstore.LayfolkPeer, true
			}
			return pstore.ServerPeer, true
		}
	}

	return pstore.LayfolkPeer, false
}

// ConnectingPeers return all connecting peers' ids.
func (p *BoxPeer) ConnectingPeers() []string {
	pids := []string{}
	p.conns.Range(func(k, v interface{}) bool {
		pids = append(pids, k.(peer.ID).Pretty())
		return true
	})
	return pids
}

// PeerID return peer.ID.
func (p *BoxPeer) PeerID() string {
	return p.id.Pretty()
}

// Miners return miners and ips.
func (p *BoxPeer) Miners() []*types.PeerInfo {
	miners, _ := p.minerreader.Miners()
	re, _ := regexp.Compile(ipRegex)

	infos := []*types.PeerInfo{}
	for _, m := range miners {
		pid, err := peer.IDB58Decode(m)
		if err != nil {
			logger.Errorf("IDFromString failed, Err: %v, m: %s", err, m)
			continue
		}
		info := p.table.peerStore.PeerInfo(pid)
		iplist := []string{}
		for _, ip := range info.Addrs {
			ipp := re.Find([]byte(ip.String()))
			if len(ipp) == 0 {
				continue
			}
			iplist = append(iplist, string(ipp))
		}

		pinfo := &types.PeerInfo{
			ID:     m,
			Iplist: iplist,
		}
		infos = append(infos, pinfo)
	}
	return infos
}

// ReorgConns reorg conns by pids.
func (p *BoxPeer) ReorgConns() uint8 {
	peertype, nodoubt := p.Type(p.id)
	if !nodoubt || (peertype != pstore.MinerPeer && peertype != pstore.CandidatePeer) {
		return 0
	}
	close := uint8(0)
	p.conns.Range(func(k, v interface{}) bool {
		conn := v.(*Conn)
		remotetype, _ := p.Type(conn.remotePeer)
		if !util.InArray(remotetype, []pstore.PeerType{pstore.MinerPeer, pstore.CandidatePeer, pstore.ServerPeer}) {
			if err := conn.Close(); err != nil {
				logger.Errorf("Conn close failed. Err: %v", err)
			} else {
				close++
			}
		}
		return true
	})
	return close
}

// UpdateSynced update peers' isSynced
func UpdateSynced(synced bool) {
	isSynced = synced
}
