// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

const (
	HeaderLength = 16
	Ping         = 0x00
	Pong         = 0x01
)

type MessageHeader struct {
	Magic      BoxNet
	Code       uint32
	DataLength int
	Checksum   [4]byte
}

func ParseHeader(header []byte) (*MessageHeader, error) {
	return nil, nil
}

func ParseBody(body []byte) error {
	return nil
}
