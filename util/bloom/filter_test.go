// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bloom

import (
	"encoding/hex"
	"fmt"
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
	{"a5ddc786d9ef4658892ced1234b37bd6456a2fff", true},
}

func TestNewBloomFilterWithMK(t *testing.T) {
	var tests = []struct {
		m uint32
		k uint32
	}{
		{m: 32, k: 5},
		{m: 31, k: 2},
		{m: 128, k: 14},
		{m: 257, k: 21},
		{m: 257, k: MaxFilterHashFuncs + 1},
		{m: MaxFilterSize*8 + 1, k: 21},
	}
	for _, test := range tests {
		bf := NewFilterWithMK(test.m, test.k)
		if test.m > MaxFilterSize*8 {
			ensure.True(t, bf.Size() == MaxFilterSize*8)
		} else {
			ensure.True(t, bf.Size() >= test.m)
		}
		if test.k <= MaxFilterHashFuncs {
			ensure.DeepEqual(t, bf.K(), test.k)
		} else {
			ensure.DeepEqual(t, bf.K(), uint32(MaxFilterHashFuncs))
		}
		ensure.DeepEqual(t, bf.Tweak(), uint32(0))
	}
}

func TestNewBloomFilterWithMKAndSeed(t *testing.T) {
	var tests = []struct {
		m     uint32
		k     uint32
		tweak uint32
	}{
		{m: 32, k: 5, tweak: 0x123},
		{m: 31, k: 2, tweak: 0x123321},
		{m: 128, k: 14, tweak: 0xa867fce2},
		{m: 257, k: 21, tweak: 0xa61fcf98},
		{m: 257, k: MaxFilterHashFuncs + 1, tweak: 0},
		{m: MaxFilterSize*8 + 1, k: 21, tweak: 0},
	}
	for _, test := range tests {
		bf := NewFilterWithMKAndTweak(test.m, test.k, test.tweak)
		if test.m > MaxFilterSize*8 {
			ensure.True(t, bf.Size() == MaxFilterSize*8)
		} else {
			ensure.True(t, bf.Size() >= test.m)
		}
		if test.k <= MaxFilterHashFuncs {
			ensure.DeepEqual(t, bf.K(), test.k)
		} else {
			ensure.DeepEqual(t, bf.K(), uint32(MaxFilterHashFuncs))
		}
		ensure.DeepEqual(t, bf.Tweak(), test.tweak)
	}
}

func TestNewBloomFilter(t *testing.T) {
	bf := NewFilterWithTweak(1024, 0.000001, 0x7486f09a)
	ensure.True(t, bf.Size()/8 <= MaxFilterSize)

	bf = NewFilter(1024, 0.00000000000001)
	ensure.True(t, bf.Size()/8 <= MaxFilterSize)

	bf = NewFilter(1024, 1.1)
	ensure.True(t, bf.Size()/8 <= MaxFilterSize)

	bf = NewFilterWithMKAndTweak(512, 13, 0x092453)
	ensure.True(t, bf.Size() == 512)
}

func setupBloomFilter(t *testing.T) Filter {
	bf := NewFilterWithTweak(1024, 0.01, 0x83765463)
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
}

func TestFilterMatchesAndAdd(t *testing.T) {
	bf := setupBloomFilter(t)

	var c uint32
	for _, test := range tests {
		if test.insert {
			data, _ := hex.DecodeString(test.hex)
			ensure.True(t, bf.MatchesAndAdd(data))
		} else {
			data, _ := hex.DecodeString(test.hex)
			ensure.False(t, bf.MatchesAndAdd(data))
		}
		c++
	}
}

func TestFilterK(t *testing.T) {
	bf := setupBloomFilter(t)
	ensure.True(t, bf.K() > 5)
}

