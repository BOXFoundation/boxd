// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/p2p"
	peer "github.com/libp2p/go-libp2p-peer"
)

// LocalMessage sent between local goroutines
type LocalMessage struct {
	code uint32
	// payload
	data interface{}
}

var _ p2p.Message = (*LocalMessage)(nil)

// NewLocalMessage creates a localMessage
func NewLocalMessage(code uint32, data interface{}) *LocalMessage {
	return &LocalMessage{
		code: code,
		data: data,
	}
}

// implement Message interface

// Code returns the message code.
func (msg *LocalMessage) Code() uint32 {
	return msg.code
}

// Body returns the message body.
// Not used.
func (msg *LocalMessage) Body() []byte {
	return nil
}

// From returns the remote peer id from which the message was received.
// Not applicable locally.
func (msg *LocalMessage) From() peer.ID {
	id, _ := peer.IDFromString("DUMMY_ID")
	return id
}

// Data returns message payload
func (msg *LocalMessage) Data() interface{} {
	return msg.data
}

// ChainUpdateMsg sent from blockchain to, e.g., mempool
type ChainUpdateMsg struct {
	// block connected/disconnected from main chain
	Connected bool
	Block     *types.Block
}
