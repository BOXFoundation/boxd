// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"container/heap"
	"math/rand"
	"testing"
	"time"
)

const n = 1000

func TestNewPriorityQueue(t *testing.T) {
	lessfuncs := LessFunc(func(pq *PriorityQueue, i int, j int) bool {
		return pq.items[i].(int) < pq.items[j].(int)
	})
	pq := NewPriorityQueue(lessfuncs)
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < n; i++ {
		r := randomInt(1, n)
		heap.Push(pq, r)
	}
	previous := -1
	count := 0
	for pq.Len() > 0 {
		count++
		item := heap.Pop(pq).(int)
		if previous > item {
			t.Errorf("PriorityQueue item %v is not in correct order.", item)
		}
		previous = item
	}
	if count != n {
		t.Errorf("PriorityQueue item number %v is not correct.", count)
	}
}

func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}
