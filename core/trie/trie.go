// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package trie

import (
	"bytes"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/storage"
)

var logger = log.NewLogger("trie") // logger

// Trie is a Merkle Patricia Trie.
type Trie struct {
	rootHash *crypto.HashType
	db       storage.Table
}

// New creates a trie with an existing root hash from db.
func New(rootHash *crypto.HashType, db storage.Table) (*Trie, error) {
	if db == nil {
		panic("trie.New called without a database")
	}
	if rootHash != nil && bytes.Equal(rootHash[:], crypto.ZeroHash[:]) {
		rootHash = nil
	}
	trie := &Trie{
		db:       db,
		rootHash: rootHash,
	}
	if rootHash == nil {
		return trie, nil
	}

	if _, err := trie.db.Get(rootHash[:]); err != nil {
		logger.Error(err)
		return nil, err
	}
	return trie, nil
}

// Hash return root hash of trie.
func (t *Trie) Hash() crypto.HashType {
	if t.rootHash == nil {
		return crypto.HashType{}
	}
	return *t.rootHash
}

func (t *Trie) getNode(hash *crypto.HashType) (*Node, error) {

	nodeBin, err := t.db.Get(hash[:])
	if err != nil {
		return nil, err
	}
	if nodeBin == nil {
		return nil, core.ErrNodeNotFound
	}
	node := new(Node)
	if err := node.Unmarshal(nodeBin); err != nil {
		return nil, err
	}
	return node, nil
}

// Commit persistent the data of the trie.
func (t *Trie) Commit() (*crypto.HashType, error) {
	return t.rootHash, nil
}

func (t *Trie) commit(node *Node) error {

	if node.Type() == unknown {
		return core.ErrNodeNotFound
	}
	nodeBin, err := node.Marshal()
	if err != nil {
		return err
	}
	node.Hash = bytesToHash(crypto.Sha3256(nodeBin))
	t.db.Put(node.Hash[:], nodeBin)
	return nil
}

// Get value by key in the trie.
func (t *Trie) Get(key []byte) ([]byte, error) {
	// return t.get(t.rootHash, keyToHex(key))
	value, err := t.get(t.rootHash, keyToHex(key))
	if err == core.ErrNodeNotFound {
		return nil, nil
	}
	return value, err
}

func (t *Trie) get(hash *crypto.HashType, key []byte) ([]byte, error) {

	if hash == nil || len(key) == 0 {
		return nil, core.ErrNodeNotFound
	}
	root, err := t.getNode(hash)
	if err != nil {
		return nil, err
	}
	// if root == nil {
	// 	return nil, nil
	// }
	if root.Type() == unknown {
		return nil, core.ErrNodeNotFound
	}
	prefixs, err := commonPrefixes(root.Value[0], key)
	if err != nil {
		return nil, err
	}
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
		return t.get(bytesToHash(root.Value[key[0]]), key[1:])
	case extension:
		return t.get(bytesToHash(root.Value[1]), key[prefixsLen:])
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

// Delete node in the trie.
func (t *Trie) Delete(key []byte) error {
	return t.Update(key, nil)
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
			hash, err := t.update(bytesToHash(root.Value[1]), key[prefixsLen:], value)
			if err != nil {
				return nil, err
			}
			root.Value[1] = hash[:]
			if err := t.commit(root); err != nil {
				return nil, err
			}
			return root.Hash, nil

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
		return t.updateBranchNode(root, bytesToHash(root.Value[key[0]]), key, value)
	}
	return nil, nil
}

func (t *Trie) updateBranchNode(root *Node, rootHash *crypto.HashType, key, value []byte) (*crypto.HashType, error) {
	hash, err := t.update(rootHash, key[1:], value)
	if err != nil {
		return nil, err
	}
	root.Value[key[0]] = hashToBytes(hash)
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
	var prefixs []byte
	var prefixsLen int
	if root.Type() != branch {
		prefixs, err = commonPrefixes(root.Value[0], key)
		prefixsLen = len(prefixs)
		if err != nil || prefixsLen == 0 {
			return nil, core.ErrNodeNotFound
		}
	}

	var rootKeyLen = len(root.Value[0])
	var keyLen = len(key)
	switch root.Type() {
	case leaf:
		if prefixsLen != rootKeyLen || prefixsLen != keyLen {
			return nil, core.ErrNodeNotFound
		}
		return nil, nil
	case branch:
		newHash, err := t.delete(bytesToHash(root.Value[key[0]]), key[1:])
		if err != nil {
			return nil, err
		}
		root.Value[key[0]] = hashToBytes(newHash)
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
		newHash, err := t.delete(bytesToHash(root.Value[1]), key[prefixsLen:])
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
		return []byte{}, core.ErrInvalidKeyPath
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

func bytesToHash(bytes []byte) *crypto.HashType {
	if len(bytes) == 0 {
		// if len(bytes) == 0 || bytes.Equal(bytes, crypto.ZeroHash[:]) {
		return nil
	}
	var hash = new(crypto.HashType)
	hash.SetBytes(bytes)
	return hash
}

func hashToBytes(hash *crypto.HashType) []byte {
	if hash == nil {
		return nil
	}
	return hash[:]
}
