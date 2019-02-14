// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package eventbus

const (
	// TopicSetDebugLevel is topic for changing debug level
	TopicSetDebugLevel = "rpc:setdebuglevel"
	// TopicUpdateNetworkID is topic for updating network id
	TopicUpdateNetworkID = "rpc:updatenetworkid"
	// TopicGetNetworkID is topic for querying network id
	TopicGetNetworkID = "rpc:getnetworkid"
	// TopicGetAddressBook is topic for listing p2p peer status
	TopicGetAddressBook = "rpc:getaddressbook"
	//TopicP2PPeerAddr is a event topic for new peer addr found or peer addr updated
	TopicP2PPeerAddr = "p2p:peeraddr"
	//TopicP2PAddPeer is a event topic for adding peer addr to peer store
	TopicP2PAddPeer = "p2p:addpeer"
	// TopicConnEvent is a event topic of events for score updated
	TopicConnEvent = "p2p:connevent"

	////////////////////////////// chain /////////////////////////////

	// TopicChainUpdate is topic for notifying that the chain is updated,
	// either chain reorg, or chain extended.
	TopicChainUpdate = "chain:update"

	// TopicUtxoUpdate is topic for notifying that chain utxo is changed
	TopicUtxoUpdate = "chain:utxoupdate"

	////////////////////////////// db /////////////////////////////

	// TopicGetDatabaseKeys is topic for get keys of a specified storage
	TopicGetDatabaseKeys = "rpc:database:keys"
	// TopicGetDatabaseValue is topic for get value of specified key
	TopicGetDatabaseValue = "rpc:database:get"
	// TopicRPCSendNewBlock is topic for sending new block to explorer
	TopicRPCSendNewBlock = "rpc:newblock:send"
)