func TestFilterInserts(t *testing.T) {
	bf := NewFilter(1000, 0.0001)
	fprate := bf.FPRate()
	for i := uint32(0); i < 1000; i++ {
		if i%3 != 0 {
			bf.Add(util.FromUint32(i))
			nfp := bf.FPRate()
			ensure.True(t, nfp > fprate)
			fprate = nfp
		}
	}

	for i := uint32(0); i < 1000; i++ {
		if i%3 != 0 {
			ensure.True(t, bf.Matches(util.FromUint32(i)))
		} else {
			ensure.False(t, bf.Matches(util.FromUint32(i)), i)
		}
	}
}

func TestFilterFPRate(t *testing.T) {
	var size uint32 = 1000
	bf := NewFilter(size, 0.0001)
	for i := uint32(0); i < size; i++ {
		bf.Add(util.FromUint32(i))
	}

	var fprate = bf.FPRate()

	var tests = uint32(100. / fprate)
	var fails = 0
	for i := size; i < size+tests; i++ {
		if bf.Matches(util.FromUint32(i)) {
			fails++
		}
	}
	var rate = float64(fails) / float64(tests) / fprate
	ensure.True(t, rate < 1.1 && rate > 0.9)
}

func TestFilterMerge(t *testing.T) {
	bf0 := NewFilter(1000, 0.0001)
	for i := uint32(0); i < 2000; i++ {
		if i%3 == 0 {
			bf0.Add(util.FromUint32(i))
		}
	}
	bf1 := NewFilter(1000, 0.0001)
	for i := uint32(0); i < 2000; i++ {
		if i%3 == 1 {
			bf1.Add(util.FromUint32(i))
		}
	}

	ensure.Nil(t, bf0.Merge(bf1))
	for i := uint32(0); i < 2000; i++ {
		if i%3 == 0 || i%3 == 1 {
			ensure.True(t, bf0.Matches(util.FromUint32(i)))
		} else {
			ensure.False(t, bf0.Matches(util.FromUint32(i)))
		}
	}

}

func TestFilterMergeError(t *testing.T) {
	bf0 := NewFilterWithMK(1000, 23)

	bf1 := NewFilterWithMK(1000, 22)
	ensure.NotNil(t, bf0.Merge(bf1))

	bf2 := NewFilterWithMK(1001, 23)
	ensure.NotNil(t, bf0.Merge(bf2))

	bf3 := NewFilterWithMKAndTweak(1000, 23, 0x1111)
	ensure.NotNil(t, bf0.Merge(bf3))
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

func TestLargeSizeSerialize(t *testing.T) {
	bf := NewFilterWithMK(1024*1024, 128)
	var testData = make([][]byte, 128*1024)
	for i := 0; i < len(testData); i++ {
		testData[i] = []byte(fmt.Sprintf("%08d", i))
		bf.Add(testData[i])
	}
	data, err := bf.Marshal()
	ensure.Nil(t, err)

	bf2, err := LoadFilter(data)
	ensure.Nil(t, err)
	for i := 0; i < len(testData); i++ {
		ensure.True(t, bf2.Matches(testData[i]))
	}
}

func BenchmarkAdd(b *testing.B) {
	bf0 := NewFilter(1000, 0.0001)
	data := []byte("1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b.ResetTimer()
	for i := uint32(0); i < 10000000; i++ {
		bf0.Add(data)
	}
}

func BenchmarkMatch(b *testing.B) {
	bf0 := NewFilter(1000, 0.0001)
	data := []byte("1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	bf0.Add(data)
	b.ResetTimer()
	for i := uint32(0); i < 10000000; i++ {
		bf0.Matches(data)
	}
}

func BenchmarkMatchAndAdd(b *testing.B) {
	bf0 := NewFilter(1000, 0.0001)
	data := []byte("1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	bf0.Add(data)
	b.ResetTimer()
	for i := uint32(0); i < 10000000; i++ {
		bf0.MatchesAndAdd(data)
	}
}

func BenchmarkMatchAndAdd2(b *testing.B) {
	bf0 := NewFilter(1000, 0.0001)
	b.ResetTimer()
	for i := uint32(0); i < 10000000; i++ {
		bf0.MatchesAndAdd([]byte(fmt.Sprintf("%058d", i)))
	}
}
