// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bloom

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"sync"

	conv "github.com/BOXFoundation/boxd/p2p/convert"
	"github.com/BOXFoundation/boxd/util/murmur3"
)

const ln2Squared = math.Ln2 * math.Ln2

const (
	// MaxFilterHashFuncs is the maximum number of hash functions of bloom filter.
	MaxFilterHashFuncs = 256

	// MaxFilterSize is the maximum byte size in bytes a filter may be.
	MaxFilterSize = 1024 * 1024
)

// Filter defines bloom filter interface
type Filter interface {
	Matches(data []byte) bool
	Add(data []byte)
	MatchesAndAdd(data []byte) bool
	Merge(f Filter) error

	Size() uint32
	K() uint32
	Elements() uint32
	Tweak() uint32
	FPRate() float64

	conv.Serializable
}

// filter implements the bloom-filter
type filter struct {
	sm        sync.Mutex
	filter    []byte
	hashFuncs uint32
	tweak     uint32
	elements  uint32
}

// NewFilterWithMK returns a Filter.
// M is the cap of the bloom filter
// K is the number of hash functions
func NewFilterWithMK(m uint32, k uint32) Filter {
	return newFilter(m, k, 0)
}

// NewFilterWithMKAndTweak returns a Filter with specific tweak for hash seed.
// M is the cap of the bloom filter
// K is the number of hash functions
func NewFilterWithMKAndTweak(m uint32, k uint32, tweak uint32) Filter {
	return newFilter(m, k, tweak)
}

func newFilter(m uint32, k uint32, tweak uint32) Filter {
	if m <= 0 {
		m = 1
	}
	if k <= 0 {
		k = 1
	}

	dataLen := uint32(math.Ceil(float64(m) / 8.))
	if MaxFilterSize < dataLen {
		dataLen = MaxFilterSize
	}
	hashFuncs := k
	if MaxFilterHashFuncs < hashFuncs {
		hashFuncs = MaxFilterHashFuncs
	}
	data := make([]byte, dataLen)

	return &filter{
		filter:    data,
		hashFuncs: hashFuncs,
		tweak:     tweak,
		elements:  0,
	}
}

// NewFilter returns a Filter
func NewFilter(elements uint32, fprate float64) Filter {
	return NewFilterWithTweak(elements, fprate, 0)
}

// NewFilterWithTweak returns a Filter with with specific tweak for hash seed
func NewFilterWithTweak(elements uint32, fprate float64, tweak uint32) Filter {
	// Massage the false positive rate to sane values.
	if fprate > 1.0 {
		fprate = 1.0
	} else if fprate < 1e-9 {
		fprate = 1e-9
	}

	// Calculate the size of the filter in bits for the given number of
	// elements and false positive rate.
	m := uint32(math.Ceil(float64(-1 * float64(elements) * math.Log(fprate) / ln2Squared)))

	// Calculate the number of hash functions based on the size of the
	// filter calculated above and the number of elements.
	k := uint32(math.Ceil(math.Ln2 * float64(m) / float64(elements)))

	return newFilter(m, k, tweak)
}

// LoadFilter loads bloom filter from serialized data.
func LoadFilter(data []byte) (Filter, error) {
	var f = &filter{}
	if err := f.Unmarshal(data); err != nil {
		return nil, err
	}
	return f, nil
}

// hash returns the bit offset in the bloom filter which corresponds to the
// passed data for the given indepedent hash function number.
func (bf *filter) hash(hashFuncNum uint32, data []byte) uint32 {
	h32 := murmur3.MurmurHash3WithSeed(data, hashFuncNum*0xfba4c795+bf.tweak)
	return h32 % (uint32(len(bf.filter)) << 3)
}

// matches returns true if the bloom filter might contain the passed data and
// false if it definitely does not.
func (bf *filter) matches(data []byte) bool {
	// idx >> 3 = idx / 8 : byte index in the byte slice
	// 1<<(idx&7) = idx % 8 : bit index in the byte
	for i := uint32(0); i < bf.hashFuncs; i++ {
		idx := bf.hash(i, data)
		if bf.filter[idx>>3]&(1<<(idx&7)) == 0 {
			return false
		}
	}
	return true
}

