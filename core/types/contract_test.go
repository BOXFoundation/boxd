// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"math"
	"strings"
	"testing"

	"github.com/facebookgo/ensure"

	"github.com/BOXFoundation/boxd/crypto"
)

func makeBytes(len int, content byte) []byte {
	res := make([]byte, len, len)
	for i := 0; i < len; i++ {
		res[i] = content
	}
	return res
}

func hashWithBytes(buf []byte) (*crypto.HashType, error) {
	hash := &crypto.HashType{}
	if err := hash.SetBytes(buf); err != nil {
		return nil, err
	}
	return hash, nil
}

func TestToken_EnsurePrefix(t *testing.T) {
	minByte := makeBytes(crypto.HashSize, 0)
	minHash, err := hashWithBytes(minByte)
	ensure.Nil(t, err)
	cMin := Token{
		op: OutPoint{
			Hash:  *minHash,
			Index: 0,
		},
	}
	ensure.True(t, strings.HasPrefix(cMin.String(), "b4"))

	maxByte := makeBytes(crypto.HashSize, 255)
	maxHash, err := hashWithBytes(maxByte)
	ensure.Nil(t, err)
	cMax := Token{
		op: OutPoint{
			Hash:  *maxHash,
			Index: math.MaxUint32,
		},
	}
	ensure.True(t, strings.HasPrefix(cMax.String(), "b4"))
}
