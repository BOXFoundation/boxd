// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"bufio"
	"errors"
	"hash/crc32"
	"time"

	"github.com/BOXFoundation/boxd/p2p/pb"
	proto "github.com/gogo/protobuf/proto"
	"github.com/jbenet/goprocess"
	goprocessctx "github.com/jbenet/goprocess/context"
	libp2pnet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
)

// const
const (
	PeriodTime = 5
)

// Conn represents a connection to a remote node
type Conn struct {
	stream             libp2pnet.Stream
	peer               *BoxPeer
	remotePeer         peer.ID
	establish          bool
	establishSucceedCh chan bool
	proc               goprocess.Process
}

// NewConn create a stream to remote peer.
func NewConn(stream libp2pnet.Stream, peer *BoxPeer, peerID peer.ID) *Conn {
	return &Conn{
		stream:             stream,
		peer:               peer,
		remotePeer:         peerID,
		establish:          false,
		establishSucceedCh: make(chan bool, 1),
		proc:               goprocess.WithParent(peer.proc),
	}
}

func (conn *Conn) loop() {
	if conn.stream == nil {
		ctx := goprocessctx.OnClosingContext(conn.peer.proc)
		s, err := conn.peer.host.NewStream(ctx, conn.remotePeer, ProtocolID)
		if err != nil {
			logger.Errorf("Failed to new stream to %s, err = %s", conn.remotePeer.Pretty(), err.Error())
			return
		}
		conn.stream = s
		if err := conn.Ping(); err != nil {
			logger.Errorf("Failed to ping peer %s, err = %s", conn.remotePeer.Pretty(), err.Error())
			return
		}
	}

	var r = bufio.NewReader(conn.stream)
	for {
		msg, err := conn.readMessage(r)
		if err != nil {
			conn.Close()
			return
		}
		if err := conn.checkMessage(msg); err != nil {
			logger.Error("Invalid message. ", err)
			conn.Close()
			return
		}
		if err := conn.handle(msg); err != nil {
			logger.Error("Failed to handle message. ", err)
			conn.Close()
			return
		}
	}
}

// readMessage returns the next message, with remote peer id
func (conn *Conn) readMessage(r *bufio.Reader) (*remoteMessage, error) {
	msg, err := readMessageData(r)
	if err != nil {
		return nil, err
	}
	return &remoteMessage{message: msg, from: conn.remotePeer}, nil
}

func (conn *Conn) handle(msg *remoteMessage) error {
	// handle handshake messages
	switch msg.code {
	case Ping:
		return conn.onPing(msg.body)
	case Pong:
		return conn.onPong(msg.body)
	}
	if !conn.establish {
		// return error in case no handshake with remote peer
		return ErrNoConnectionEstablished
	}

	// handle discovery messages
	switch msg.code {
	case PeerDiscover:
		return conn.OnPeerDiscover(msg.body)
	case PeerDiscoverReply:
		return conn.OnPeerDiscoverReply(msg.body)
	default:
		// others, notify its subscriber
		conn.peer.notifier.Notify(msg)
	}
	return nil
}

func (conn *Conn) heartBeatService() {
	t := time.NewTicker(time.Second * PeriodTime)
	for {
		select {
		case <-t.C:
			conn.Ping()
		case <-conn.proc.Closing():
			t.Stop()
			break
		}
	}
}

// Ping the target node
func (conn *Conn) Ping() error {
	body := []byte("ping")
	return conn.Write(Ping, body)
}

func (conn *Conn) onPing(data []byte) error {
	if "ping" != string(data) {
		return ErrMessageDataContent
	}
	body := []byte("pong")
	if !conn.establish {
		conn.established()
	}
	return conn.Write(Pong, body)
}

func (conn *Conn) onPong(data []byte) error {
	if "pong" != string(data) {
		return ErrMessageDataContent
	}
	if !conn.establish {
		conn.established()
		go conn.heartBeatService()
	}

	return nil
}

// PeerDiscover discover new peers from remoute peer.
func (conn *Conn) PeerDiscover() error {
	if !conn.establish {
		establishedTimeout := time.NewTicker(30 * time.Second)
		select {
		case <-conn.establishSucceedCh:
		case <-establishedTimeout.C:
			conn.Close()
			return errors.New("Handshaking timeout")
		}
	}
	return conn.Write(PeerDiscover, []byte{})
}

// OnPeerDiscover handle PeerDiscover message.
func (conn *Conn) OnPeerDiscover(body []byte) error {
	// get random peers from routeTable
	peers := conn.peer.table.GetRandomPeers(conn.stream.Conn().LocalPeer())
	msg := &p2ppb.Peers{Peers: make([]*p2ppb.PeerInfo, len(peers))}

	for i, v := range peers {
		peerInfo := &p2ppb.PeerInfo{
			Id:    v.ID.Pretty(),
			Addrs: []string{}[:],
		}
		for _, addr := range v.Addrs {
			peerInfo.Addrs = append(peerInfo.Addrs, addr.String())
		}
		msg.Peers[i] = peerInfo
	}
	body, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.Write(PeerDiscoverReply, body)
}

// OnPeerDiscoverReply handle PeerDiscoverReply message.
func (conn *Conn) OnPeerDiscoverReply(body []byte) error {
	peers := new(p2ppb.Peers)
	if err := proto.Unmarshal(body, peers); err != nil {
		logger.Error("Failed to unmarshal PeerDiscoverReply message.")
		return err
	}

	conn.peer.table.AddPeers(conn, peers)
	return nil
}

func (conn *Conn) Write(opcode uint32, body []byte) error {
	data, err := newMessageData(conn.peer.config.Magic, opcode, nil, body).Marshal()
	if err != nil {
		return err
	}
	_, err = conn.stream.Write(data)
	return err // error or nil
}

// Close connection to remote peer.
func (conn *Conn) Close() {
	if conn.stream != nil {
		delete(conn.peer.conns, conn.remotePeer)
		conn.peer.table.peerStore.ClearAddrs(conn.remotePeer)
		conn.stream.Close()
	}
}

func (conn *Conn) checkMessage(msg *remoteMessage) error {
	return checkMessage(conn.peer.config.Magic, msg)
}

func (conn *Conn) established() {
	// TODO: need mutex?
	conn.peer.table.AddPeerToTable(conn)
	conn.establish = true
	conn.establishSucceedCh <- true
	conn.peer.conns[conn.remotePeer] = conn
	logger.Info("Succed to established with peer ", conn.remotePeer.Pretty())
}

// checkMessage checks whether the message data is valid
func checkMessage(magic uint32, msg *remoteMessage) error {
	if magic != msg.magic {
		return ErrMagic
	}
	if msg.dataLength > MaxMessageDataLength {
		return ErrExceedMaxDataLength
	}

	expectedDataCheckSum := crc32.ChecksumIEEE(msg.body)
	if expectedDataCheckSum != msg.dataChecksum {
		return ErrBodyCheckSum
	}
	return nil
}
