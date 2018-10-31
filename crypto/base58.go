// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"fmt"

	"github.com/btcsuite/btcutil/base58"
)

// Base58CheckEncode calculates the 4 bytes checksum of input bytes,
// append checksum to the input bytes, and convert to base58 format
func Base58CheckEncode(in []byte) string {
	b := make([]byte, 0, len(in)+4)
	b = append(b, in[:]...)
	cksum := Checksum(in)
	b = append(b, cksum[:]...)
	return base58.Encode(b)
}

// Checksum return input bytes checksum.
func Checksum(input []byte) (cksum [4]byte) {
	h := Sha256(Sha256(input))
	copy(cksum[:], h[:4])
	return
}

// Base58CheckDecode converts a base58 format string to byte array,
// checks the checksum and returns the wrapped byte array content
func Base58CheckDecode(in string) ([]byte, error) {
	rawBytes := base58.Decode(in)
	if len(rawBytes) < 5 {
		return nil, fmt.Errorf("Error decoding base58: %s", in)
	}
	var cksum [4]byte
	sep := len(rawBytes) - 4
	content := make([]byte, sep)
	copy(cksum[:], rawBytes[sep:])
	copy(content, rawBytes[:sep])
	if Checksum(content) != cksum {
		return nil, fmt.Errorf("Error checksum of base58: %s", in)
	}
	return content, nil
}
