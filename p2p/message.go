// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"bufio"
	"bytes"
	"errors"
	"hash/crc32"

	conv "github.com/BOXFoundation/Quicksilver/p2p/convert"
	"github.com/BOXFoundation/Quicksilver/p2p/pb"
	"github.com/BOXFoundation/Quicksilver/util"
	proto "github.com/gogo/protobuf/proto"
)

// const
const (
	ProtocolID = "/box/1.0.0"
	// Mainnet velocity of light
	Mainnet         uint32 = 0x11de784a
	Testnet         uint32 = 0x54455354
	FixHeaderLength        = 4

	Ping              = 0x00
	Pong              = 0x01
	PeerDiscover      = 0x02
	PeerDiscoverReply = 0x03
	NewBlockMsg       = 0x04
	TransactionMsg    = 0x05

	MaxNebMessageDataLength = 1024 * 1024 * 1024 // 1G bytes
)

// NetworkNamtToMagic is a map from network name to magic number.
var NetworkNamtToMagic = map[string]uint32{
	"mainnet": Mainnet,
	"testnet": Testnet,
}

// error
var (
	ErrMessageHeader           = errors.New("Invalid message header data")
	ErrFromProtoMessageMessage = errors.New("Invalid proto message")
)

// MessageHeader message header info from network.
type MessageHeader struct {
	Magic        uint32
	Code         uint32
	DataLength   uint32
	DataChecksum uint32
	Reserved     []byte
}

var _ conv.Convertible = (*MessageHeader)(nil)
var _ conv.Serializable = (*MessageHeader)(nil)

// MessageData defines the full message content from network.
type MessageData struct {
	*MessageHeader
	Body []byte
}

// NewMessageData returns a message data object
func NewMessageData(magic uint32, code uint32, reserved []byte, body []byte) *MessageData {
	return NewMessageDataWithHeader(NewMessageHeader(magic, code, reserved, body), body)
}

// NewMessageDataWithHeader returns a message data object
func NewMessageDataWithHeader(header *MessageHeader, body []byte) *MessageData {
	return &MessageData{
		MessageHeader: header,
		Body:          body,
	}
}

// NewMessageHeader returns a message header object
func NewMessageHeader(magic uint32, code uint32, reserved []byte, body []byte) *MessageHeader {
	return &MessageHeader{
		Magic:        magic,
		Code:         code,
		DataLength:   uint32(len(body)),
		DataChecksum: crc32.ChecksumIEEE(body),
		Reserved:     reserved,
	}
}

// UnmarshalHeader parse the bytes data into MessageHeader
func UnmarshalHeader(data []byte) (*MessageHeader, error) {
	header := &MessageHeader{}
	if err := header.Unmarshal(data); err != nil {
		return nil, err
	}
	return header, nil
}

// ReadMessageData reads a MessageData from reader
func ReadMessageData(r *bufio.Reader) (*MessageData, error) {
	var lenbuf = make([]byte, 4)
	if err := readBuffer(r, lenbuf); err != nil {
		return nil, err
	}
	var headerLen = util.Uint32(lenbuf)

	var headerbuf = make([]byte, headerLen)
	if err := readBuffer(r, headerbuf); err != nil {
		return nil, err
	}
	header, err := UnmarshalHeader(headerbuf)
	if err != nil {
		return nil, err
	}

	var body = make([]byte, header.DataLength)
	if err := readBuffer(r, body); err != nil {
		return nil, err
	}

	return NewMessageDataWithHeader(header, body), nil
}

func readBuffer(r *bufio.Reader, buf []byte) error {
	var total = len(buf)
	var read = 0
	for read < total {
		n, err := r.Read(buf[read:])
		if err != nil {
			return err
		}
		read += n
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////

// ToProtoMessage converts header message in proto.
func (header *MessageHeader) ToProtoMessage() (proto.Message, error) {
	return &p2ppb.MessageHeader{
		Magic:        header.Magic,
		Code:         header.Code,
		DataLength:   header.DataLength,
		DataChecksum: header.DataChecksum,
		Reserved:     header.Reserved,
	}, nil
}

// FromProtoMessage header message in proto.
func (header *MessageHeader) FromProtoMessage(msg proto.Message) error {

	pb := msg.(*p2ppb.MessageHeader)
	if pb != nil {
		header.Magic = pb.Magic
		header.Code = pb.Code
		header.DataLength = pb.DataLength
		header.DataChecksum = pb.DataChecksum
		header.Reserved = pb.Reserved
		return nil
	}
	return ErrFromProtoMessageMessage
}

// Marshal method marshal MessageHeader object to binary
func (header *MessageHeader) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(header)
}

// Unmarshal method unmarshal binary data to MessageHeader object
func (header *MessageHeader) Unmarshal(data []byte) error {
	msg := &p2ppb.MessageHeader{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return header.FromProtoMessage(msg)
}

////////////////////////////////////////////////////////////////////////////////

// Marshal method marshal MessageData object to binary
func (msg *MessageData) Marshal() (data []byte, err error) {
	headerData, err := msg.MessageHeader.Marshal()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Write(util.FromUint32(uint32(len(headerData))))
	buf.Write(headerData)
	buf.Write(msg.Body)
	return buf.Bytes(), nil
}

// Unmarshal method unmarshal binary data to MessageData object
// func (msg *MessageData) Unmarshal(data []byte) error {
// 	msg := &p2ppb.MessageHeader{}
// 	if err := proto.Unmarshal(data, msg); err != nil {
// 		return err
// 	}
// 	return header.FromProtoMessage(msg)
// }