// Matches returns true if the bloom filter might contain the passed data and
// false if it definitely does not.
func (bf *filter) Matches(data []byte) bool {
	bf.sm.Lock()
	match := bf.matches(data)
	bf.sm.Unlock()
	return match
}

// add adds the passed byte slice to the bloom filter.
func (bf *filter) add(data []byte) {
	for i := uint32(0); i < bf.hashFuncs; i++ {
		idx := bf.hash(i, data)
		bf.filter[idx>>3] |= (1 << (7 & idx))
	}
	bf.elements++
}

// Add adds the passed byte slice to the bloom filter.
func (bf *filter) Add(data []byte) {
	bf.sm.Lock()
	bf.add(data)
	bf.sm.Unlock()
}

// matchesAndAdd adds the passed byte slice to the bloom filter.
func (bf *filter) matchesAndAdd(data []byte) (r bool) {
	r = true
	for i := uint32(0); i < bf.hashFuncs; i++ {
		idx := bf.hash(i, data)
		if bf.filter[idx>>3]&(1<<(idx&7)) == 0 {
			bf.filter[idx>>3] |= (1 << (7 & idx))
			r = false
		}
	}
	if !r {
		bf.elements++
	}
	return r
}

// MatchesAndAdd matchs the passed data, and add it to the bloom filter in case of false
func (bf *filter) MatchesAndAdd(data []byte) (r bool) {
	bf.sm.Lock()
	match := bf.matchesAndAdd(data)
	bf.sm.Unlock()
	return match
}

// merge merges the passed one with the bloom filter.
func (bf *filter) merge(f Filter) error {
	var rf, ok = f.(*filter)
	if !ok {
		return errors.New("unknown bloom filter")
	}

	rf.sm.Lock()
	defer rf.sm.Unlock()

	if bf.hashFuncs != rf.hashFuncs {
		return errors.New("HashFuncs doesn't match")
	}
	if len(bf.filter) != len(rf.filter) {
		return errors.New("Size of filter bits doesn't match")
	}
	if bf.tweak != rf.tweak {
		return errors.New("Seed of hash doesn't match")
	}

	for i := 0; i < len(bf.filter); i++ {
		bf.filter[i] |= rf.filter[i]
	}
	// TODO: total elements
	bf.elements += rf.elements

	return nil
}

// Merge merges the passed one with the bloom filter.
func (bf *filter) Merge(f Filter) error {
	bf.sm.Lock()
	err := bf.merge(f)
	bf.sm.Unlock()
	return err
}

// Elements returns the number of elements inserted to the filter.
func (bf *filter) Elements() uint32 {
	bf.sm.Lock()
	e := bf.elements
	bf.sm.Unlock()
	return e
}

// Size returns the length of filter in bits.
func (bf *filter) Size() uint32 {
	bf.sm.Lock()
	s := uint32(len(bf.filter)) << 3
	bf.sm.Unlock()
	return s
}

// Size returns the number of hash functions.
func (bf *filter) K() uint32 {
	return bf.hashFuncs
}

// Tweak returns the random value added to the hash seed.
func (bf *filter) Tweak() uint32 {
	return bf.tweak
}

// FPRate returns the false positive probability
func (bf *filter) FPRate() float64 {
	return math.Exp(-ln2Squared * float64(bf.Size()) / float64(bf.Elements()))
}

// Marshal marshals the bloom filter to a binary representation of it.
func (bf *filter) Marshal() (data []byte, err error) {
	var buf bytes.Buffer
	var w = bufio.NewWriter(&buf)
	if err := binary.Write(w, binary.LittleEndian, bf.elements); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.LittleEndian, bf.hashFuncs); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.LittleEndian, bf.tweak); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(bf.filter))); err != nil {
		return nil, err
	}
	if err := binary.Write(w, binary.LittleEndian, bf.filter); err != nil {
		return nil, err
	}
	w.Flush()
	return buf.Bytes(), nil
}

// Unmarshal unmarshals a bloom filter from binary data.
func (bf *filter) Unmarshal(data []byte) error {
	var buf = bytes.NewBuffer(data)
	var r = bufio.NewReader(buf)
	var l uint32
	if err := binary.Read(r, binary.LittleEndian, &bf.elements); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &bf.hashFuncs); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &bf.tweak); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &l); err != nil {
		return err
	}

	bf.filter = make([]byte, l)
	return binary.Read(r, binary.LittleEndian, bf.filter)
}
