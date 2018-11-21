// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"crypto/rand"
	"fmt"
	rand2 "math/rand"
	"strconv"
	"sync"
	"testing"

	"github.com/BOXFoundation/boxd/storage/memdb"

	"github.com/facebookgo/ensure"

	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util/bloom"
)

func TestNewFilterHolder(t *testing.T) {
	filter := NewFilterHolder()
	ensure.NotNil(t, filter)
	internalFilter, ok := filter.(*MemoryBloomFilterHolder)
	ensure.True(t, ok)
	ensure.DeepEqual(t, len(internalFilter.entries), 0)
	ensure.NotNil(t, internalFilter.entries)
	ensure.NotNil(t, internalFilter.mux)
}

func hashForHeight(height uint32) (hash crypto.HashType) {
	bts := crypto.Sha256([]byte(strconv.FormatUint(uint64(height), 10)))
	hash.SetBytes(bts)
	return
}

func wordWithInt(in uint64) []byte {
	return []byte(strconv.FormatUint(in, 10))
}

func wordWithRand(len uint32) ([]byte, error) {
	target := make([]byte, len)
	_, err := rand.Read(target)
	if err != nil {
		return nil, err
	}
	return target, nil
}

func filterForHeight(height uint32) (filter bloom.Filter) {
	filter = bloom.NewFilter(height, 0.0001)
	for i := 0; i <= int(height); i++ {
		filter.Add(wordWithInt(uint64(i)))
	}
	return
}

func heavyFilter(t *testing.T, preset [][]byte, totalCaps int) bloom.Filter {
	filter := bloom.NewFilter(uint32(totalCaps), 0.0001)
	for _, word := range preset {
		filter.Add(word)
	}
	for i := 0; i < (totalCaps - len(preset)); i++ {
		word, err := wordWithRand(uint32(rand2.Intn(2048)))
		ensure.Nil(t, err)
		filter.Add(word)
	}
	return filter
}

func prepareEntries(length int) (entries []*FilterEntry) {
	for i := 0; i < length; i++ {
		height := uint32(i + 1)
		entry := &FilterEntry{
			Filter:    filterForHeight(height),
			Height:    height,
			BlockHash: hashForHeight(height),
		}
		entries = append(entries, entry)
	}
	return
}

func prepareHeavyEntries(t *testing.T, length int, preset map[uint32][][]byte, minCap, maxCap int) (entries []*FilterEntry) {
	for i := 0; i < length; i++ {
		height := uint32(i + 1)
		presetWords, ok := preset[height]
		if !ok {
			presetWords = [][]byte{}
		}
		caps := rand2.Intn(maxCap-minCap+1) + minCap
		filter := heavyFilter(t, presetWords, caps)
		entry := &FilterEntry{
			Filter:    filter,
			Height:    height,
			BlockHash: hashForHeight(height),
		}
		entries = append(entries, entry)
	}
	return
}

func prepareFilterDb(t *testing.T, entries []*FilterEntry) storage.Table {
	db, err := memdb.NewMemoryDB("filter holder test", nil)
	ensure.NotNil(t, db)
	ensure.Nil(t, err)
	table, err := db.Table("filter")
	ensure.NotNil(t, table)
	ensure.Nil(t, err)
	for _, entry := range entries {
		filterMarshalled, err := entry.Filter.Marshal()
		ensure.Nil(t, err)
		table.Put(FilterKey(entry.BlockHash), filterMarshalled)
	}
	return table
}

// uFilter is an bloom.Filter implmentation with marshall errors
type uFilter struct {
}

func (*uFilter) Matches(data []byte) bool { return true }

func (*uFilter) Add(data []byte) { return }

func (*uFilter) MatchesAndAdd(data []byte) bool { return true }

func (*uFilter) Merge(f bloom.Filter) error { return nil }

func (*uFilter) Size() uint32 { return 0 }

func (*uFilter) K() uint32 { return 0 }

func (*uFilter) Tweak() uint32 { return 0 }

func (*uFilter) FPRate() float64 { return 0.1 }

func (*uFilter) Marshal() (data []byte, err error) { return nil, fmt.Errorf("test error") }

func (*uFilter) Unmarshal(data []byte) error { return nil }

