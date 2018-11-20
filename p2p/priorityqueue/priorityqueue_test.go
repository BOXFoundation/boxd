// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package priorityqueue

import (
	"os"
	"testing"

	"github.com/facebookgo/ensure"
	"github.com/jbenet/goprocess"
)

func TestPriorityQueue(t *testing.T) {
	ch := make(chan int, 1)
	pq := New(2, 20)
	go pq.Run(goprocess.WithSignals(os.Interrupt), func(v interface{}) {
		ensure.DeepEqual(t, v.(string), "box")
		ch <- 1
	})
	pq.Push("box", 1)
	<-ch
}
