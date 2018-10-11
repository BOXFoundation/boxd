// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package murmur3

import (
	"encoding/binary"
)

const (
	c1 uint32 = 0xcc9e2d51
	c2 uint32 = 0x1b873593
)

// MurmurHash3 returns the MurmurHash3 sum of data.
func MurmurHash3(data []byte) uint32 {
	return MurmurHash3WithSeed(data, 0)
}

// MurmurHash3WithSeed returns the MurmurHash3 sum of data.
func MurmurHash3WithSeed(data []byte, seed uint32) uint32 {
	hash := seed
	nlen := uint32(len(data))
	nblocks := uint32(nlen / 4)

	for i := uint32(0); i < nblocks; i++ {
		k := binary.LittleEndian.Uint32(data[i*4:])

		k *= c1
		k = (k << 15) | (k >> 17)
		k *= c2

		hash ^= k
		hash = (hash << 13) | (hash >> 19)
		hash = (hash << 2) + hash + 0xe6546b64
	}

	tail := data[nblocks*4:]

	var k uint32
	switch len(tail) & 3 {
	case 3:
		k ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		k ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		k ^= uint32(tail[0])
		k *= c1
		k = (k << 15) | (k >> 17)
		k *= c2
		hash ^= k
	}

	hash ^= uint32(nlen)

	hash ^= hash >> 16
	hash *= 0x85ebca6b
	hash ^= hash >> 13
	hash *= 0xc2b2ae35
	hash ^= hash >> 16

	return hash
}
