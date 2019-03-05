// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wallet

import (
	"sync"
	"time"

	"github.com/BOXFoundation/boxd/core/types"
)

const (
	defaultUtxoLiveDuration = 10
)

// LiveUtxoCache defines a cache in that utxos keep alive
type LiveUtxoCache struct {
	sync.RWMutex
	oldest       int64
	liveDuration int64
	ts2Op        map[int64][]*types.OutPoint
	Op2ts        map[types.OutPoint]int64
}

// NewLiveUtxoCache new a LiveUtxoCache instance with expired seconds
func NewLiveUtxoCache() *LiveUtxoCache {
	return &LiveUtxoCache{
		liveDuration: defaultUtxoLiveDuration,
		ts2Op:        make(map[int64][]*types.OutPoint),
		Op2ts:        make(map[types.OutPoint]int64),
		oldest:       time.Now().Unix(),
	}
}

// SetLiveDuration sets live duration
func (cache *LiveUtxoCache) SetLiveDuration(expired int) {
	if expired <= 0 {
		expired = defaultUtxoLiveDuration
	}
	cache.liveDuration = int64(expired)
}

// Add adds a OutPoint to cache
func (cache *LiveUtxoCache) Add(op *types.OutPoint) {
	if op == nil {
		return
	}
	cache.Lock()
	// check whether op already exists, if exists, remove it
	ts, ok := cache.Op2ts[*op]
	if ok {
		ops := cache.ts2Op[ts]
		for i, o := range ops {
			if *o == *op {
				cache.ts2Op[ts] = append(ops[:i], ops[i+1:]...)
				break
			}
		}
	}
	// update op in cache
	now := time.Now().Unix()
	cache.ts2Op[now] = append(cache.ts2Op[now], op)
	cache.Op2ts[*op] = now
	cache.Unlock()
}

// Del deletes a OutPoint in cache
func (cache *LiveUtxoCache) Del(op ...*types.OutPoint) {
	if op == nil {
		return
	}
	cache.Lock()
	for _, o := range op {
		delete(cache.Op2ts, *o)
	}
	cache.Unlock()
}

// Shrink shrinks phase out OutPoints
func (cache *LiveUtxoCache) Shrink() {
	now := time.Now().Unix()
	if now-cache.oldest < cache.liveDuration {
		return
	}
	cache.Lock()
	for t := cache.oldest; t < now-cache.liveDuration; t++ {
		// get OutPoints
		ops, ok := cache.ts2Op[t]
		if !ok {
			continue
		}
		// remove OutPoint from Op2ts
		for _, o := range ops {
			delete(cache.Op2ts, *o)
		}
		// remove timestamp from ts2Op
		delete(cache.ts2Op, t)
	}
	cache.oldest = now - cache.liveDuration
	cache.Unlock()
}

// Contains return whether OutPoint is in cache
func (cache *LiveUtxoCache) Contains(op *types.OutPoint) bool {
	if op == nil {
		return false
	}
	cache.RLock()
	_, ok := cache.Op2ts[*op]
	cache.RUnlock()
	return ok
}

// Count returns OutPoint count
func (cache *LiveUtxoCache) Count() int {
	return len(cache.Op2ts)
}
