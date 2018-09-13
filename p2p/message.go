// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"github.com/BOXFoundation/Quicksilver/p2p/pb"
	"github.com/gogo/protobuf/proto"
)

// const
const (
	ProtocolID = "/box/1.0.0"
	// Mainnet velocity of light
	Mainnet                 uint32 = 0x11de784a
	Testnet                 uint32 = 0x54455354
	MessageHeaderLength            = 20
	Ping                           = 0x00
	Pong                           = 0x01
	MaxNebMessageDataLength        = 1024 * 1024 * 1024 // 1G bytes
)

// MessageHeader message header info from network.
type MessageHeader struct {
	Magic        uint32
	Code         uint32
	DataLength   uint32
	Checksum     uint32
	DataChecksum uint32
}

// ParseHeader parse the bytes data into MessageHeader
func ParseHeader(data []byte) (*MessageHeader, error) {
	pb := new(p2ppb.MessageHeader)
	if err := proto.Unmarshal(data, pb); err != nil {
		logger.Error("Failed to unmarshal message header.")
		return nil, err
	}
	header := &MessageHeader{
		Magic:        pb.Magic,
		Code:         pb.Code,
		DataLength:   pb.DataLength,
		Checksum:     pb.Checksum,
		DataChecksum: pb.DataChecksum,
	}
	return header, nil
}

// ToProto converts domain BlockHeader to proto BlockHeader
func (header *MessageHeader) ToProto() (proto.Message, error) {
	return &p2ppb.MessageHeader{
		Magic:        header.Magic,
		Code:         header.Code,
		DataLength:   header.DataLength,
		Checksum:     header.Checksum,
		DataChecksum: header.DataChecksum,
	}, nil
}
