// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"bytes"
	"sync"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util"
	"github.com/BOXFoundation/boxd/util/bloom"
)

//
const (
	// SectionBloomByteLength represents the number of blooms used in a new section.
	SectionBloomByteLength = SectionBloomBitLength >> 3
	SectionBloomBitLength  = 8192
	// SectionBloomBitLength  = 32
)

// SectionManager takes a number of bloom filters and generates the rotated bloom bits
// to be used for batched filtering.
type SectionManager struct {
	blooms  [types.BloomBitLength][SectionBloomByteLength]byte // Rotated blooms for per-bit matching
	section uint32                                             // Section is the section number being processed currently
	height  uint32                                             // Last block height added into bloom

	chain *BlockChain
	db    storage.Table
	mtx   sync.Mutex
}

// NewSectionManager creates a rotated bloom section manager that can iteratively fill a
// batched bloom filter's bits.
func NewSectionManager(chain *BlockChain, db storage.Storage) (mgr *SectionManager, err error) {
	mgr = &SectionManager{chain: chain}
	if mgr.db, err = db.Table(SectionTableName); err != nil {
		return nil, err
	}
	return mgr, nil
}

// AddBloom takes a single bloom filter and sets the corresponding bit column
// in memory accordingly.
func (sm *SectionManager) AddBloom(index uint32, bloom bloom.Filter) error {
	if index <= sm.height && sm.height != 0 {
		return nil
	}
	if index > sm.height+1 {
		block, err := sm.chain.LoadBlockByHeight(index - 1)
		if err != nil {
			return err
		}
		err = sm.AddBloom(index-1, block.Header.Bloom)
		if err != nil {
			return err
		}
	}

	sm.mtx.Lock()
	defer sm.mtx.Unlock()

	h := index
	index = index % SectionBloomBitLength

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

	// store section into db
	if index == SectionBloomBitLength-1 {
		if err := sm.commit(); err == nil {
			sm.blooms = [types.BloomBitLength][SectionBloomByteLength]byte{}
			sm.section++
		} else {
			logger.Errorf("Failed to commit section manager. Err: %v", err)
			return err
		}
	}
	sm.height = h
	return nil
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
	txn, err := sm.db.NewTransaction()
	if err != nil {
		return err
	}
	defer txn.Discard()

	for i := 0; i < types.BloomBitLength; i++ {
		bits, err := sm.Bitset(uint(i))
		if err != nil {
			return err
		}
		if err := txn.Put(SecBloomBitSetKey(sm.section, uint(i)), bits); err != nil {
			return err
		}
		// a := [SectionBloomByteLength]byte{}
		// if !bytes.Equal(bits, a[:]) {
		// 	logger.Errorf("PUT: %d, section: %d", i, sm.section)
		// }
	}
	return txn.Commit()
}

