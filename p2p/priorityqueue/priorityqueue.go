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
	errPQFull      = errors.New("the priority queue is full")
)

// PriorityMsgQueue is a priority message queue
type PriorityMsgQueue struct {
	queues []chan interface{}
	notify chan struct{}
}

// New return a new PriorityMsgQueue
func New(n int, l int) *PriorityMsgQueue {
	var queues = make([]chan interface{}, n)
	for i := range queues {
		queues[i] = make(chan interface{}, l)
	}
	return &PriorityMsgQueue{queues: queues, notify: make(chan struct{}, 1)}
}

// Run is a loop popping items from the priority message queues
func (pq *PriorityMsgQueue) Run(proc goprocess.Process, f func(interface{})) {
	var q chan interface{}
	var x interface{}
	top := len(pq.queues) - 1
	p := top
	defer logger.Info("Quit PriorityMsgQueue loop")
	for {
		q = pq.queues[p]
		select {
		case x = <-q:
			f(x)
			p = top
		case <-proc.Closing():
			return
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
func (pq *PriorityMsgQueue) Push(item interface{}, p int) error {
	if p < 0 || p >= len(pq.queues) {
		return errBadPriority
	}
	select {
	case pq.queues[p] <- item:
	default:
		logger.Debugf("pq size: %v, %v, %v, %v", len(pq.queues[0]), len(pq.queues[1]), len(pq.queues[2]), len(pq.queues[3]))
		return errPQFull
	}
	select {
	case pq.notify <- struct{}{}:
	default:
	}
	return nil
}

// Close close the queue
func (pq *PriorityMsgQueue) Close() {
	for i := 0; i < len(pq.queues); i++ {
		pq.queues[i] = nil
	}
	pq.notify = nil
}
