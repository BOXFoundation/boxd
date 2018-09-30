// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"container/heap"
)

// LessFunc define less function.
type LessFunc func(*PriorityQueue, int, int) bool

// PriorityQueue define a priority queue.
type PriorityQueue struct {
	lessFunc LessFunc
	items    []interface{}
}

// Items return item in queue at index i.
func (pq *PriorityQueue) Items(i int) interface{} {
	return pq.items[i]
}

// Len returns the length of items.  It implemented heap.Interface.
func (pq *PriorityQueue) Len() int {
	return len(pq.items)
}

// Less implemented the heap.Interface.
func (pq *PriorityQueue) Less(i, j int) bool {
	return pq.lessFunc(pq, i, j)
}

// Swap swaps the items in the priority queue.  It implemented heap.Interface.
func (pq *PriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
}

// Push pushes the passed item onto the priority queue.
func (pq *PriorityQueue) Push(x interface{}) {
	pq.items = append(pq.items, x)
}

// Pop removes the highest priority item from the priority
func (pq *PriorityQueue) Pop() interface{} {
	n := len(pq.items)
	item := pq.items[n-1]
	pq.items[n-1] = nil
	pq.items = pq.items[0 : n-1]
	return item
}

// SetLessFunc sets the compare function for the priority queue
func (pq *PriorityQueue) SetLessFunc(lessFunc LessFunc) {
	pq.lessFunc = lessFunc
	heap.Init(pq)
}

// NewPriorityQueue create a new PriorityQueue
func NewPriorityQueue(lessFunc LessFunc) *PriorityQueue {
	pq := &PriorityQueue{
		lessFunc: lessFunc,
	}
	// Initialize here for immediate use, e.g., Push/Pop
	heap.Init(pq)
	return pq
}
