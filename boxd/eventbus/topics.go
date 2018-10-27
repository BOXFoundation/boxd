// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package eventbus

const (
	// TopicSetDebugLevel is topic for changing debug level
	TopicSetDebugLevel = "rpc:setdebuglevel"

	//TopicP2PPeerAddr is a event topic for new peer addr found or peer addr updated
	TopicP2PPeerAddr = "p2p:peeraddr"

	// TopicChainScoreEvent is a event topic of chain event for score updated
	TopicChainScoreEvent = "chain:scoreevent"

	////////////////////////////// db /////////////////////////////

	// TopicGetDatabaseKeys is topic for get keys of a specified storage
	TopicGetDatabaseKeys = "rpc:database:keys"
	// TopicGetDatabaseValue is topic for get value of specified key
	TopicGetDatabaseValue = "rpc:database:get"
)
