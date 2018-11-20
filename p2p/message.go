// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"bytes"
	"hash/crc32"
	"io"
	"unsafe"

	conv "github.com/BOXFoundation/boxd/p2p/convert"
	"github.com/BOXFoundation/boxd/p2p/pb"
	"github.com/BOXFoundation/boxd/util"
	proto "github.com/gogo/protobuf/proto"
	peer "github.com/libp2p/go-libp2p-peer"
)

// const
const (
	ProtocolID = "/box/1.0.0"
	// Mainnet velocity of light
	Mainnet         uint32 = 0x11de784a
	Testnet         uint32 = 0x54455354
	FixHeaderLength        = 4

	Ping              uint32 = 0x00
	Pong              uint32 = 0x01
	PeerDiscover      uint32 = 0x02
	PeerDiscoverReply uint32 = 0x03
	NewBlockMsg       uint32 = 0x04
	TransactionMsg    uint32 = 0x05

	// Sync Manager
	LocateForkPointRequest  = 0x10
	LocateForkPointResponse = 0x11
	LocateCheckRequest      = 0x12
	LocateCheckResponse     = 0x13
	BlockChunkRequest       = 0x14
	BlockChunkResponse      = 0x15

	EternalBlockMsg = 0x16

	LightSyncRequest = 0x17
	LightSyncReponse = 0x18

	MaxMessageDataLength = 1024 * 1024 * 1024 // 1GB
)

const (
	lowPriority uint8 = iota
	midPriority
	highPriority
	topPriority
)

var msgToAttribute = map[uint32]*messageAttribute{
	Ping:                    &messageAttribute{compress: false, priority: lowPriority},
	Pong:                    &messageAttribute{compress: false, priority: lowPriority},
	PeerDiscover:            &messageAttribute{compress: false, priority: lowPriority},
	PeerDiscoverReply:       &messageAttribute{compress: true, priority: midPriority},
	NewBlockMsg:             &messageAttribute{compress: true, priority: topPriority},
	TransactionMsg:          &messageAttribute{compress: true, priority: highPriority},
	LocateForkPointRequest:  &messageAttribute{compress: false, priority: highPriority},
	LocateForkPointResponse: &messageAttribute{compress: true, priority: midPriority},
	LocateCheckRequest:      &messageAttribute{compress: false, priority: highPriority},
	LocateCheckResponse:     &messageAttribute{compress: false, priority: midPriority},
	BlockChunkRequest:       &messageAttribute{compress: true, priority: highPriority},
	BlockChunkResponse:      &messageAttribute{compress: true, priority: highPriority},
	EternalBlockMsg:         &messageAttribute{compress: false, priority: midPriority},
}

// NetworkNamtToMagic is a map from network name to magic number.
var NetworkNamtToMagic = map[string]uint32{
	"mainnet": Mainnet,
	"testnet": Testnet,
}

// messageHeader message header info from network.
type messageHeader struct {
	magic        uint32
	code         uint32
	dataLength   uint32
	dataChecksum uint32
	reserved     []byte
}

var _ conv.Convertible = (*messageHeader)(nil)
var _ conv.Serializable = (*messageHeader)(nil)

// message defines the full message content from network.
type message struct {
	*messageHeader
	body []byte
}

var _ conv.Serializable = (*message)(nil)

// newMessageData returns a message data object
func newMessageData(magic uint32, code uint32, reserved []byte, body []byte) *message {
	return newMessageDataWithHeader(newMessageHeader(magic, code, reserved, body), body)
}

// newMessageDataWithHeader returns a message data object
func newMessageDataWithHeader(header *messageHeader, body []byte) *message {
	return &message{
		messageHeader: header,
		body:          body,
	}
}

// newMessageHeader returns a message header object
func newMessageHeader(magic uint32, code uint32, reserved []byte, body []byte) *messageHeader {
	return &messageHeader{
		magic:        magic,
		code:         code,
		dataLength:   uint32(len(body)),
		dataChecksum: crc32.ChecksumIEEE(body),
		reserved:     reserved,
	}
}

