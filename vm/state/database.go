// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package state

import (
	"github.com/BOXFoundation/boxd/core/trie"
	"github.com/BOXFoundation/boxd/crypto"
)

// Database wraps access to tries and contract code.
type Database interface {
	// OpenTrie opens the main account trie.
	OpenTrie(root crypto.HashType) (*trie.Trie, error)

	// OpenStorageTrie opens the storage trie of an account.
	OpenStorageTrie(addrHash, root crypto.HashType) (*trie.Trie, error)

	// CopyTrie returns an independent copy of the given trie.
	CopyTrie(*trie.Trie) *trie.Trie

	// ContractCode retrieves a particular contract's code.
	ContractCode(addrHash, codeHash crypto.HashType) ([]byte, error)

	// ContractCodeSize retrieves a particular contracts code's size.
	ContractCodeSize(addrHash, codeHash crypto.HashType) (int, error)

	// TrieDB retrieves the low level trie database used for data storage.
	// TrieDB() *trie.Database
}
