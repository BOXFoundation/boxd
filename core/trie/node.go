// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package trie

import (
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/crypto"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	proto "github.com/gogo/protobuf/proto"
)

// Flag to identify the type of node
type Flag int

const (
	unknown Flag = iota
	leaf
	extension
	branch
)

var termintor = []byte{0x0}

// Node in trie.
// Branch [hash_0, hash_1, ..., hash_f]
// Extension [path, next hash]
// Leaf [path, value, terminator]
type Node struct {
	Hash  *crypto.HashType
	Value [][]byte
}

var _ conv.Convertible = (*Node)(nil)
var _ conv.Serializable = (*Node)(nil)

// ToProtoMessage converts node to proto message.
func (n *Node) ToProtoMessage() (proto.Message, error) {
	return &corepb.Node{
		Value: n.Value,
	}, nil
}

// FromProtoMessage converts proto message to node.
func (n *Node) FromProtoMessage(msg proto.Message) error {
	if msg, ok := msg.(*corepb.Node); ok {
		if msg != nil {
			bytes, err := proto.Marshal(msg)
			if err != nil {
				return err
			}
			// n.Hash = crypto.Sha3256(bytes)
			n.Hash = new(crypto.HashType)
			n.Hash.SetBytes(crypto.Sha3256(bytes))
			n.Value = msg.Value
			return nil
		}
		return core.ErrInvalidTrieProtoMessage
	}
	return core.ErrInvalidTrieProtoMessage
}

// Marshal method marshal node object to binary
func (n *Node) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(n)
}

// Unmarshal method unmarshal binary data to node object
func (n *Node) Unmarshal(data []byte) error {
	msg := &corepb.Node{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return n.FromProtoMessage(msg)
}

func newNode(value [][]byte) *Node {
	return &Node{Value: value}
}

func newEmptyBranchNode() *Node {
	value := [][]byte{nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil}
	return newNode(value)
}

func newExtensionNode(key, value []byte) *Node {
	v := [][]byte{key, value}
	return newNode(v)
}

// Type returns the type of node.
func (n *Node) Type() Flag {
	switch len(n.Value) {
	case 3:
		return leaf
	case 2:
		return extension
	case 16:
		return branch
	default:
		return unknown
	}
}

// BranchLen returns the length of branch node.
func (n *Node) BranchLen() int {
	var count int
	for _, v := range n.Value {
		if len(v) > 0 {
			count++
		}
	}
	return count
}

// FirstSubNodeHashInBranch returns the first sub node hash in branch node.
func (n *Node) FirstSubNodeHashInBranch() (*crypto.HashType, int) {
	for idx, v := range n.Value {
		if len(v) > 0 {
			return bytesToHash(v), idx
		}
	}
	return nil, 0
}
