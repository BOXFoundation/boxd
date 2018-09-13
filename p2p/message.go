// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"errors"
	"hash/crc32"

	"github.com/BOXFoundation/Quicksilver/util"
)

// const
const (
	ProtocolID = "/box/1.0.0"
	// Mainnet velocity of light
	Mainnet                 uint32 = 0x11de784a
	Testnet                 uint32 = 0x54455354
	Step                           = 4
	MessageHeaderLength            = 24
	Ping                           = 0x00
	Pong                           = 0x01
	MaxNebMessageDataLength        = 1024 * 1024 * 1024 // 1G bytes
)

// error
var (
	ErrMessageHeader = errors.New("Invalid message header data")
)

// MessageHeader message header info from network.
type MessageHeader struct {
	Magic        uint32
	Code         uint32
	DataLength   uint32
	Checksum     uint32
	DataChecksum uint32
	Reserved     []byte
}

// NewMessage return full message in bytes
func NewMessage(header *MessageHeader, body []byte) []byte {

	msg := make([]byte, MessageHeaderLength+len(body))
	copy(msg[:4], util.FromUint32(header.Magic))
	copy(msg[4:8], util.FromUint32(header.Code))
	copy(msg[8:12], util.FromUint32(header.DataLength))
	copy(msg[12:16], util.FromUint32(header.DataChecksum))
	copy(msg[16:20], header.Reserved)
	headerCheckSum := crc32.ChecksumIEEE(msg[:20])
	copy(msg[20:24], util.FromUint32(headerCheckSum))
	copy(msg[24:], body)
	return msg
}

// ParseHeader parse the bytes data into MessageHeader
func ParseHeader(data []byte) (*MessageHeader, error) {

	if len(data) != MessageHeaderLength {
		return nil, ErrMessageHeader
	}
	header := &MessageHeader{}
	header.Magic = util.Uint32(data[:4])
	header.Code = util.Uint32(data[4:8])
	header.DataLength = util.Uint32(data[8:12])
	header.DataChecksum = util.Uint32(data[12:16])
	header.Reserved = data[16:20]
	header.Checksum = util.Uint32(data[20:24])
	return header, nil
}
