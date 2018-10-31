// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package eventbus

// BusEvent means events happened transfering by bus.
type BusEvent int64

const (
	// ConnTimeOutEvent indicates the event if the conn time out.
	ConnTimeOutEvent BusEvent = iota

	// BadBlockEvent indicates the event if process new block throwing err.
	BadBlockEvent

	// BadTxEvent indicates the event if process new tx throwing err.
	BadTxEvent

	// SyncMsgEvent indicates the event when receive sync msg.
	SyncMsgEvent

	// HeartBeatEvent indicates the event when receive hb.
	HeartBeatEvent

	// NoHeartBeatEvent indicates the event when long time no receive hb.
	NoHeartBeatEvent

	// ConnUnsteadinessEvent indicates the event when conn is not steady.
	ConnUnsteadinessEvent

	// NewBlockEvent indicates the event for new block.
	NewBlockEvent

	// NewTxEvent indicates the event for new tx.
	NewTxEvent

	// PeerConnEvent indicates the event for conn.
	PeerConnEvent

	// PeerDisconnEvent indicates the event for disconn.
	PeerDisconnEvent
)
