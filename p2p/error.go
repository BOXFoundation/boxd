// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import "errors"

// error defined
var (
	//conn.go
	ErrMagic                     = errors.New("magic is error")
	ErrHeaderCheckSum            = errors.New("header checksum is error")
	ErrExceedMaxDataLength       = errors.New("exceed max data length")
	ErrBodyCheckSum              = errors.New("body checksum is error")
	ErrMessageDataContent        = errors.New("Invalid message data content")
	ErrNoConnectionEstablished   = errors.New("No connection established")
	ErrFailedToSendMessageToPeer = errors.New("Failed to send message to peer")

	//message.go
	ErrMessageHeaderLength     = errors.New("Can not read p2p message header length")
	ErrMessageHeader           = errors.New("Invalid p2p message header data")
	ErrMessageDataBody         = errors.New("Invalid p2p message body")
	ErrFromProtoMessageMessage = errors.New("Invalid proto message")
)
