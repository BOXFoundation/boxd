// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	"sync"

	"github.com/golang/snappy"
)

var (
	// msgToCompressioncompress store the corresponding compression status to msg type
	msgToCompression = new(sync.Map)

	compressFlag = 1 << 7
)

// MaxEncodedLen = 0xffffffff 3GB
func compress(dst, src []byte) []byte {
	return snappy.Encode(dst, src)
}

func decompress(dst, src []byte) ([]byte, error) {
	return snappy.Decode(dst, src)
}
