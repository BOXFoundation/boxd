// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util/bloom"
)

const (
	// SectionBloomLength represents the number of blooms used in a new section.
	SectionBloomLength = 65535
)

// SectionManager takes a number of bloom filters and generates the rotated bloom bits
// to be used for batched filtering.
type SectionManager struct {
	blooms  [types.BloomBitLength][SectionBloomLength]byte // Rotated blooms for per-bit matching
	section uint64                                         // Section is the section number being processed currently
	// Offset uint                                     // Next bit position to set when adding a bloom

	db storage.Table
}

// NewSectionManager creates a rotated bloom section manager that can iteratively fill a
// batched bloom filter's bits.
func NewSectionManager(db storage.Storage) (mgr *SectionManager, err error) {
	mgr = &SectionManager{}
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
	}
	return sm.db.Flush()
}

type section struct {
	blooms [types.BloomBitLength][SectionBloomLength]byte
	index  uint32
}
