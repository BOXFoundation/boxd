// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"errors"
	"hash/crc32"
	"time"

	"github.com/BOXFoundation/Quicksilver/p2p/pb"
	"github.com/BOXFoundation/Quicksilver/util"
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

// error defined
var (
	ErrMagic                   = errors.New("magic is error")
	ErrHeaderCheckSum          = errors.New("header checksum is error")
	ErrExceedMaxDataLength     = errors.New("exceed max data length")
	ErrBodyCheckSum            = errors.New("body checksum is error")
	ErrMessageDataContent      = errors.New("Invalid message data content")
	ErrNoConnectionEstablished = errors.New("No connection established")
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
			return
		}
		conn.stream = s
		if err := conn.Ping(); err != nil {
			return
		}
	}

	buf := make([]byte, 1024)
	messageBuffer := make([]byte, 0)
	var header *MessageHeader
	for {
		n, err := conn.stream.Read(buf)
		if err != nil {
			conn.Close()
			return
		}
		messageBuffer = append(messageBuffer, buf[:n]...)
		for {
			if header == nil {
				var err error
				if len(messageBuffer) < FixHeaderLength {
					break
				}
				headerLength := util.Uint32(messageBuffer[:FixHeaderLength])
				if len(messageBuffer) < FixHeaderLength+int(headerLength) {
					break
				}
				headerBytes := messageBuffer[FixHeaderLength : headerLength+FixHeaderLength]
				header, err = ParseHeader(headerBytes)
				if err != nil {
					conn.Close()
					return
				}
				if err := conn.checkHeader(header); err != nil {
					logger.Error("Invalid message header. ", err)
					conn.Close()
					return
				}
				messageBuffer = messageBuffer[FixHeaderLength+int(headerLength):]
			}
			if len(messageBuffer) < int(header.DataLength) {
				break
			}
			body := messageBuffer[:header.DataLength]
			if err := conn.checkBody(header, body); err != nil {
				logger.Error("Invalid message body. ", err)
				conn.Close()
				return
			}
			messageBuffer = messageBuffer[header.DataLength:]
			if err := conn.handle(header.Code, body); err != nil {
				logger.Error("Failed to handle message. ", err)
				conn.Close()
				return
			}
			header = nil
		}
	}

}

func (conn *Conn) handle(messageCode uint32, body []byte) error {

	switch messageCode {
	case Ping:
		return conn.onPing(body)
	case Pong:
		return conn.onPong(body)
	}
	if !conn.establish {
		return ErrNoConnectionEstablished
	}

	switch messageCode {
	case PeerDiscover:
		return conn.OnPeerDiscover(body)
	case PeerDiscoverReply:
		return conn.OnPeerDiscoverReply(body)
	}
	return nil
}

func (conn *Conn) buildMessageHeader(code uint32, body []byte, reserved []byte) *MessageHeader {
	header := &MessageHeader{}
	header.Magic = conn.peer.config.Magic
	header.Code = code
	header.DataLength = uint32(len(body))
	header.DataChecksum = crc32.ChecksumIEEE(body)
	header.Reserved = reserved
	return header
}

func (conn *Conn) heartBeatService() {

	t := time.NewTicker(time.Second * PeriodTime)
	for {
		select {
		case <-t.C:
			logger.Info("do heartbeat...")
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
	logger.Info("receive ping message.")
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
	logger.Info("reveive pong message.")
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
			logger.Error("Handshaking timeout")
			conn.Close()
			return errors.New("Handshaking timeout")
		}
	}
	return conn.Write(PeerDiscover, []byte{})
}

// OnPeerDiscover handle PeerDiscover message.
func (conn *Conn) OnPeerDiscover(body []byte) error {
	logger.Info("receive peer discover message.")
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
	logger.Info("receive peer discover reply message.")
	peers := new(p2ppb.Peers)
	if err := proto.Unmarshal(body, peers); err != nil {
		logger.Error("Failed to unmarshal PeerDiscoverReply message.")
		return err
	}
	conn.peer.table.AddPeers(conn, peers)
	return nil
}

func (conn *Conn) Write(OpCode uint32, body []byte) error {

	header := conn.buildMessageHeader(OpCode, body, nil)
	msg, err := NewMessage(header, body)
	if err != nil {
		return err
	}
	_, err = conn.stream.Write(msg)
	if err != nil {
		return err
	}
	return nil
}

// Close connection to remote peer.
func (conn *Conn) Close() {
	delete(conn.peer.conns, conn.stream.Conn().RemotePeer())
	conn.stream.Close()
}

func (conn *Conn) checkHeader(header *MessageHeader) error {

	if conn.peer.config.Magic != header.Magic {
		return ErrMagic
	}
	if header.DataLength > MaxNebMessageDataLength {
		return ErrExceedMaxDataLength
	}
	return nil
}

func (conn *Conn) checkBody(header *MessageHeader, body []byte) error {

	expectedDataCheckSum := crc32.ChecksumIEEE(body)
	if expectedDataCheckSum != header.DataChecksum {
		return ErrBodyCheckSum
	}
	return nil
}

func (conn *Conn) established() {
	// TODO: need mutex?
	conn.peer.table.AddPeerToTable(conn)
	conn.establish = true
	conn.establishSucceedCh <- true
	conn.peer.conns[conn.remotePeer] = conn
	logger.Info("Succed to established with peer ", conn.remotePeer.Pretty())
}
