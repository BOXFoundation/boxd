// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bloom

import (
	"encoding/hex"
	"testing"

	"github.com/BOXFoundation/boxd/util"
	"github.com/facebookgo/ensure"
)

var tests = []struct {
	hex    string
	insert bool
}{
	{"99108ad8ed9bb6274d3980bab5a85c048f0950c8", true},
	{"19108ad8ed9bb6274d3980bab5a85c048f0950c8", false},
	{"b5a2c786d9ef4658287ced5914b37a1b4aa32eee", true},
	{"b9300670b4c5366e95b2699e8b18bc75e5f729c5", true},
	{"a5ddc786d9ef4658287ced5914b37a1b456a2fff", true},
}

func TestNewBloomFilter(t *testing.T) {
	bf := NewFilter(1024, 0x7486f09a, 0.000001)
	ensure.True(t, bf.Size()/8 <= MaxFilterSize)

	bf = NewFilter(1024, 0x7486f09a, 0.00000000000001)
	ensure.True(t, bf.Size()/8 <= MaxFilterSize)

	bf = NewFilter(1024, 0x7486f09a, 1.1)
	ensure.True(t, bf.Size()/8 <= MaxFilterSize)
}

func setupBloomFilter(t *testing.T) Filter {
	bf := NewFilter(1024, 0, 0.01)
	for _, test := range tests {
		data, err := hex.DecodeString(test.hex)
		ensure.Nil(t, err)
		if test.insert {
			bf.Add(data)
		}
	}
	return bf
}

func assertBloomFilter(t *testing.T, bf Filter) {
	for _, test := range tests {
		data, _ := hex.DecodeString(test.hex)
		ensure.DeepEqual(t, bf.Matches(data), test.insert)
	}
}

func TestFilterInsert(t *testing.T) {
	bf := setupBloomFilter(t)
	assertBloomFilter(t, bf)

	var c uint32
	for _, test := range tests {
		if test.insert {
			c++
		}
	}
	ensure.DeepEqual(t, bf.Elements(), c)
}

func TestFilterInserts(t *testing.T) {
	bf := NewFilter(33, 0x7486f09a, 0.01)
	fprate := bf.FPRate()
	for i := uint32(0); i < 100; i++ {
		if i%3 != 0 {
			bf.Add(util.FromUint32(i))
			nfp := bf.FPRate()
			ensure.True(t, nfp >= fprate)
			fprate = nfp
		}
	}

	for i := uint32(0); i < 100; i++ {
		if i%3 != 0 {
			ensure.True(t, bf.Matches(util.FromUint32(i)))
		} else {
			ensure.False(t, bf.Matches(util.FromUint32(i)), i)
		}
	}
}

func TestFilterElements(t *testing.T) {
	bf := setupBloomFilter(t)
	var c uint32
	for _, test := range tests {
		if test.insert {
			c++
		}
	}
	ensure.DeepEqual(t, bf.Elements(), c)
}

func TestFilterSerialize(t *testing.T) {
	bf := setupBloomFilter(t)
	data, err := bf.Marshal()
	ensure.Nil(t, err)

	bf2, err := LoadFilter(data)
	ensure.Nil(t, err)
	assertBloomFilter(t, bf2)

	ensure.DeepEqual(t, bf, bf2)
}
