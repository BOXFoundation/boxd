// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package murmur3

import (
	"testing"

	"github.com/facebookgo/ensure"
)

var data = []struct {
	seed uint32
	h32  uint32
	s    string
}{
	{0x00, 0x00000000, ""},
	{0x00, 0x248bfa47, "hello"},
	{0x00, 0x149bbb7f, "hello, world"},
	{0x00, 0xe31e8a70, "19 Jan 2038 at 3:14:07 AM"},
	{0x00, 0xd5c48bfc, "The quick brown fox jumps over the lazy dog."},

	{0x01, 0x514e28b7, ""},
	{0x01, 0xbb4abcad, "hello"},
	{0x01, 0x6f5cb2e9, "hello, world"},
	{0x01, 0xf50e1f30, "19 Jan 2038 at 3:14:07 AM"},
	{0x01, 0x846f6a36, "The quick brown fox jumps over the lazy dog."},

	{0x2a, 0x087fcd5c, ""},
	{0x2a, 0xe2dbd2e1, "hello"},
	{0x2a, 0x7ec7c6c2, "hello, world"},
	{0x2a, 0x58f745f6, "19 Jan 2038 at 3:14:07 AM"},
	{0x2a, 0xc02d1434, "The quick brown fox jumps over the lazy dog."},
}

func TestMurmurHash3(t *testing.T) {
	var data = []struct {
		seed uint32
		h32  uint32
		s    string
	}{
		{0x00, 0x00000000, ""},
		{0x00, 0x248bfa47, "hello"},
		{0x00, 0x149bbb7f, "hello, world"},
		{0x00, 0xe31e8a70, "19 Jan 2038 at 3:14:07 AM"},
		{0x00, 0xd5c48bfc, "The quick brown fox jumps over the lazy dog."},

		{0x01, 0x514e28b7, ""},
		{0x01, 0xbb4abcad, "hello"},
		{0x01, 0x6f5cb2e9, "hello, world"},
		{0x01, 0xf50e1f30, "19 Jan 2038 at 3:14:07 AM"},
		{0x01, 0x846f6a36, "The quick brown fox jumps over the lazy dog."},

		{0x2a, 0x087fcd5c, ""},
		{0x2a, 0xe2dbd2e1, "hello"},
		{0x2a, 0x7ec7c6c2, "hello, world"},
		{0x2a, 0x58f745f6, "19 Jan 2038 at 3:14:07 AM"},
		{0x2a, 0xc02d1434, "The quick brown fox jumps over the lazy dog."},
	}

	for _, elem := range data {
		ensure.DeepEqual(t, MurmurHash3WithSeed([]byte(elem.s), elem.seed), elem.h32)
	}
}

// test data from btcd
func TestMurmurHash3Data(t *testing.T) {
	var data = []struct {
		seed  uint32
		h32   uint32
		bytes []byte
	}{
		{0x00000000, 0x00000000, []byte{}},
		{0xfba4c795, 0x6a396f08, []byte{}},
		{0xffffffff, 0x81f16f39, []byte{}},
		{0x00000000, 0x514e28b7, []byte{0x00}},
		{0xfba4c795, 0xea3f0b17, []byte{0x00}},
		{0x00000000, 0xfd6cf10d, []byte{0xff}},
		{0x00000000, 0x16c6b7ab, []byte{0x00, 0x11}},
		{0x00000000, 0x8eb51c3d, []byte{0x00, 0x11, 0x22}},
		{0x00000000, 0xb4471bf8, []byte{0x00, 0x11, 0x22, 0x33}},
		{0x00000000, 0xe2301fa8, []byte{0x00, 0x11, 0x22, 0x33, 0x44}},
		{0x00000000, 0xfc2e4a15, []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}},
		{0x00000000, 0xb074502c, []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66}},
		{0x00000000, 0x8034d2a0, []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}},
		{0x00000000, 0xb4698def, []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}},
	}

	for _, elem := range data {
		if elem.seed == 0 {
			ensure.DeepEqual(t, MurmurHash3([]byte(elem.bytes)), elem.h32)
		} else {
			ensure.DeepEqual(t, MurmurHash3WithSeed([]byte(elem.bytes), elem.seed), elem.h32)
		}
	}
}
