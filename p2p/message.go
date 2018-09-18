// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"bytes"
	"errors"

	"github.com/BOXFoundation/Quicksilver/p2p/pb"
	"github.com/BOXFoundation/Quicksilver/util"
	proto "github.com/gogo/protobuf/proto"
)

// const
const (
	ProtocolID = "/box/1.0.0"
	// Mainnet velocity of light
	Mainnet                 uint32 = 0x11de784a
	Testnet                 uint32 = 0x54455354
	FixHeaderLength                = 4
	Ping                           = 0x00
	Pong                           = 0x01
	PeerDiscover                   = 0x02
	PeerDiscoverReply              = 0x03
	MaxNebMessageDataLength        = 1024 * 1024 * 1024 // 1G bytes
)

// NetworkNamtToMagic is a map from network name to magic number.
var NetworkNamtToMagic = map[string]uint32{
	"mainnet": Mainnet,
	"testnet": Testnet,
}

// error
var (
	ErrMessageHeader      = errors.New("Invalid message header data")
	ErrDeserializeMessage = errors.New("Invalid proto message")
)

// MessageHeader message header info from network.
type MessageHeader struct {
	Magic        uint32
	Code         uint32
	DataLength   uint32
	DataChecksum uint32
	Reserved     []byte
}

// NewMessage return full message in bytes
func NewMessage(header *MessageHeader, body []byte) ([]byte, error) {

	pbHeader := header.Serialize()
	headerBytes, err := proto.Marshal(pbHeader)
	if err != nil {
		return nil, err
	}
	var msg bytes.Buffer
	msg.Write(util.FromUint32(uint32(len(headerBytes))))
	msg.Write(headerBytes)
	msg.Write(body)
	return msg.Bytes(), nil
}

// Serialize header message in proto.
func (header *MessageHeader) Serialize() proto.Message {

	return &p2ppb.MessageHeader{
		Magic:        header.Magic,
		Code:         header.Code,
		DataLength:   header.DataLength,
		DataChecksum: header.DataChecksum,
		Reserved:     header.Reserved,
	}
}

// Deserialize header message in proto.
func (header *MessageHeader) Deserialize(msg proto.Message) error {

	pb := msg.(*p2ppb.MessageHeader)
	if pb != nil {
		header.Magic = pb.Magic
		header.Code = pb.Code
		header.DataLength = pb.DataLength
		header.DataChecksum = pb.DataChecksum
		header.Reserved = pb.Reserved
		return nil
	}
	return ErrDeserializeMessage
}

// ParseHeader parse the bytes data into MessageHeader
func ParseHeader(data []byte) (*MessageHeader, error) {

	pb := new(p2ppb.MessageHeader)
	if err := proto.Unmarshal(data, pb); err != nil {
		return nil, err
	}
	header := &MessageHeader{}
	if err := header.Deserialize(pb); err != nil {
		return nil, err
	}
	return header, nil
}
