// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"errors"
	"hash/crc32"
	"io"
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/p2p/pb"
	pq "github.com/BOXFoundation/boxd/p2p/priorityqueue"
	proto "github.com/gogo/protobuf/proto"
	"github.com/jbenet/goprocess"
	goprocessctx "github.com/jbenet/goprocess/context"
	libp2pnet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/whyrusleeping/yamux"
)

// const
const (
	PeriodTime = 2 * 60

	PingBody = "ping"
	PongBody = "pong"

	// [Low, Mid, High, Top]
	PriorityMsgTypeSize = 4
	PriorityQueueCap    = 1024
)

// Conn represents a connection to a remote node
type Conn struct {
	stream             libp2pnet.Stream
	peer               *BoxPeer
	remotePeer         peer.ID
	isEstablished      bool
	isSynced           bool
	establishSucceedCh chan bool
	pq                 *pq.PriorityMsgQueue
	proc               goprocess.Process
	procHeartbeat      goprocess.Process
	mutex              sync.Mutex
}

// NewConn create a stream to remote peer.
func NewConn(stream libp2pnet.Stream, peer *BoxPeer, peerID peer.ID) *Conn {
	return &Conn{
		stream:             stream,
		peer:               peer,
		remotePeer:         peerID,
		pq:                 pq.New(PriorityMsgTypeSize, PriorityQueueCap),
		isEstablished:      false,
		isSynced:           false,
		establishSucceedCh: make(chan bool, 1),
	}
}

// Loop start
func (conn *Conn) Loop(parent goprocess.Process) {
	conn.mutex.Lock()
	if conn.proc == nil {
		conn.proc = goprocess.WithParent(parent)
		conn.proc.Go(conn.loop).SetTeardown(conn.Close)

		go conn.pq.Run(conn.proc, func(i interface{}) {
			data := i.([]byte)
			if _, err := conn.stream.Write(data); err != nil {
				logger.Errorf("Failed to write message to %v, %v. ", conn.peer.id.Pretty(), err)
			} else {
				metricsWriteMeter.Mark(int64(len(data) / 8))
			}
		})
	}
	conn.mutex.Unlock()
}

func (conn *Conn) loop(proc goprocess.Process) {
	if conn.stream == nil {
		ctx := goprocessctx.OnClosingContext(proc)
		s, err := conn.peer.host.NewStream(ctx, conn.remotePeer, ProtocolID)
		if err != nil {
			logger.Errorf("Failed to new stream to %s, addrs=%v, err = %s", conn.remotePeer.Pretty(), conn.peer.table.peerStore.PeerInfo(conn.remotePeer), err.Error())
			return
		}
		conn.stream = s
		if err := conn.Ping(); err != nil {
			logger.Errorf("Failed to ping peer %s, err = %s", conn.remotePeer.Pretty(), err.Error())
			return
		}
	}

	defer logger.Debug("Quit conn message loop with ", conn.remotePeer.Pretty())
	for {
		select {
		case <-proc.Closing():
			logger.Debug("Closing connection with peer ", conn.remotePeer.Pretty())
			return
		default:
		}

		msg, err := conn.readMessage(conn.stream)
		if err != nil {
			if err == yamux.ErrConnectionReset {
				logger.Warnf("ReadMessage occurs error. Err: %s", err.Error())
			} else if err == ErrDuplicateMessage {
				continue
			} else {
				logger.Errorf("ReadMessage occurs error. Err: %s", err.Error())
			}
			return
		}
		//logger.Debugf("Receiving message %02x from peer %s", msg.Code(), conn.remotePeer.Pretty())
		if err := conn.Handle(msg); err != nil {
			logger.Error("Failed to handle message. ", err)
			return
		}
	}
}