// GetLogs get logs matchs key words.
func (sm *SectionManager) GetLogs(from, to uint32, topicslist [][][]byte) ([]*types.Log, error) {

	tail := sm.chain.TailBlock()
	if to > tail.Header.Height {
		to = tail.Header.Height
	}

	section := sm.section
	start := from / SectionBloomBitLength
	end := to / SectionBloomBitLength
	if start > end {
		return nil, core.ErrInvalidBounds
	}

	if start > section {
		return nil, nil
	}
	if end >= section {
		end = section - 1
	}

	heights := []uint32{}
	if start < section {
		for i := 0; i <= int(end-start); i++ {
			h, err := sm.indexed(start+uint32(i), topicslist)
			if err != nil {
				return nil, err
			}
			if i == 0 || i == int(end-start) {
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
	if to > sm.section*SectionBloomBitLength {
		h, err := sm.unIndexed(section*SectionBloomBitLength, to, topicslist)
		logger.Errorf("get2 heights: %v", h)
		if err != nil {
			return nil, err
		}
		heights = append(heights, h...)
	}
	logger.Debugf("HEIGHT: %d", heights)

	return sm.getLogs(heights, topicslist)
}

func (sm *SectionManager) indexed(section uint32, topicslist [][][]byte) ([]uint32, error) {
	/*
		创建一个[]byte result保存最终的结果
		创建bloom filter
		for 一个地址内的所有的topics list {
			创建一个[]byte matcher保存每个位置的匹配结果
			if 不是第一二个位置(因为是合约地址s和topic ids) && len==0 {
				continue (因为空集合匹配所有indexes)
			}
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

	heightMask := [SectionBloomByteLength]byte{}
	bf := bloom.NewFilterWithMK(types.BloomBitLength, types.BloomHashNum)
	for i, topics := range topicslist {

		topicMatcher := [SectionBloomByteLength]byte{}
		if i > 1 && len(topics) == 0 {
			continue
		}

		for j, topic := range topics {
			bf.Reset()
			bf.Add(topic)
			indexes := bf.Indexes()

			tmp := [SectionBloomByteLength]byte{}
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

				// a := [SectionBloomByteLength]byte{}
				// if !bytes.Equal(tmp[:], a[:]) {
				// 	logger.Errorf("PUT: true index: %d, %d", index, section)
				// } else {
				// 	logger.Errorf("PUT: false index: %d, %d", index, section)
				// }
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
		heights[i] += section * SectionBloomBitLength
	}
	return heights, nil
}

func (sm *SectionManager) unIndexed(from, to uint32, topicslist [][][]byte) ([]uint32, error) {

	var heights []uint32
	for ; from <= to; from++ {
		block, err := sm.chain.LoadBlockByHeight(from)
		if err != nil {
			logger.Errorf("Load block failed. Height: %d, Err: %v", from, err)
			continue
		}

		if sm.bloomFilter(block.Header.Bloom, topicslist) {
			heights = append(heights, from)
		}

	}

	return heights, nil
}

func (sm *SectionManager) getLogs(heights []uint32, topicslist [][][]byte) ([]*types.Log, error) {
	var ret []*types.Log
	for _, height := range heights {
		block, err := sm.chain.LoadBlockByHeight(height)
		if err != nil {
			return nil, err
		}
		logs, err := sm.chain.GetBlockLogs(block.Hash)
		if err != nil {
			logger.Errorf("Get block failed. Err: %v", err)
			continue
		}
		logs, err = sm.filterLogs(logs, topicslist)
		if err != nil {
			logger.Errorf("Filter logs failed. Err: %v", err)
			continue
		}
		ret = append(ret, logs...)
	}
	return ret, nil
}

func (sm *SectionManager) filterLogs(logs []*types.Log, topicslist [][][]byte) ([]*types.Log, error) {

	ret := []*types.Log{}
	addrs := topicslist[0]
LOGS:
	for _, log := range logs {
		if len(addrs) > 0 && !include(addrs, log.Address.Bytes()) {
			continue
		}
		if len(topicslist)-1 > len(log.Topics) {
			continue
		}

		for i, sub := range topicslist[1:] {
			match := len(sub) == 0 // empty rule set == wildcard
			for _, topic := range sub {
				if bytes.Equal(log.Topics[i].Bytes(), topic) {
					match = true
					break
				}
			}
			if !match {
				continue LOGS
			}
		}
		ret = append(ret, log)
	}
	return ret, nil
}

func (sm *SectionManager) bloomFilter(bf bloom.Filter, topicslist [][][]byte) bool {
	if len(topicslist[0]) > 0 {
		var included bool
		for _, addr := range topicslist[0] {
			bb := bloom.NewFilterWithMK(types.BloomBitLength, 3)
			bb.Add(addr)
			if bf.Matches(addr) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	for _, sub := range topicslist[1:] {
		included := len(sub) == 0 // empty rule set == wildcard
		for _, topic := range sub {
			if bf.Matches(topic) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}
	return true
}

func include(s [][]byte, d []byte) bool {
	for _, ss := range s {
		if bytes.Equal(ss, d) {
			return true
		}
	}
	return false
}
