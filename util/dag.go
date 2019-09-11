// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"container/heap"
)

// Node define a node struct
type Node struct {
	key      interface{}
	index    int
	children []*Node
	pNum     int
	weight   int
}

// NewNode new node
func NewNode(key interface{}, index int, weight int) *Node {
	return &Node{
		key:      key,
		index:    index,
		weight:   weight,
		pNum:     0,
		children: make([]*Node, 0),
	}
}

// Key return node key
func (node *Node) Key() interface{} {
	return node.key
}

// Dag define a dag struct
type Dag struct {
	nodes map[interface{}]*Node
	// index  int
	indexs map[int]interface{}
}

// NewDag new dag
func NewDag() *Dag {
	return &Dag{
		nodes:  make(map[interface{}]*Node, 0),
		indexs: make(map[int]interface{}, 0),
	}
}

// Index return current index in dag
func (dag *Dag) Index() int {
	return len(dag.nodes)
}

// AddNode add node for dag
func (dag *Dag) AddNode(key interface{}, weight int) error {
	if _, ok := dag.nodes[key]; ok {
		return ErrKeyIsExisted
	}
	index := len(dag.nodes)
	dag.nodes[key] = NewNode(key, index, weight)
	dag.indexs[index] = key
	return nil
}

// TopoSort topo_sort in dag
func (dag *Dag) TopoSort() []*Node {

	var sortedNode []*Node
	queue := NewPriorityQueue(func(queue *PriorityQueue, i, j int) bool {
		nodei := queue.Items(i).(*Node)
		nodej := queue.Items(j).(*Node)
		return nodei.weight < nodej.weight
	})
	rootNodes := dag.GetRootNodes()
	return sortNodes(rootNodes, queue, sortedNode)
}

func sortNodes(rootNodes []*Node, queue *PriorityQueue, sortedNodes []*Node) []*Node {

	var tempNodes []*Node
	for _, node := range rootNodes {
		if node.pNum == 0 {
			heap.Push(queue, node)
			for _, nchild := range node.children {
				nchild.pNum--
				if nchild.pNum == 0 {
					tempNodes = append(tempNodes, nchild)
				}
			}
		}
	}
	for queue.Len() > 0 {
		n := heap.Pop(queue).(*Node)
		sortedNodes = append(sortedNodes, n)
	}
	if len(tempNodes) > 0 {
		sortedNodes = sortNodes(tempNodes, queue, sortedNodes)
	}
	return sortedNodes
}

// GetRootNodes get root nodes in dag
func (dag *Dag) GetRootNodes() []*Node {
	nodes := make([]*Node, 0)
	for _, node := range dag.nodes {
		if node.pNum == 0 {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// AddEdge add edge for dag
func (dag *Dag) AddEdge(fkey, tkey interface{}) error {
	var from, to *Node
	var ok bool

	if from, ok = dag.nodes[fkey]; !ok {
		return ErrKeyNotFound
	}

	if to, ok = dag.nodes[tkey]; !ok {
		return ErrKeyNotFound
	}

	for _, childNode := range from.children {
		if childNode == to {
			return ErrKeyIsExisted
		}
	}

	dag.nodes[tkey].pNum++
	dag.nodes[fkey].children = append(from.children, to)
	return nil
}

//IsCirclular  check if circlular in dag
func (dag *Dag) IsCirclular() bool {

	visited := make(map[interface{}]int, len(dag.nodes))
	rootNodes := make(map[interface{}]*Node)
	for key, node := range dag.nodes {
		visited[key] = 0
		rootNodes[key] = node
	}

	for _, node := range rootNodes {
		if dag.hasCirclularDep(node, visited) {
			return true
		}
	}

	for _, count := range visited {
		if count == 0 {
			return true
		}
	}
	return false
}

func (dag *Dag) hasCirclularDep(current *Node, visited map[interface{}]int) bool {

	visited[current.key] = 1
	for _, child := range current.children {
		if visited[child.key] == 1 {
			return true
		}

		if dag.hasCirclularDep(child, visited) {
			return true
		}
	}
	visited[current.key] = 2
	return false
}
