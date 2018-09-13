/*
 * Copyright (C) 2018 ContentBox Authors
 * This file is part of The go-contentbox library.
 *
 * The go-contentbox library is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The go-contentbox library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The go-contentbox library.  If not, see <http://www.gnu.org/licenses/>.
 */

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
