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
	"github.com/BOXFoundation/boxd/storage"
)

const (
	// Number of codehash->size associations to keep.
	codeSizeCacheSize = 100000
)

// Database wraps access to tries and contract code.
type Database interface {
	// OpenTrie opens the main account trie.
	OpenTrie(root crypto.HashType) (*trie.Trie, error)

	// CopyTrie returns an independent copy of the given trie.
	CopyTrie(*trie.Trie) *trie.Trie

	// TrieDB retrieves the low level trie database used for data storage.
	TrieDB() *storage.Database
}

func NewDatabase(db *storage.Database) Database {
	return &cachingDB{
		db: db,
	}
}

type cachingDB struct {
	db *storage.Database
}

// OpenTrie opens the main account trie.
func (db *cachingDB) OpenTrie(root crypto.HashType) (*trie.Trie, error) {
	return trie.NewByHash(&root, db.db)
}

// func (db *cachingDB) pushTrie(t *trie.SecureTrie) {
// 	db.mu.Lock()
// 	defer db.mu.Unlock()

// 	if len(db.pastTries) >= maxPastTries {
// 		copy(db.pastTries, db.pastTries[1:])
// 		db.pastTries[len(db.pastTries)-1] = t
// 	} else {
// 		db.pastTries = append(db.pastTries, t)
// 	}
// }

// CopyTrie returns an independent copy of the given trie.
func (db *cachingDB) CopyTrie(t *trie.Trie) *trie.Trie {
	// switch t := t.(type) {
	// case cachedTrie:
	// 	return cachedTrie{t.SecureTrie.Copy(), db}
	// case *trie.SecureTrie:
	// 	return t.Copy()
	// default:
	// 	panic(fmt.Errorf("unknown trie type %T", t))
	// }
	return nil
}

// TrieDB retrieves any intermediate trie-node caching layer.
func (db *cachingDB) TrieDB() *storage.Database {
	return db.db
}
