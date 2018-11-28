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

var (
	NodeCount                = flag.Int("node_count", 1000, "node count in network, default is 1000")
	NeighborCount            = flag.Int64("neighbor_count", 50, "neighbor count in route table, default is 50")
	NeighborSyncMaxCount     = flag.Int64("neighbor_sync_max_count", 16, "neighbor sync max count in route table, default is 16")
	DiscoverNeighborMaxCount = flag.Int64("discover_neighbor_max_count", 16, "max neighbor to sync the table, default is 16")
	DiscoverTimes            = flag.Int64("discover_times", 8, "times of sync table, default is 5")
	LoopTimes                = flag.Int("loop_times", 20, "number of loop times, default is 20")
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

func gotask() float32 {

	nodeCount := int(*NodeCount)
	var nodes []*Node
	// 初始化节点
	nodes = initRouteTable(nodeCount, nodes)

	discoverTimes := int(*DiscoverTimes)

	for i := 0; i < discoverTimes; i++ {
		for _, v := range nodes {
			// 选取需要来同步路由的节点
			targets := chooseNodes(v)
			for _, target := range targets {
				syncRoute(v, nodes[target], nodes)
			}
		}
	}

	// 获取聚合系数
	count := float32(0)
	for _, v := range nodes {
		count += calculate(v, nodes)
	}
	// TODO: 取中位数
	fmt.Printf("range of aggregation：%v\n", float32(count)/float32(len(nodes)))
	return count
}

// 计算聚合系数
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

	nodes := []int{}

	if len(node.neighbor) > discoverNeighborMaxCount*3/4 {
		for _, v := range shuffle(node.neighbor)[:discoverNeighborMaxCount*3/4] {
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

	if len(node.store) <= discoverNeighborMaxCount/4 {
		for _, v := range shuffle(node.store) {
			if !inArray(v, nodes) {
				nodes = append(nodes, v)
			}
		}
	} else {
		for _, v := range shuffle(node.store)[:discoverNeighborMaxCount/4] {
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

	// 初始化nodeCount个节点，并从seed同步路由
	for i := 1; i < nodeCount; i++ {
		node := newNode(i)
		nodes = append(nodes, node)
		syncRoute(node, seed, nodes)
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

// node 来同步 target 的 neighbor
func syncRoute(node *Node, target *Node, nodes []*Node) {

	neighborCount := int(*NeighborCount)
	neighborSyncMaxCount := int(*NeighborSyncMaxCount)

	ret := target.neighbor
	if len(ret) > neighborSyncMaxCount {
		ret = shuffle(target.neighbor)[:neighborSyncMaxCount]
	}

	// 如果target的neighbor数小于 default cnt，则全都同步到node
	for id := range ret {
		// 存起来
		if id != node.id && !inArray(id, node.store) {
			node.store = append(node.store, id)
		}
	}
	// 把target自身id也放进来
	node.neighbor = connect(node.neighbor, target.id, neighborCount)
	if !inArray(target.id, node.store) {
		node.store = append(node.store, target.id)
	}

	// 把node.id也放到target来
	target.neighbor = connect(target.neighbor, node.id, neighborCount)
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

// 如果ids的长度 >= limit 则随机取limit-1个，然后把id加进去
func connect(ids []int, id int, limit int) []int {
	if len(ids) >= limit {
		count := len(ids) - limit
		ids = shuffle(ids)
		ids = ids[count+1:]
	}
	if !inArray(id, ids) {
		ids = append(ids, id)
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

// 打乱数组
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
	total := float32(0)
	fmt.Printf("Usage: [-node_count] [-neighbor_count] [-neighbor_sync_max_count] [-discover_neighbor_max_count] [-max_ttl] [-loop_times]\n")

	for i := 0; i < *LoopTimes; i++ {
		count := gotask()
		total += count
	}

	fmt.Println("The average rate of coverage：", float32(total)/(float32(*LoopTimes*(*NodeCount))))

}