// readMessage returns the next message, with remote peer id
func (conn *Conn) readMessage(r io.Reader) (*remoteMessage, error) {
	msg, err := readMessageData(r)
	if err != nil {
		return nil, err
	}

	if err := conn.checkMessage(msg); err != nil {
		return nil, err
	}

	// filter out the duplicate messages.
	attr := msgToAttribute[msg.code]
	if attr == nil {
		attr = defaultMessageAttribute
	}
	if !attr.duplicateFilter(msg.body, conn.peer.id, attr.frequency) {
		return nil, ErrDuplicateMessage
	}

	reserved := msg.reserved
	if len(reserved) != 0 {
		if int(reserved[0])&compressFlag != 0 {
			data, err := decompress(nil, msg.body)
			if err != nil {
				return nil, err
			}
			msg.body = data
		}
		if attr.relay {
			attr.relayCache.Add(crc32.ChecksumIEEE(msg.body), int(reserved[0])&relayFlag)
		}
	}

	metricsReadMeter.Mark(msg.Len())
	if err != nil {
		return nil, err
	}
	return &remoteMessage{message: msg, from: conn.remotePeer}, nil
}

// Handle is called on loop
func (conn *Conn) Handle(msg *remoteMessage) error {
	// handle handshake messages
	switch msg.code {
	case Ping:
		return conn.OnPing(msg.body)
	case Pong:
		return conn.OnPong(msg.body)
	}
	if !conn.Established() {
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

func (conn *Conn) heartBeatService(p goprocess.Process) {
	t := time.NewTicker(time.Second * PeriodTime)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if err := conn.Ping(); err != nil {
				logger.Errorf("Failed to ping peer. PeerID: %s", conn.remotePeer.Pretty())
			}
		case <-p.Closing():
			logger.Debug("closing heart beat service with ", conn.remotePeer.Pretty())
			return
		}
	}
}

// Ping the target node
func (conn *Conn) Ping() error {
	return conn.Write(Ping, []byte(PingBody))
}

// OnPing respond the ping message
func (conn *Conn) OnPing(data []byte) error {
	if PingBody != string(data) {
		return ErrMessageDataContent
	}

	conn.peer.bus.Publish(eventbus.TopicConnEvent, conn.remotePeer, eventbus.HeartBeatEvent)
	conn.Establish() // establish connection

	return conn.Write(Pong, []byte(PongBody))
}

// OnPong respond the pong message
func (conn *Conn) OnPong(data []byte) error {
	if PongBody != string(data) {
		return ErrMessageDataContent
	}
	conn.peer.bus.Publish(eventbus.TopicConnEvent, conn.remotePeer, eventbus.HeartBeatEvent)
	if !conn.Establish() {
		conn.mutex.Lock()
		if conn.procHeartbeat == nil {
			conn.procHeartbeat = conn.proc.Go(conn.heartBeatService)
		}
		conn.mutex.Unlock()
	}

	return nil
}

// PeerDiscover discover new peers from remoute peer.
// TODO: we should discover other peers periodly via randomly
// selected remote active peers. Now we only send peer discovery
// msg once after connections is established.
func (conn *Conn) PeerDiscover() error {
	if !conn.Established() {
		establishedTimeout := time.NewTicker(30 * time.Second)
		defer establishedTimeout.Stop()

		select {
		case <-conn.establishSucceedCh:
		case <-establishedTimeout.C:
			conn.peer.bus.Publish(eventbus.TopicConnEvent, conn.peer.id, eventbus.ConnTimeOutEvent)
			conn.proc.Close()
			return errors.New("Handshaking timeout")
		}
	}
	return conn.Write(PeerDiscover, []byte{})
}

// OnPeerDiscover handle PeerDiscover message.
func (conn *Conn) OnPeerDiscover(body []byte) error {
	// get random peers from routeTable
	peers := conn.peer.table.GetRandomPeers(conn.stream.Conn().LocalPeer())
	msg := &p2ppb.Peers{Peers: make([]*p2ppb.PeerInfo, len(peers)), IsSynced: isSynced}

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
		logger.Errorf("[OnPeerDiscover]Failed to handle PeerDiscover message. Err: %s", err.Error())
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
	conn.isSynced = peers.IsSynced
	conn.peer.table.AddPeers(conn, peers)
	return nil
}

