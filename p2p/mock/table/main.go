// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package main

import (
	crand "crypto/rand"
	"flag"
	"fmt"
	"hash/fnv"
	"math/rand"
	"reflect"
	"time"

	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
)

type nodeIdx []int

// Len returns the size of nodeIdx
func (idx nodeIdx) Len() int {
	return len(idx)
}

// Swap swaps the ith with jth
func (idx nodeIdx) Swap(i, j int) {
	idx[i], idx[j] = idx[j], idx[i]
}

// Less returns true if ith <= jth else false
func (idx nodeIdx) Less(i, j int) bool {
	return idx[i] <= idx[j]
}

//
var (
	NodeCount                = flag.Int("node_count", 1000, "node count in network, default is 1000")
	NeighborCount            = flag.Int64("neighbor_count", 50, "neighbor count in route table, default is 50")
	DiscoverTimes            = flag.Int64("discover_times", 5, "times of sync table, default is 5")
	NeighborSyncMaxCount     = flag.Int64("neighbor_sync_max_count", 16, "neighbor sync max count in route table, default is 16")
	DiscoverDisconnRate      = flag.Float64("discover_disconn_rate", 0.25, "rate of disconnected node in sync table, default is 0.25")
	DiscoverNeighborMaxCount = flag.Int64("discover_neighbor_max_count", 16, "max neighbor to sync the table, default is 16")
	MaxTTL                   = flag.Int64("max_ttl", 3, "max ttl, default is 3")
	LoopTimes                = flag.Int("loop_times", 10, "number of loop times, default is 20")
)

// Node the simulation of the node
type Node struct {
	id       int
	name     string
	neighbor []int
	store    []int
	bingo    bool
	ttl      int
	counter  int
}

func gotask() (float32, float32, float32) {

	nodeCount := int(*NodeCount)
	var nodes []*Node

	nodes = initRouteTable(nodeCount, nodes)

	discoverTimes := int(*DiscoverTimes)

	for i := 0; i < discoverTimes; i++ {
		for _, v := range nodes {
			targets := chooseNodes(v)
			for _, target := range targets {
				syncRoute(v, nodes[target], nodes)
			}
		}
	}

	count := float32(0)
	for _, v := range nodes {
		count += calculate(v, nodes)
	}

	random := rand.Intn(nodeCount)
	node := nodes[random]
	node.bingo = true
	broadcast(node, nodes)

	bingoCnt := 0
	msgCnt := 0
	for _, v := range nodes {
		msgCnt += v.counter
		if v.bingo == true {
			bingoCnt++
		}
	}
	fmt.Printf("range of aggregation：%v, rate of coverage：%v, avg of node msg cnt: %v\n",
		float32(count)/float32(len(nodes)),
		float32(bingoCnt)/float32(nodeCount),
		float32(msgCnt)/float32(nodeCount))

	return float32(count) / float32(len(nodes)), float32(bingoCnt) / float32(nodeCount), float32(msgCnt) / float32(nodeCount)

}

func broadcast(node *Node, nodes []*Node) {
	maxTTL := int(*MaxTTL)
	if node.ttl <= maxTTL {
		for _, v := range node.neighbor {
			n := nodes[v]
			n.counter++
			if n.id != node.id && !n.bingo {
				n.bingo = true
				n.ttl = node.ttl + 1
				broadcast(n, nodes)
			}
		}
	}
	return
}

func calculate(node *Node, nodes []*Node) float32 {

	denominator := len(node.neighbor) * (len(node.neighbor) - 1) / 2
	neighbors := []*Node{}
	for _, id := range node.neighbor {
		neighbors = append(neighbors, nodes[id])
	}

	count := 0
	for _, neighbor := range neighbors {
		for _, id := range neighbor.neighbor {
			if inArray(id, node.neighbor) {
				count++
			}
		}
	}
	return float32(count) / 2 / float32(denominator)
}

func chooseNodes(node *Node) []int {

	discoverNeighborMaxCount := int(*DiscoverNeighborMaxCount)
	discoverDisconnRate := float32(*DiscoverDisconnRate)

	nodes := []int{}

	if len(node.neighbor) > int(float32(discoverNeighborMaxCount)*(float32(1)-discoverDisconnRate)) {
		for _, v := range shuffle(node.neighbor)[:int(float32(discoverNeighborMaxCount)*(float32(1)-discoverDisconnRate))] {
			if !inArray(v, nodes) {
				nodes = append(nodes, v)
			}
		}
	} else {
		for _, v := range shuffle(node.neighbor) {
			if !inArray(v, nodes) {
				nodes = append(nodes, v)
			}
		}
	}

	if len(node.store) <= int(float32(discoverNeighborMaxCount)*discoverDisconnRate) {
		for _, v := range shuffle(node.store) {
			if !inArray(v, nodes) {
				nodes = append(nodes, v)
			}
		}
	} else {
		for _, v := range shuffle(node.store)[:int(float32(discoverNeighborMaxCount)*discoverDisconnRate)] {
			if !inArray(v, nodes) {
				nodes = append(nodes, v)
			}
		}
	}

	return nodes
}

