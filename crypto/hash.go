// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/ripemd160"
)

const (
	// HashSize is length of digest
	HashSize = 32
)

// HashType is renamed hash type
type HashType [HashSize]byte

// String returns the Hash as the hexadecimal string of the byte-reversed
// hash.
func (hash HashType) String() string {
	for i := 0; i < HashSize/2; i++ {
		hash[i], hash[HashSize-1-i] = hash[HashSize-1-i], hash[i]
	}
	return hex.EncodeToString(hash[:])
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

// DoubleHashH calculates hash(hash(b)) and returns the resulting bytes as a hash.
func DoubleHashH(b []byte) HashType {
	first := sha256.Sum256(b)
	return HashType(sha256.Sum256(first[:]))
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
	if len(hashBytes) != HashSize {
		return fmt.Errorf("Incorrect hash length : %v", hashBytes)
	}
	copy(hash[:], hashBytes)
	return nil
}
