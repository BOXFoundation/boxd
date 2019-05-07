// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/sha3"
)

const (
	// HashSize is length of digest
	HashSize = 32
)

var (
	// ZeroHash is a HashType full of 0
	ZeroHash = HashType{}
)

// HashType is renamed hash type
type HashType [HashSize]byte

// String returns the Hash as the hexadecimal string of the byte-reversed
// hash.
func (hash HashType) String() string {
	buf := hash.GetBytes()
	reverseBytes(buf)
	return hex.EncodeToString(buf)
}

// MarshalJSON returns hash of hex type as the JSON encoding of HashType
func (hash HashType) MarshalJSON() ([]byte, error) {
	return json.Marshal(hash.String())
}

// SetString sets hash using bytes parsed from string
// returns error if input string is not valid hex string
func (hash *HashType) SetString(str string) error {
	buf, err := hex.DecodeString(str)
	if err != nil {
		return err
	}
	if len(buf) != HashSize {
		return fmt.Errorf("invalid hash length")
	}
	reverseBytes(buf)
	hash.SetBytes(buf)
	return nil
}

func reverseBytes(buf []byte) {
	length := len(buf)
	for i := 0; i < length/2; i++ {
		buf[i], buf[length-i-1] = buf[length-i-1], buf[i]
	}
}

// Ripemd160 calculates the RIPEMD160 digest of buf
func Ripemd160(buf []byte) []byte {
	hasher := ripemd160.New()
	hasher.Write(buf)
	return hasher.Sum(nil)
}

// Sha256 calculates the sha256 digest of buf
func Sha256(buf []byte) []byte {
	digest := sha256.Sum256(buf)
	return digest[:]
}

// Sha3256 returns the SHA3-256 digest of the data.
func Sha3256(args ...[]byte) []byte {
	hasher := sha3.New256()
	for _, bytes := range args {
		hasher.Write(bytes)
	}
	return hasher.Sum(nil)
}

// Sha256Multi calculates the sha256 digest of buf array
func Sha256Multi(data ...[]byte) []byte {
	h := sha256.New()
	h.Reset()
	for _, d := range data {
		h.Write(d)
	}
	return h.Sum(nil)
}

// DoubleHashH calculates hash(hash(b)) and returns the resulting bytes as a hash.
func DoubleHashH(b []byte) HashType {
	first := sha256.Sum256(b)
	return HashType(sha256.Sum256(first[:]))
}

// Hash160 calculates the hash ripemd160(sha256(b)).
func Hash160(b []byte) []byte {
	return Ripemd160(Sha256(b))
}

// IsEqual returns true if target is the same as hash.
func (hash *HashType) IsEqual(target *HashType) bool {
	if hash == nil && target == nil {
		return true
	}
	if hash == nil || target == nil {
		return false
	}
	return *hash == *target
}

// SetBytes convert type []byte to HashType
func (hash *HashType) SetBytes(hashBytes []byte) error {
	if len(hashBytes) > len(hash) {
		hashBytes = hashBytes[len(hashBytes)-HashSize:]
	}
	copy(hash[HashSize-len(hashBytes):], hashBytes)
	return nil
}

// GetBytes convert type HashType to []byte
func (hash *HashType) GetBytes() []byte {
	hashBytes := make([]byte, HashSize)
	copy(hashBytes, hash[:])
	return hashBytes
}

func (hash HashType) Bytes() []byte { return hash[:] }

func (hash *HashType) Big() *big.Int { return new(big.Int).SetBytes(hash[:]) }

func BigToHash(b *big.Int) HashType {
	var h HashType
	bb := b.Bytes()

	if len(bb) > len(h) {
		bb = bb[len(bb)-HashSize:]
	}

	copy(h[HashSize-len(bb):], bb)

	return BytesToHash(h.Bytes())
}

func BytesToHash(b []byte) HashType {
	var h HashType
	h.SetBytes(b)
	return h
}
