// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"crypto/sha256"

	"golang.org/x/crypto/ripemd160"
)

const (
	// HashSize is length of digest
	HashSize = 32
)

// HashType is renamed hash type
type HashType [HashSize]byte

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
