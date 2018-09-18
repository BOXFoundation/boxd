// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
)

type Table struct {
	peerStore  peerstore.Peerstore
	routeTable *kbucket.RoutingTable
}

func NewTable() *Table {
	return nil
}

func (t *Table) Loop() {
	// Load Route Table.

}

func (t *Table) lookup() {

}

// AddPeerToTable add peer route table.
func (t *Table) AddPeerToTable(conn *Conn) {
	peerID := conn.stream.Conn().RemotePeer()
	t.peerStore.AddAddr(
		peerID,
		conn.stream.Conn().RemoteMultiaddr(),
		peerstore.PermanentAddrTTL,
	)
	t.routeTable.Update(peerID)
}
