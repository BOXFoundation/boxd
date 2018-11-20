// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package priorityqueue

import (
	"errors"

	"github.com/BOXFoundation/boxd/log"
	"github.com/jbenet/goprocess"
)

var (
	logger = log.NewLogger("pq")

	errBadPriority = errors.New("bad priority")
)

// PriorityQueue is a priority queue
type PriorityQueue struct {
	queues []chan interface{}
	notify chan struct{}
}

// New return a new PriorityQueue
func New(n int, l int) *PriorityQueue {
	var queues = make([]chan interface{}, n)
	for i := range queues {
		queues[i] = make(chan interface{}, l)
	}
	return &PriorityQueue{queues: queues, notify: make(chan struct{}, l)}
}

// Run is a loop popping items from the priority queues
func (pq *PriorityQueue) Run(proc goprocess.Process, f func(interface{})) {
	top := len(pq.queues) - 1
	p := top
	for {
		q := pq.queues[p]
		select {
		case x := <-q:
			f(x)
			p = top
		default:
			if p > 0 {
				p--
				continue
			}
			select {
			case <-pq.notify:
				p = top
			case <-proc.Closing():
				return
			}
		}
	}
}

// Push pushes an item to the queue specified in the priority argument
// and notify the loop
func (pq *PriorityQueue) Push(item interface{}, p int) error {
	if p < 0 || p >= len(pq.queues) {
		return errBadPriority
	}
	pq.queues[p] <- item
	pq.notify <- struct{}{}
	return nil
}
