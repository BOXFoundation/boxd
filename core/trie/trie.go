// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package trie

import (
	"errors"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/storage"
)

var logger = log.NewLogger("trie") // logger

// Trie is a Merkle Patricia Trie.
type Trie struct {
	rootHash *crypto.HashType
	db       storage.Storage
}

// New creates a trie with an existing root node from db.
func New(root *Node, db storage.Storage) (*Trie, error) {
	if db == nil {
		panic("trie.New called without a database")
	}
	trie := &Trie{
		db: db,
	}
	if root == nil {
		return trie, nil
	}

	if _, err := trie.db.Get(root.Hash[:]); err != nil {
		return nil, err
	}
	return trie, nil
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
		return core.ErrNodeNotFound
	}
	nodeBin, err := node.Marshal()
	if err != nil {
		return err
	}
	node.Hash = new(crypto.HashType)
	node.Hash.SetBytes(crypto.Sha3256(nodeBin))
	return t.db.Put(node.Hash[:], nodeBin)
}

// Get value by key in the trie.
func (t *Trie) Get(key []byte) ([]byte, error) {
	return t.get(t.rootHash, keyToHex(key))
}

func (t *Trie) get(hash *crypto.HashType, key []byte) ([]byte, error) {

	if hash == nil || len(key) == 0 {
		return nil, core.ErrNodeNotFound
	}
	root, err := t.getNode(hash)
	if err != nil {
		return nil, err
	}
	if root.Type() == unknown {
		return nil, core.ErrNodeNotFound
	}
	prefixs, err := commonPrefixes(root.Value[0], key)
	if err != nil {
		return nil, err
	}
	var tmpHash = new(crypto.HashType)
	var rootKeyLen = len(root.Value[0])
	var prefixsLen = len(prefixs)
	var keyLen = len(key)
	switch root.Type() {
	case leaf:
		if prefixsLen != rootKeyLen || prefixsLen != keyLen {
			return nil, core.ErrNodeNotFound
		}
		return root.Value[1], nil
	case branch:
		tmpHash.SetBytes(root.Value[key[0]])
		return t.get(tmpHash, key[1:])
	case extension:
		tmpHash.SetBytes(root.Value[1])
		return t.get(tmpHash, key[prefixsLen:])
	}

	return nil, core.ErrNodeNotFound
}

// Update key with value in the trie.
func (t *Trie) Update(key, value []byte) error {

	k := keyToHex(key)
	var rootHash *crypto.HashType
	var err error
	if len(value) != 0 {
		if rootHash, err = t.update(t.rootHash, k, value); err != nil {
			return err
		}
	} else {
		if rootHash, err = t.delete(t.rootHash, k); err != nil {
			return err
		}
	}
	t.rootHash = rootHash
	return nil
}

func (t *Trie) update(hash *crypto.HashType, key, value []byte) (*crypto.HashType, error) {
	if hash == nil {
		value := [][]byte{key, value, termintor}
		node := newNode(value)
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
		return nil, core.ErrNodeNotFound
	}

	prefixs, err := commonPrefixes(root.Value[0], key)
	if err != nil {
		return nil, err
	}
	var rootKeyLen = len(root.Value[0])
	var prefixsLen = len(prefixs)
	switch root.Type() {
	case leaf:
		switch prefixsLen {
		case rootKeyLen:
			root.Value[1] = value
			if err := t.commit(root); err != nil {
				return nil, err
			}
			return root.Hash, nil
		default:
			branch := newEmptyBranchNode()
			oldleafHash, err := t.update(nil, root.Value[0][prefixsLen+1:], root.Value[1])
			if err != nil {
				return nil, err
			}
			newLeafHash, err := t.update(nil, key[prefixsLen+1:], value)
			if err != nil {
				return nil, err
			}
			branch.Value[root.Value[0][prefixsLen]] = oldleafHash[:]
			branch.Value[key[prefixsLen]] = newLeafHash[:]
			if err := t.commit(branch); err != nil {
				return nil, err
			}
			if prefixsLen == 0 { // only need branch node
				return branch.Hash, nil
			}

			// extension node is needed
			extension := newExtensionNode(prefixs, branch.Hash[:])
			if err := t.commit(extension); err != nil {
				return nil, err
			}
			return extension.Hash, nil
		}
	case extension:
		//*****************************************
		// the initial scenario
		// abcde
		// abcf2
		//      abc
		//       |
		//     d   f
		//    /     \
		//   e       2
		//*****************************************
		switch prefixsLen {
		case rootKeyLen:
			var tmpHash = new(crypto.HashType)
			tmpHash.SetBytes(root.Value[1])
			return t.updateBranchNode(root, tmpHash, key, value, prefixsLen+1)

		//*****************************************
		// there are two scenarios
		//     2  a                    2  a
		//    /    \                  /    |
		//  abcd   bc             abcd   b    c
		//         |                    /      \
		//       d   f                cde      bf2
		//      /     \
		//     e       2
		//******************************************
		case 0:
			branch := newEmptyBranchNode()
			newLeafHash, err := t.update(nil, key[1:], value)
			if err != nil {
				return nil, err
			}
			if rootKeyLen == 1 {
				branch.Value[root.Value[0][0]] = root.Value[1]
			} else {
				extension := newExtensionNode(root.Value[0][1:], root.Value[1])
				if err := t.commit(extension); err != nil {
					return nil, err
				}
				branch.Value[root.Value[0][0]] = extension.Hash[:]
			}
			branch.Value[key[0]] = newLeafHash[:]
			if err := t.commit(branch); err != nil {
				return nil, err
			}
			return branch.Hash, nil

		//*****************************************
		// there are another two scenarios
		//           a                       ab
		//           |                        |
		//         b   e                   5     c
		//        /     \                 /      |
		//       c      32f              32    d   f
		//       |                            /     \
		//     d   f                         e       2
		//    /     \
		//   e       2
		//******************************************
		default:
			branch := newEmptyBranchNode()
			newLeafHash, err := t.update(nil, key[prefixsLen+1:], value)
			if err != nil {
				return nil, err
			}
			if prefixsLen == rootKeyLen-1 {
				branch.Value[root.Value[0][prefixsLen]] = root.Value[1]
			} else {
				extension := newExtensionNode(root.Value[0][prefixsLen+1:], root.Value[1])
				if err := t.commit(extension); err != nil {
					return nil, err
				}
				branch.Value[root.Value[0][prefixsLen]] = extension.Hash[:]
			}
			branch.Value[key[prefixsLen]] = newLeafHash[:]
			if err := t.commit(branch); err != nil {
				return nil, err
			}

			newRoot := newExtensionNode(prefixs, branch.Hash[:])
			if err := t.commit(newRoot); err != nil {
				return nil, err
			}
			return newRoot.Hash, nil
		}

	case branch:
		var tmpHash = new(crypto.HashType)
		tmpHash.SetBytes(root.Value[key[0]])
		return t.updateBranchNode(root, tmpHash, key, value, 1)
	}
	return nil, nil
}