func initRouteTable(nodeCount int, nodes []*Node) []*Node {
	seed := newNode(0)
	nodes = append(nodes, seed)

	for i := 1; i < nodeCount; i++ {
		node := newNode(i)
		nodes = append(nodes, node)
		syncRoute(node, seed, nodes)
		for k := 1; k < i; k++ {
			node := nodes[k]
			syncRoute(node, seed, nodes)
		}
	}
	return nodes
}

func newNode(id int) *Node {
	_, pub, _ := crypto.GenerateEd25519Key(crand.Reader)
	name, _ := peer.IDFromPublicKey(pub)
	node := &Node{
		id:       id,
		name:     name.Pretty(),
		neighbor: []int{},
		store:    []int{},
		bingo:    false,
		ttl:      0,
		counter:  0,
	}
	return node
}

func syncRoute(node *Node, target *Node, nodes []*Node) {

	neighborCount := int(*NeighborCount)
	neighborSyncMaxCount := int(*NeighborSyncMaxCount)

	ret := target.neighbor
	if len(ret) > neighborSyncMaxCount {
		ret = shuffle(target.neighbor)[:neighborSyncMaxCount]
	}

	for _, id := range ret {
		if id != node.id && !inArray(id, node.store) {
			node.store = append(node.store, id)
		}
	}
	node.neighbor = connect(node, target.id, neighborCount, nodes)
	if !inArray(target.id, node.store) {
		node.store = append(node.store, target.id)
	}

	target.neighbor = connect(target, node.id, neighborCount, nodes)
	if !inArray(node.id, target.store) {
		target.store = append(target.store, node.id)
	}
	return
}

func hash(str string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(str))
	return h.Sum32()
}

func connect(node *Node, id int, limit int, nodes []*Node) []int {
	ids := node.neighbor
	disconns := []int{}
	if len(ids) >= limit {
		count := len(ids) - limit
		ids = shuffle(ids)
		disconns = ids[:count+1]
		ids = ids[count+1:]
	}
	if !inArray(id, ids) {
		ids = append(ids, id)
	}

	for _, v := range disconns {
		disconnNode := nodes[v]
		newNeighbors := []int{}
		for _, neighbor := range disconnNode.neighbor {
			if neighbor != node.id {
				newNeighbors = append(newNeighbors, neighbor)
			}
		}
		disconnNode.neighbor = newNeighbors
	}

	return ids
}

func inArray(obj interface{}, array interface{}) bool {
	arrayValue := reflect.ValueOf(array)
	if reflect.TypeOf(array).Kind() == reflect.Array || reflect.TypeOf(array).Kind() == reflect.Slice {
		for i := 0; i < arrayValue.Len(); i++ {
			if arrayValue.Index(i).Interface() == obj {
				return true
			}
		}
	}
	return false
}

func shuffle(vals []int) []int {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ret := make([]int, len(vals))
	perm := r.Perm(len(vals))
	for i, randIndex := range perm {
		ret[i] = vals[randIndex]
	}
	return ret
}

func main() {
	flag.Parse()
	fmt.Printf("Usage: [-node_count] [-neighbor_count] [-neighbor_sync_max_count] [-discover_neighbor_max_count] [-max_ttl] [-loop_times]\n")

	total := float32(0)
	coveredNodeTotal := float32(0)
	msgCntTotal := float32(0)

	for i := 0; i < *LoopTimes; i++ {
		count, coveredNodeCntRate, msgCntPerNode := gotask()
		total += count
		coveredNodeTotal += coveredNodeCntRate
		msgCntTotal += msgCntPerNode
	}

	fmt.Printf("The average rate of aggregation: %v, The average rate of coverage：%v, avg of node msg cnt %v \n",
		float32(total)/(float32(*LoopTimes)),
		float32(coveredNodeTotal)/(float32(*LoopTimes)),
		msgCntTotal/float32(*LoopTimes))

}
