// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import peerstore "github.com/libp2p/go-libp2p-peerstore"

type Table struct {
	peerStore peerstore.Peerstore
}

func NewTable() *Table {
	return nil
}

func (t *Table) Loop() {
	// Load Route Table.

}

func (t *Table) lookup() {

}
