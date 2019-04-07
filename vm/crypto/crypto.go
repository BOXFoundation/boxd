// Copyright 2014 The go-ethereum Authors
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

package crypto

import (
	"math/big"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/vm/common"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

var (
	secp256k1N, _  = new(big.Int).SetString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16)
	secp256k1halfN = new(big.Int).Div(secp256k1N, big.NewInt(2))
)

// Keccak256 calculates and returns the Keccak256 hash of the input data.
func Keccak256(data ...[]byte) []byte {
	d := sha3.NewLegacyKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to an internal Hash data structure.
func Keccak256Hash(data ...[]byte) (h crypto.HashType) {
	d := sha3.NewLegacyKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	d.Sum(h[:0])
	return h
}

// CreateAddress creates an ethereum address given the bytes and the nonce
//
//  e.g.
//	b: [0,0,0,0,0,0,0,0,0,0,120,117,106,105,110,103,115,104,105]
//	nonce: 0
//
//	data: [214,148,0,0,0,0,0,0,0,0,0,0,120,117,106,105,110,103,115,104,105,128]
//		  0xd6(0xc0 + 22 data len) 0x94(0x80 + 20 b len) + b + 0x80(means 0)
//
func CreateAddress(b types.AddressHash, nonce uint64) types.AddressHash {
	data, _ := rlp.EncodeToBytes([]interface{}{b, nonce}) // rlp encode
	return types.BytesToAddressHash(Keccak256(data)[12:]) // hash by Keccak256
}

// CreateAddress2 creates an ethereum address given the address bytes, initial
// contract code hash and a salt.
func CreateAddress2(b types.AddressHash, salt [32]byte, inithash []byte) types.AddressHash {
	return types.BytesToAddressHash(Keccak256([]byte{0xff}, b[:], salt[:], inithash)[12:])
}

// ValidateSignatureValues verifies whether the signature values are valid with
// the given chain rules. The v value is assumed to be either 0 or 1.
func ValidateSignatureValues(v byte, r, s *big.Int, homestead bool) bool {
	if r.Cmp(common.Big1) < 0 || s.Cmp(common.Big1) < 0 {
		return false
	}
	// reject upper range of s values (ECDSA malleability)
	// see discussion in secp256k1/libsecp256k1/include/secp256k1.h
	if homestead && s.Cmp(secp256k1halfN) > 0 {
		return false
	}
	// Frontier: allow s to be in full N range
	return r.Cmp(secp256k1N) < 0 && s.Cmp(secp256k1N) < 0 && (v == 0 || v == 1)
}