func (t *Trie) updateBranchNode(root *Node, rootHash *crypto.HashType, key, value []byte, prefixsIndex int) (*crypto.HashType, error) {

	hash, err := t.update(rootHash, key[prefixsIndex:], value)
	if err != nil {
		return nil, err
	}
	root.Value[key[0]] = hash[:]
	if err := t.commit(root); err != nil {
		return nil, err
	}
	return root.Hash, nil
}

func (t *Trie) delete(hash *crypto.HashType, key []byte) (*crypto.HashType, error) {

	if hash == nil || len(key) == 0 {
		return nil, core.ErrNodeNotFound
	}
	root, err := t.getNode(hash)
	if err != nil {
		return nil, err
	}
	if root.Type() == unknown {
		return nil, core.ErrInvalidNodeType
	}
	prefixs, err := commonPrefixes(root.Value[0], key)
	var prefixsLen = len(prefixs)
	if err != nil || prefixsLen == 0 {
		return nil, core.ErrNodeNotFound
	}
	var tmpHash = new(crypto.HashType)
	var rootKeyLen = len(root.Value[0])
	var keyLen = len(key)
	switch root.Type() {
	case leaf:
		if prefixsLen != rootKeyLen || prefixsLen != keyLen {
			return nil, core.ErrNodeNotFound
		}
		return nil, nil
	case branch:
		tmpHash.SetBytes(root.Value[key[0]])
		newHash, err := t.delete(tmpHash, key[1:])
		if err != nil {
			return nil, err
		}
		root.Value[key[0]] = newHash[:]
		// the branch node will change to a extension node.
		// 3 scenarios
		// ext -> branch ->
		// ext -> ext ->
		// ext -> leaf
		if root.BranchLen() == 1 {
			subHash, index := root.FirstSubNodeHashInBranch()
			subNode, err := t.getNode(subHash)
			if err != nil {
				return nil, err
			}
			var newNode *Node
			switch subNode.Type() {
			case leaf: // ext -> leaf
				subNode.Value[0] = append([]byte{byte(index)}, subNode.Value[0]...)
				newNode = subNode
			case branch: // ext -> branch ->
				extension := newExtensionNode([]byte{byte(index)}, subHash[:])
				newNode = extension
			case extension: // ext -> ext ->
				tempkey := append([]byte{byte(index)}, subNode.Value[0]...)
				extension := newExtensionNode(tempkey, subNode.Value[1])
				newNode = extension
			default:
				return nil, core.ErrInvalidNodeType
			}
			if err := t.commit(newNode); err != nil {
				return nil, err
			}
			return newNode.Hash, nil
		}
		if err := t.commit(root); err != nil {
			return nil, err
		}
		return root.Hash, nil
	case extension:
		tmpHash.SetBytes(root.Value[1])
		newHash, err := t.delete(tmpHash, key[prefixsLen:])
		if err != nil {
			return nil, err
		}
		if newHash == nil { // ext
			return nil, nil
		}
		var newNode *Node
		subNode, err := t.getNode(newHash)
		if subNode.Type() == unknown {
			return nil, core.ErrInvalidNodeType
		} else if subNode.Type() == branch { // ext -> branch ->
			root.Value[1] = newHash[:]
			newNode = root
		} else { // ext -> ext -> branch  ext -> leaf
			subNode.Value[0] = append(root.Value[0], subNode.Value[0]...)
			newNode = subNode
		}
		if err := t.commit(newNode); err != nil {
			return nil, err
		}
		return newNode.Hash, nil
	}
	return nil, core.ErrInvalidNodeType
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

func hexToKey(hex []byte) []byte {
	l := len(hex) / 2
	var key = make([]byte, l)
	for i := 0; i < l; i++ {
		key[i] = hex[i*2]<<4 + hex[i*2+1]
	}
	return key
}
