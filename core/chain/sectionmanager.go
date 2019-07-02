// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/util/bloom"
)

const (
	// SectionBloomLength represents the number of blooms used in a new section.
	SectionBloomLength = 65536 >> 3
)

// SectionManager takes a number of bloom filters and generates the rotated bloom bits
// to be used for batched filtering.
type SectionManager struct {
	blooms  [types.BloomBitLength][SectionBloomLength]byte // Rotated blooms for per-bit matching
	section uint32                                         // Section is the section number being processed currently
	// Offset uint                                     // Next bit position to set when adding a bloom

	db storage.Table
}

// NewSectionManager creates a rotated bloom section manager that can iteratively fill a
// batched bloom filter's bits.
func NewSectionManager(db storage.Storage) (mgr *SectionManager, err error) {
	mgr = &SectionManager{section: 1}
	if mgr.db, err = db.Table(SectionTableName); err != nil {
		return nil, err
	}
	return mgr, nil
}

// AddBloom takes a single bloom filter and sets the corresponding bit column
// in memory accordingly.
func (sm *SectionManager) AddBloom(index uint, bloom bloom.Filter) error {

	// TODO: mutex?

	index = index % SectionBloomLength

	byteIndex := index / 8
	bitMask := byte(1) << byte(7-index%8)

	// Rotate the bloom and insert into our collection
	for i := 0; i < types.BloomBitLength; i++ {
		bloomByteIndex := types.BloomByteLength - 1 - i/8
		bloomBitMask := byte(1) << byte(i%8)

		if (bloom.GetByte(uint32(bloomByteIndex)) & bloomBitMask) != 0 {
			sm.blooms[i][byteIndex] |= bitMask
		}
	}
	// sm.Offset++

	// store section into db
	if index == SectionBloomLength-1 {
		if err := sm.commit(); err == nil {
			sm.section++
		} else {
			logger.Errorf("Failed to commit section manager. Err: %v", err)
			return err
		}
	}
	return nil
}

// RemoveBloom takes a single bloom filter and reset the corresponding bit column
// in memory accordingly.
func (sm *SectionManager) RemoveBloom(index uint) {

	// TODO: 库里修改

	index = index % SectionBloomLength

	byteIndex := index / 8
	bitMask := ^(byte(1) << byte(7-index%8))

	// Rotate the bloom and insert into our collection
	for i := 0; i < types.BloomBitLength; i++ {
		sm.blooms[i][byteIndex] &= bitMask
	}
}

// Bitset returns the bit vector belonging to the given bit index after all
// blooms have been added.
func (sm *SectionManager) Bitset(idx uint) ([]byte, error) {
	if idx >= types.BloomBitLength {
		return nil, core.ErrBloomBitOutOfBounds
	}
	return sm.blooms[idx][:], nil
}

func (sm *SectionManager) commit() error {
	sm.db.EnableBatch()
	defer sm.db.DisableBatch()
	for i := 0; i < types.BloomBitLength; i++ {
		bits, err := sm.Bitset(uint(i))
		if err != nil {
			return err
		}
		if err := sm.db.Put(SecBloomBitSetKey(sm.section, uint(i)), bits); err != nil {
			return err
		}
		// a := [SectionBloomLength]byte{}
		// if !bytes.Equal(bits, a[:]) {
		// 	fmt.Println("PUT: ", i)
		// }
	}
	return sm.db.Flush()
}

// Index get blocks' height matchs key words.
func (sm *SectionManager) Index(from, to uint32, topicslist [][][]byte) ([]uint32, error) {

	section := sm.section
	start := from/SectionBloomLength + 1
	end := to/SectionBloomLength + 1
	if start > end {
		return nil, core.ErrInvalidBounds
	}
	// TODO: from to 异常处理

	if start >= section {
		start = section - 1
	}
	if end >= section {
		end = section - 1
	}

	heights := []uint32{}
	if start != 0 {
		for i := 0; i <= int(end-start); i++ {
			h, err := sm.indexed(start, topicslist)
			if err != nil {
				return nil, err
			}
			if i == 0 || i == int(start-end) {
				for _, hh := range h {
					if hh >= from && hh <= to {
						heights = append(heights, hh)
					}
				}
			} else {
				heights = append(heights, h...)
			}
		}
	}
	if to%SectionBloomLength != 0 {
		h, err := sm.unIndexed(end*SectionBloomLength+1, to, topicslist)
		if err != nil {
			return nil, err
		}
		heights = append(heights, h...)
	}

	return heights, nil
}

func (sm *SectionManager) indexed(section uint32, topicslist [][][]byte) ([]uint32, error) {
	/*
		创建一个[]byte result保存最终的结果
		创建bloom filter
		for 一个地址内的所有的topics list {
			创建一个[]byte matcher保存每个位置的匹配结果
			for 一个topic位置的所有可能 {
				bloom.Reset()
				bloom.Add(topic)
				获得bloom的三个下标
				创建一个[]byte tmp保存bitset的最终结果
				for indexes {
					在section里找到对应的bitset
					tmp &= bitset
				}
				matcher |= tmp
			}
			result &= matcher
		}
		result一共65536位, 取对应为1的位置获得高度
	*/

	heightMask := [SectionBloomLength]byte{}
	bf := bloom.NewFilterWithMK(types.BloomBitLength, types.BloomHashNum)
	for i, topics := range topicslist {

		topicMatcher := [SectionBloomLength]byte{}

		for j, topic := range topics {
			bf.Reset()
			bf.Add(topic)
			indexes := bf.Indexes()

			tmp := [SectionBloomLength]byte{}
			for k, index := range indexes {
				index = types.BloomBitLength - 1 - index
				bits, err := sm.db.Get(SecBloomBitSetKey(section, uint(index)))
				if err != nil {
					logger.Errorf("Get section failed. Err: %v", err)
					break
				}
				if k == 0 {
					copy(tmp[:], bits)
				} else {
					copy(tmp[:], util.ANDBytes(tmp[:], bits))
				}
			}
			if j == 0 {
				copy(topicMatcher[:], tmp[:])
			} else {
				copy(topicMatcher[:], util.ORBytes(topicMatcher[:], tmp[:]))
			}
		}
		if i == 0 {
			copy(heightMask[:], topicMatcher[:])
		} else {
			copy(heightMask[:], util.ANDBytes(heightMask[:], topicMatcher[:]))
		}
	}

	heights := util.BitIndexes(heightMask[:])
	for i := 0; i < len(heights); i++ {
		heights[i] += (section - 1) * SectionBloomLength
	}
	return heights, nil
}

func (sm *SectionManager) unIndexed(from, to uint32, topicslist [][][]byte) ([]uint32, error) {
	return nil, nil
}
