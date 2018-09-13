// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"errors"
	"hash/crc32"
	"time"

	libp2pnet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
)

// const
const (
	PeriodTime = 3
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
	establish          bool
	establishSucceedCh chan bool
	quitHeartBeat      chan bool
}

// NewConn create a stream to remote peer.
func NewConn(stream libp2pnet.Stream, peer *BoxPeer, peerID peer.ID) *Conn {
	return &Conn{
		stream:             stream,
		peer:               peer,
		establish:          false,
		establishSucceedCh: make(chan bool, 1),
		quitHeartBeat:      make(chan bool, 1),
	}
}

func (conn *Conn) loop() {
	if conn.stream == nil {
		if err := conn.Ping(); err != nil {
			return
		}
		go conn.heartBeatService()
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
				if len(messageBuffer) < MessageHeaderLength {
					break
				}
				headerBytes := messageBuffer[:MessageHeaderLength]
				header, err = ParseHeader(headerBytes)
				if err != nil {
					conn.Close()
					return
				}
				if err := conn.checkHeader(header, headerBytes[:20]); err != nil {
					logger.Error("Invalid message header. ", err)
					conn.Close()
					return
				}
				messageBuffer = messageBuffer[MessageHeaderLength:]
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
			conn.Ping()
		case <-conn.quitHeartBeat:
			t.Stop()
			break
		}
	}
}

// Ping the target node
func (conn *Conn) Ping() error {
	body := []byte("ping")
	header := conn.buildMessageHeader(Ping, body, nil)
	msg := NewMessage(header, body)
	if err := conn.Write(msg); err != nil {
		return err
	}
	return nil
}

func (conn *Conn) onPing(data []byte) error {
	if "ping" != string(data) {
		return ErrMessageDataContent
	}
	body := []byte("pong")
	header := conn.buildMessageHeader(Pong, body, nil)
	msg := NewMessage(header, body)
	if err := conn.Write(msg); err != nil {
		return err
	}
	return nil
}

func (conn *Conn) onPong(data []byte) error {
	if "pong" != string(data) {
		return ErrMessageDataContent
	}
	conn.peer.table.AddPeerToTable(conn)
	conn.establish = true
	conn.establishSucceedCh <- true
	return nil
}

func (conn *Conn) Write(data []byte) error {
	_, err := conn.stream.Write(data)
	if err != nil {
		return err
	}
	return nil
}

// Close connection to remote peer.
func (conn *Conn) Close() {
	delete(conn.peer.conns, conn.stream.Conn().RemotePeer().String())
	conn.stream.Close()
	conn.quitHeartBeat <- true
}

func (conn *Conn) checkHeader(header *MessageHeader, headerBytes []byte) error {
	if conn.peer.config.Magic != header.Magic {
		return ErrMagic
	}
	expectedCheckSum := crc32.ChecksumIEEE(headerBytes)
	if expectedCheckSum != header.Checksum {
		return ErrHeaderCheckSum
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