// unmarshalHeader parse the bytes data into messageHeader
func unmarshalHeader(data []byte) (*messageHeader, error) {
	header := &messageHeader{}
	if err := header.Unmarshal(data); err != nil {
		return nil, err
	}
	return header, nil
}

// readMessageData reads a message from reader
func readMessageData(r io.Reader) (*message, error) {
	headerLen, err := util.ReadUint32(r)
	if err != nil {
		return nil, err
	}
	headerBuf, err := util.ReadBytesOfLength(r, headerLen)
	if err != nil {
		return nil, err
	}
	header, err := unmarshalHeader(headerBuf)
	if err != nil {
		return nil, err
	}

	// return error if the data length exceeds the max data length
	if header.dataLength > MaxMessageDataLength {
		return nil, ErrExceedMaxDataLength
	}

	body, err := util.ReadBytesOfLength(r, header.dataLength)
	if err != nil {
		return nil, err
	}

	return newMessageDataWithHeader(header, body), nil
}

// message defines the full message content from network.
type messageAttribute struct {
	compress bool
	priority uint8
}

////////////////////////////////////////////////////////////////////////////////

// ToProtoMessage converts header message in proto.
func (header *messageHeader) ToProtoMessage() (proto.Message, error) {
	return &p2ppb.MessageHeader{
		Magic:        header.magic,
		Code:         header.code,
		DataLength:   header.dataLength,
		DataChecksum: header.dataChecksum,
		Reserved:     header.reserved,
	}, nil
}

// FromProtoMessage header message in proto.
func (header *messageHeader) FromProtoMessage(msg proto.Message) error {

	pb := msg.(*p2ppb.MessageHeader)
	if pb != nil {
		header.magic = pb.Magic
		header.code = pb.Code
		header.dataLength = pb.DataLength
		header.dataChecksum = pb.DataChecksum
		header.reserved = pb.Reserved
		return nil
	}
	return ErrFromProtoMessageMessage
}

// Marshal method marshal messageHeader object to binary
func (header *messageHeader) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(header)
}

// Unmarshal method unmarshal binary data to messageHeader object
func (header *messageHeader) Unmarshal(data []byte) error {
	msg := &p2ppb.MessageHeader{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return header.FromProtoMessage(msg)
}

////////////////////////////////////////////////////////////////////////////////
// implements conv.Serializable interface

// Marshal method marshal message object to binary
func (msg *message) Marshal() (data []byte, err error) {
	headerData, err := msg.messageHeader.Marshal()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := util.WriteUint32(&buf, uint32(len(headerData))); err != nil {
		return nil, err
	}
	if err := util.WriteBytes(&buf, headerData); err != nil {
		return nil, err
	}
	if err := util.WriteBytes(&buf, msg.body); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Unmarshal method unmarshal binary data to message object
func (msg *message) Unmarshal(data []byte) error {
	var buf = bytes.NewBuffer(data)
	m, err := readMessageData(buf)
	if err != nil {
		return err
	}
	msg.messageHeader = m.messageHeader
	msg.body = m.body

	return nil
}

// check checks whether the message data is valid
func (msg *message) check() error {
	if msg.dataLength > MaxMessageDataLength {
		return ErrExceedMaxDataLength
	}

	expectedDataCheckSum := crc32.ChecksumIEEE(msg.body)
	if expectedDataCheckSum != msg.dataChecksum {
		return ErrBodyCheckSum
	}
	return nil
}

// Len returns the msg len
func (msg *message) Len() int64 {
	return int64(unsafe.Sizeof(*msg.messageHeader) + unsafe.Sizeof(msg.body))
}

// p2p message with remote peer ID
type remoteMessage struct {
	*message
	from peer.ID
}

var _ Message = (*remoteMessage)(nil)

// implement Message interface

// Code returns the message code
func (msg *remoteMessage) Code() uint32 {
	return msg.messageHeader.code
}

// Body returns the message body data as bytes
func (msg *remoteMessage) Body() []byte {
	return msg.body
}

// From returns the remote peer id from which the message was received
func (msg *remoteMessage) From() peer.ID {
	return msg.from
}
