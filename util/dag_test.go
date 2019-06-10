// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"testing"

	"github.com/facebookgo/ensure"
)

func TestTopoSort(t *testing.T) {
	dag := NewDag()
	dag.AddNode(1, 1)
	dag.AddNode(2, 2)
	dag.AddNode(3, 3)
	dag.AddNode(4, 4)
	dag.AddNode(5, 5)
	dag.AddNode(6, 6)
	dag.AddNode(7, 7)
	dag.AddNode(8, 8)
	dag.AddNode(9, 9)
	dag.AddNode(10, 10)

	dag.AddEdge(1, 2)
	dag.AddEdge(1, 4)
	dag.AddEdge(2, 3)
	dag.AddEdge(4, 5)
	dag.AddEdge(4, 6)
	dag.AddEdge(5, 7)
	dag.AddEdge(6, 8)
	dag.AddEdge(7, 9)
	dag.AddEdge(9, 10)

	node := dag.TopoSort()

	var keys []int
	for _, v := range node {
		keys = append(keys, v.key.(int))
	}
	ensure.DeepEqual(t, dag.IsCirclular(), false)
	ensure.DeepEqual(t, keys, []int{1, 4, 2, 6, 5, 3, 8, 7, 9, 10})
}

func TestIsCirclular(t *testing.T) {

	dag := NewDag()
	dag.AddNode(1, 1)
	dag.AddNode(2, 2)
	dag.AddNode(3, 3)
	dag.AddNode(4, 4)
	dag.AddNode(5, 6)

	dag.AddEdge(1, 2)
	dag.AddEdge(2, 3)
	dag.AddEdge(3, 4)
	dag.AddEdge(4, 5)
	dag.AddEdge(5, 2)

	ensure.DeepEqual(t, dag.IsCirclular(), true)
}
