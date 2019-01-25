// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wallet

import (
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
)

func TestLiveUtxoCache(t *testing.T) {
	cache := NewLiveUtxoCache(3)

	n := 4
	nonExists := make([]*types.OutPoint, 0, n)
	for i := 0; i < n; i++ {
		hash := hashFromUint64(uint64(i))
		op := types.NewOutPoint(&hash, uint32(i))
		cache.Add(op)
		nonExists = append(nonExists, op)
	}
	if cache.Count() != n {
		t.Fatalf("cache count want: %d, got: %d", n, cache.Count())
	}

	// after 2 test
	time.Sleep(2 * time.Second)
	cache.Shrink()
	if cache.Count() != n {
		t.Fatalf("cache count want: %d, got: %d", n, cache.Count())
	}
	//
	m := 6
	exists := make([]*types.OutPoint, 0, m)
	for i := n - 1; i < n+m; i++ {
		hash := hashFromUint64(uint64(i))
		op := types.NewOutPoint(&hash, uint32(i))
		cache.Add(op)
		exists = append(exists, op)
	}
	// have renew outpoint from n-1
	nonExists = nonExists[:n-1]
	cache.Shrink()
	if cache.Count() != n+m {
		t.Fatalf("cache count want: %d, got: %d", n+m, cache.Count())
	}

	// after 4s test
	time.Sleep(2 * time.Second)
	cache.Shrink()
	if cache.Count() != m+1 {
		t.Fatalf("cache count want: %d, got: %d", m, cache.Count())
	}
	// verify exists
	for _, op := range exists {
		if !cache.Contains(op) {
			t.Fatalf("%+v not exists", op)
		}
	}
	// verify non-exists
	for _, op := range nonExists {
		if cache.Contains(op) {
			t.Fatalf("%+v exists", op)
		}
	}

	// after 6s test
	time.Sleep(2 * time.Second)
	cache.Shrink()
	if cache.Count() != 0 {
		t.Fatalf("cache count want: %d, got: %d", 0, cache.Count())
	}
}