func (conn *Conn) Write(opcode uint32, body []byte) error {
	reserve, body, err := conn.reserve(opcode, body)
	if err != nil {
		return err
	}
	return conn.write(newMessageData(conn.peer.config.Magic, opcode, reserve, body))
}

func (conn *Conn) write(msg *message) error {
	msgAttr := msgToAttribute[msg.code]
	if msgAttr == nil {
		msgAttr = defaultMessageAttribute
	}

	data, err := msg.Marshal()
	if err != nil {
		return err
	}
	err = conn.pq.Push(data, int(msgAttr.priority))
	return err
}

func (conn *Conn) reserve(opcode uint32, body []byte) ([]byte, []byte, error) {
	msgAttr := msgToAttribute[opcode]
	if msgAttr == nil {
		msgAttr = defaultMessageAttribute
	}
	reserve := []byte{}
	flags := []int{}

	if msgAttr.relay {
		times := relayTimes

		if v, ok := msgAttr.relayCache.Get(crc32.ChecksumIEEE(body)); ok {
			if v.(int) == 0 {
				return nil, nil, ErrNoNeedToRelay
			}
			times = v.(int) - 1<<5
		}

		if len(flags) > 0 {
			flags[0] += times
		} else {
			flags = append(flags, times)
		}
	}
	if msgAttr.compress {
		if len(flags) > 0 {
			flags[0] += compressFlag
		} else {
			flags = append(flags, compressFlag)
		}
		body = compress(nil, body)
	}
	for _, flag := range flags {
		reserve = append(reserve, byte(flag))
	}
	msgAttr.duplicateFilter(body, conn.peer.id, msgAttr.frequency)

	return reserve, body, nil
}

// Close connection to remote peer.
func (conn *Conn) Close() error {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	pid := conn.remotePeer
	logger.Info("Closing connection with ", pid.Pretty())
	if _, ok := conn.peer.conns.Load(pid); ok {
		conn.peer.conns.Delete(pid)
	}
	if conn.stream != nil {
		conn.peer.bus.Publish(eventbus.TopicConnEvent, pid, eventbus.PeerDisconnEvent)
		addrs := conn.peer.table.peerStore.Addrs(pid)
		conn.peer.table.peerStore.SetAddrs(pid, addrs, peerstore.RecentlyConnectedAddrTTL)
		return conn.stream.Close()
	}
	return nil
}

// Established returns whether the connection is established.
func (conn *Conn) Established() bool {
	conn.mutex.Lock()
	r := conn.isEstablished
	conn.mutex.Unlock()
	return r
}

// Establish means establishing the connection. It returns the previous status.
func (conn *Conn) Establish() bool {
	conn.mutex.Lock()
	r := conn.isEstablished
	if !conn.isEstablished {
		conn.establish()
	}
	conn.mutex.Unlock()
	return r
}

func (conn *Conn) establish() {
	conn.peer.table.AddPeerToTable(conn)
	conn.isEstablished = true
	conn.establishSucceedCh <- true
	pid := conn.remotePeer
	conn.peer.conns.Store(pid, conn)
	conn.peer.bus.Publish(eventbus.TopicConnEvent, pid, eventbus.PeerConnEvent)
	logger.Infof("Succeed to establish connection with peer %s, addrs: %v", conn.remotePeer.Pretty(), conn.peer.table.peerStore.PeerInfo(conn.remotePeer))
}

// check if the message is valid. Called immediately after receiving a new message.
func (conn *Conn) checkMessage(msg *message) error {
	if conn.peer.config.Magic != msg.messageHeader.magic {
		return ErrMagic
	}

	return msg.check()
}
