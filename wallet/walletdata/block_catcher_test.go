// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletdata

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/facebookgo/ensure"

	"github.com/BOXFoundation/boxd/crypto"

	"github.com/BOXFoundation/boxd/core/types"
)

var heightControl uint32

type fakeFetcher struct {
	height    uint32
	obfs      uint32
	obfsStart uint32
}

func fakeFetcherWithHeight(height, obfs, obfsStart uint32) BlockFetcher {
	return &fakeFetcher{
		height:    height,
		obfs:      obfs,
		obfsStart: obfsStart,
	}
}

func (f *fakeFetcher) Height() (uint32, error) {
	return f.height, nil
}

func (f *fakeFetcher) HashForHeight(h uint32) (string, error) {
	if h >= f.obfsStart {
		return hashForHeightWithObfs(h, f.obfs), nil
	}
	return hashForHeightWithObfs(h, 0), nil
}

func (f *fakeFetcher) Block(hashStr string) (*types.Block, error) {
	hash := &crypto.HashType{}
	if err := hash.SetString(hashStr); err != nil {
		return nil, err
	}
	height, _ := decodeHash(hashStr)
	prevHash := &crypto.HashType{}
	if height > 1 {
		prevHashStr, err := f.HashForHeight(height - 1)
		if err != nil {
			return nil, err
		}
		if err := prevHash.SetString(prevHashStr); err != nil {
			return nil, err
		}
	}
	return &types.Block{Hash: hash, Height: height, Header: &types.BlockHeader{
		PrevBlockHash: *prevHash,
	}}, nil
}

func hashForHeightWithObfs(h, obfs uint32) string {
	return fmt.Sprintf("%032x%032x", h, obfs)
}

func decodeHash(hash string) (uint32, uint32) {
	height, _ := strconv.ParseUint(hash[:32], 16, 32)
	obfs, _ := strconv.ParseUint(hash[32:], 16, 32)
	return uint32(height), uint32(obfs)
}

func syncMapWithEntries(heights []uint32, hashes []string) *sync.Map {
	m := &sync.Map{}
	for i := 0; i < len(heights) && i < len(hashes); i++ {
		m.Store(heights[i], hashes[i])
	}
	return m
}

func syncMapLength(p *sync.Map) uint32 {
	var l uint32
	p.Range(func(key, value interface{}) bool {
		l++
		return true
	})
	return l
}

func Test_catcherImpl_sync(t *testing.T) {
	type fields struct {
		currentHeight uint32
		fetcher       BlockFetcher
		blocks        *sync.Map
	}
	tests := []struct {
		name     string
		fields   fields
		height   uint32
		lastHash string
	}{
		{
			name: "init",
			fields: fields{
				currentHeight: 0,
				fetcher:       fakeFetcherWithHeight(1, 0, 0),
				blocks:        &sync.Map{},
			},
			height:   1,
			lastHash: hashForHeightWithObfs(1, 0),
		},
		{
			name: "catch up 1 block",
			fields: fields{
				currentHeight: 1,
				fetcher:       fakeFetcherWithHeight(2, 0, 0),
				blocks:        syncMapWithEntries([]uint32{1}, []string{hashForHeightWithObfs(1, 0)}),
			},
			height:   2,
			lastHash: hashForHeightWithObfs(2, 0),
		},
		{
			name: "reorg on block",
			fields: fields{
				currentHeight: 2,
				fetcher:       fakeFetcherWithHeight(3, 1, 2),
				blocks: syncMapWithEntries([]uint32{1, 2}, []string{
					hashForHeightWithObfs(1, 0),
					hashForHeightWithObfs(2, 0),
				}),
			},
			height:   3,
			lastHash: hashForHeightWithObfs(3, 1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ci := &catcherImpl{
				currentHeight: tt.fields.currentHeight,
				fetcher:       tt.fields.fetcher,
				blocks:        tt.fields.blocks,
				syncMutex:     &sync.Mutex{},
			}
			ci.sync()
			ensure.DeepEqual(t, ci.currentHeight, tt.height)
			ensure.DeepEqual(t, syncMapLength(ci.blocks), tt.height)
			ensure.DeepEqual(t, ci.getStoredBlockHash(tt.height), tt.lastHash)
		})
	}
}
