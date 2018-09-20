// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"math"

	"github.com/BOXFoundation/Quicksilver/crypto"
)

// BuildMerkleRoot build transaction merkle trie
//
//	         root = h1234 = h(h12 + h34)
//	        /                           \
//	  h12 = h(h1 + h2)            h34 = h(h3 + h4)
//	   /            \              /            \
//	h1 = h(tx1)  h2 = h(tx2)    h3 = h(tx3)  h4 = h(tx4)
//
// The above stored as a linear array is as follows:
//
// 	[h1 h2 h3 h4 h12 h34 root]
func BuildMerkleRoot(txs []*Transaction) []*crypto.HashType {

	leafSize := calcLowestHierarchyCount(len(txs))
	arraySize := leafSize*2 - 1
	merkles := make([]*crypto.HashType, arraySize)
	for i, tx := range txs {
		hash, err := tx.TxHash()
		if err != nil {
			return nil
		}
		merkles[i] = hash
	}

	offset := leafSize
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		case merkles[i] == nil:
			merkles[offset] = nil
		case merkles[i+1] == nil:
			newHash := CombineHash(merkles[i], merkles[i])
			merkles[offset] = newHash
		default:
			newHash := CombineHash(merkles[i], merkles[i+1])
			merkles[offset] = newHash
		}
		offset++
	}

	return merkles
}

func calcLowestHierarchyCount(n int) int {

	if n&(n-1) == 0 {
		return n
	}
	hierarchy := uint(math.Log2(float64(n))) + 1
	return 1 << hierarchy
}

// CalcTxsHash calculate txsHash in block.
func CalcTxsHash(txs []*Transaction) *crypto.HashType {
	txsHash := BuildMerkleRoot(txs)
	return txsHash[len(txsHash)-1]
}

// CombineHash takes two hashes, and returns the hash of their concatenation.
func CombineHash(left *crypto.HashType, right *crypto.HashType) *crypto.HashType {
	// Concatenate the left and right nodes.
	var hash [crypto.HashSize * 2]byte
	copy(hash[:crypto.HashSize], left[:])
	copy(hash[crypto.HashSize:], right[:])

	newHash := crypto.DoubleHashH(hash[:])
	return &newHash
}