func TestMemoryBloomFilterHolder_AddFilter(t *testing.T) {
	type args struct {
		height      uint32
		hash        crypto.HashType
		db          storage.Table
		onCacheMiss func() bloom.Filter
	}
	tests := []struct {
		name    string
		entries []*FilterEntry
		args    args
		wantErr bool
	}{
		{
			name:    "init on cache miss",
			entries: []*FilterEntry{},
			args: args{
				height: 1,
				hash:   hashForHeight(1),
				db:     prepareFilterDb(t, []*FilterEntry{}),
				onCacheMiss: func() bloom.Filter {
					return filterForHeight(1)
				},
			},
			wantErr: false,
		},
		{
			name:    "unmarshalable filter",
			entries: []*FilterEntry{},
			args: args{
				height: 1,
				hash:   hashForHeight(1),
				db:     prepareFilterDb(t, []*FilterEntry{}),
				onCacheMiss: func() bloom.Filter {
					return &uFilter{}
				},
			},
			wantErr: true,
		},
		{
			name:    "nil on cache miss",
			entries: prepareEntries(0),
			args: args{
				height:      1,
				hash:        hashForHeight(1),
				db:          prepareFilterDb(t, prepareEntries(0)),
				onCacheMiss: nil,
			},
			wantErr: true,
		},
		{
			name:    "filter exist",
			entries: prepareEntries(1),
			args: args{
				height:      1,
				hash:        hashForHeight(1),
				db:          nil,
				onCacheMiss: nil,
			},
			wantErr: false,
		},
		{
			name:    "filter replace",
			entries: prepareEntries(1),
			args: args{
				height: 1,
				hash:   hashForHeight(2),
				db:     nil,
				onCacheMiss: func() bloom.Filter {
					return filterForHeight(2)
				},
			},
			wantErr: true,
		},
		{
			name:    "invalid height",
			entries: prepareEntries(1),
			args: args{
				height:      3,
				hash:        hashForHeight(3),
				db:          nil,
				onCacheMiss: nil,
			},
			wantErr: true,
		},
		{
			name:    "add from db",
			entries: prepareEntries(1),
			args: args{
				height:      2,
				hash:        hashForHeight(2),
				db:          prepareFilterDb(t, prepareEntries(2)),
				onCacheMiss: nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			holder := &MemoryBloomFilterHolder{
				entries: tt.entries,
				mux:     &sync.Mutex{},
			}
			if err := holder.AddFilter(tt.args.height, tt.args.hash, tt.args.db, tt.args.onCacheMiss); (err != nil) != tt.wantErr {
				t.Errorf("MemoryBloomFilterHolder.AddFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryBloomFilterHolder_filterExists(t *testing.T) {
	type args struct {
		height uint32
		hash   crypto.HashType
	}
	tests := []struct {
		name    string
		entries []*FilterEntry
		args    args
		want    bool
	}{
		{
			name:    "out of bound",
			entries: prepareEntries(1),
			args: args{
				height: 2,
				hash:   hashForHeight(2),
			},
			want: false,
		},
		{
			name:    "exist",
			entries: prepareEntries(3),
			args: args{
				height: 3,
				hash:   hashForHeight(3),
			},
			want: true,
		},
		{
			name:    "height hash mismatch",
			entries: prepareEntries(3),
			args: args{
				height: 2,
				hash:   hashForHeight(3),
			},
			want: false,
		},
		{
			name:    "hash doesn't exist",
			entries: prepareEntries(3),
			args: args{
				height: 2,
				hash:   hashForHeight(4),
			},
			want: false,
		},
		{
			name:    "height doesn't match",
			entries: prepareEntries(2),
			args: args{
				height: 4,
				hash:   hashForHeight(2),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			holder := &MemoryBloomFilterHolder{
				entries: tt.entries,
				mux:     &sync.Mutex{},
			}
			if got := holder.filterExists(tt.args.height, tt.args.hash); got != tt.want {
				t.Errorf("MemoryBloomFilterHolder.filterExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryBloomFilterHolder_addFilterInternal(t *testing.T) {

	type args struct {
		filter bloom.Filter
		height uint32
		hash   crypto.HashType
	}
	tests := []struct {
		name    string
		entries []*FilterEntry
		args    args
		wantErr bool
		wantLen int
	}{
		{
			name:    "Normal",
			entries: prepareEntries(0),
			args: args{
				filter: filterForHeight(1),
				height: 1,
				hash:   crypto.HashType{},
			},
			wantErr: false,
			wantLen: 1,
		},
		{
			name:    "Normal 1",
			entries: prepareEntries(1),
			args: args{
				filter: filterForHeight(2),
				height: 2,
				hash:   hashForHeight(2),
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name:    "Empty filter",
			entries: prepareEntries(1),
			args: args{
				filter: (bloom.Filter)(nil),
				height: 2,
				hash:   hashForHeight(2),
			},
			wantErr: true,
			wantLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			holder := &MemoryBloomFilterHolder{
				entries: tt.entries,
				mux:     &sync.Mutex{},
			}
			if err := holder.addFilterInternal(tt.args.filter, tt.args.height, tt.args.hash); (err != nil) != tt.wantErr {
				t.Errorf("MemoryBloomFilterHolder.addFilterInternal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(holder.entries) != tt.wantLen {
				t.Errorf("MemoryBloomFilterHolder.addFilterInternal() lenth = %v, want %v", len(holder.entries), tt.wantLen)
			}
		})
	}
}

func TestMemoryBloomFilterHolder_ResetFilters(t *testing.T) {
	type args struct {
		height uint32
	}
	tests := []struct {
		name    string
		entries []*FilterEntry
		args    args
		lenWant int
		wantErr bool
	}{
		{
			name:    "Reset 1",
			entries: prepareEntries(10),
			args:    args{9},
			lenWant: 9,
			wantErr: false,
		},
		{
			name:    "Reset half",
			entries: prepareEntries(10),
			args:    args{5},
			lenWant: 5,
			wantErr: false,
		},
		{
			name:    "Over reset",
			entries: prepareEntries(1),
			args:    args{2},
			lenWant: 1,
			wantErr: true,
		},
		{
			name:    "Reset none",
			entries: prepareEntries(2),
			args:    args{2},
			lenWant: 2,
			wantErr: false,
		},
		{
			name:    "Reset all",
			entries: prepareEntries(10),
			args:    args{0},
			lenWant: 0,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			holder := &MemoryBloomFilterHolder{
				entries: tt.entries,
				mux:     &sync.Mutex{},
			}
			if err := holder.ResetFilters(tt.args.height); (err != nil) != tt.wantErr {
				t.Errorf("MemoryBloomFilterHolder.ResetFilters() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryBloomFilterHolder_ListMatchedBlockHashes(t *testing.T) {
	type args struct {
		word []byte
	}
	word1 := []byte("Word 1")
	word2 := []byte("This is a long word for tests")
	word3 := []byte("")
	heavyFilters := prepareHeavyEntries(t, 200, map[uint32][][]byte{
		1:   {word3},
		10:  {word1},
		101: {word1, word2},
		152: {word1, word2, word3},
		200: {word3},
	}, 200, 300)
	tests := []struct {
		name    string
		entries []*FilterEntry
		args    args
		want    []crypto.HashType
	}{
		{
			name:    "Empty entry",
			entries: prepareEntries(0),
			args:    args{wordWithInt(10)},
			want:    nil,
		},
		{
			name:    "light contains",
			entries: prepareEntries(5),
			args:    args{wordWithInt(5)},
			want:    []crypto.HashType{hashForHeight(5)},
		},
		{
			name:    "light long contains",
			entries: prepareEntries(200),
			args:    args{wordWithInt(198)},
			want:    []crypto.HashType{hashForHeight(198), hashForHeight(199), hashForHeight(200)},
		},
		{
			name:    "heavy filter 1",
			entries: heavyFilters,
			args:    args{word1},
			want:    []crypto.HashType{hashForHeight(10), hashForHeight(101), hashForHeight(152)},
		},
		{
			name:    "heavy filter 2",
			entries: heavyFilters,
			args:    args{word2},
			want:    []crypto.HashType{hashForHeight(101), hashForHeight(152)},
		},
		{
			name:    "heavy filter 3",
			entries: heavyFilters,
			args:    args{word3},
			want:    []crypto.HashType{hashForHeight(1), hashForHeight(152), hashForHeight(200)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			holder := &MemoryBloomFilterHolder{
				entries: tt.entries,
				mux:     &sync.Mutex{},
			}
			got := holder.ListMatchedBlockHashes(tt.args.word)
			if tt.want == nil {
				if len(got) != 0 {
					t.Errorf("Empty result wanted")
				}
				return
			}
			for idx, w := range tt.want {
				found := false
				for _, a := range got {
					if a.IsEqual(&w) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("hash not found for expected No.%v", idx)
				}
			}
		})
	}
}
