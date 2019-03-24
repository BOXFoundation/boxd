// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package trie

import (
	"errors"

	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage"
)

// Trie is a Merkle Patricia Trie.
type Trie struct {
	root Node
	db   storage.Storage
}

// New creates a trie with an existing root node from db.
func New(root *Node, db storage.Storage) (*Trie, error) {
	if db == nil {
		panic("trie.New called without a database")
	}
	trie := &Trie{
		db:   db,
		root: *root,
	}
	if root == nil {
		return trie, nil
	}

	if _, err := trie.db.Get(root.Hash[:]); err != nil {
		return nil, err
	}
	return trie, nil
}

func (t *Trie) newNode(value [][]byte) *Node {
	return &Node{Value: value}
}

func (t *Trie) getNode(hash *crypto.HashType) (*Node, error) {

	nodeBin, err := t.db.Get(hash[:])
	if err != nil {
		return nil, err
	}
	node := new(Node)
	if err := node.Unmarshal(nodeBin); err != nil {
		return nil, err
	}
	return node, nil
}

func (t *Trie) commit(node *Node) error {

	if node.Type() == unknown {
		return errors.New("Failed to commit node for invalid node type")
	}
	nodeBin, err := node.Marshal()
	if err != nil {
		return err
	}
	copy(node.Hash[:], crypto.Sha3256(nodeBin))
	return t.db.Put(node.Hash[:], nodeBin)
}

// Update key with value in the trie.
func (t *Trie) Update(key, value []byte) error {

	k := keyToHex(key)
	var rootHash *crypto.HashType
	var err error
	if len(value) != 0 {
		rootHash, err = t.update(t.root.Hash, k, value)
		if err != nil {
			return err
		}
	} else {
		rootHash, err = t.delete(t.root.Hash, k)
		if err != nil {
			return err
		}
	}
	t.root.Hash = rootHash
	return nil
}

func (t *Trie) update(hash *crypto.HashType, k, value []byte) (*crypto.HashType, error) {
	if hash == nil {
		value := [][]byte{k, value, termintor}
		node := t.newNode(value)
		if err := t.commit(node); err != nil {
			return nil, err
		}
		return node.Hash, nil
	}

	root, err := t.getNode(hash)
	if err != nil {
		return nil, err
	}
	if root.Type() == unknown {
		return nil, errors.New("Failed to update trie for invalid node type")
	}
	prefixs, err := commonPrefixes(root.Value[0], k)
	if err != nil {
		return nil, err
	}
	var keyLength = len(k)
	switch root.Type() {
	case leaf:
		switch len(prefixs) {
		case keyLength:
			root.Value[1] = value
			if err := t.commit(root); err != nil {
				return nil, err
			}
			return root.Hash, nil
		default:
			branchValue := [][]byte{nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil}
			branch := t.newNode(branchValue)
			oldleafHash, err := t.update(nil, root.Value[0][len(prefixs)+1:], root.Value[1])
			if err != nil {
				return nil, err
			}
			newLeafHash, err := t.update(nil, k[len(prefixs)+1:], value)
			if err != nil {
				return nil, err
			}
			branch.Value[root.Value[0][0]] = oldleafHash[:]
			branch.Value[k[0]] = newLeafHash[:]
			if err := t.commit(branch); err != nil {
				return nil, err
			}
			if len(prefixs) == 0 { // only need branch node
				return branch.Hash, nil
			}

			// extension node is needed
			extensionValue := [][]byte{prefixs, branch.Hash[:]}
			extensionNode := t.newNode(extensionValue)
			if err := t.commit(extensionNode); err != nil {
				return nil, err
			}
			return extensionNode.Hash, nil
		}
	case extension:
	case branch:
	}
	return nil, nil
}

func (t *Trie) delete(root *crypto.HashType, k []byte) (*crypto.HashType, error) {
	return nil, nil
}

func commonPrefixes(old, new []byte) ([]byte, error) {

	if len(old) > len(new) {
		return []byte{}, errors.New("Invalid prefixes")
	}
	var i int
	for ; i < len(old); i++ {
		if old[i] != new[i] {
			break
		}
	}
	return old[:i], nil
}

func keyToHex(key []byte) []byte {
	l := len(key) * 2
	var res = make([]byte, l)
	for i, b := range key {
		res[i*2] = b / 16
		res[i*2+1] = b % 16
	}
	return res
}
