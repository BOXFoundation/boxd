package main

import (
	crand "crypto/rand"
	"flag"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"reflect"
	"sort"
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
	NodeCount     = flag.Int("node_count", 200, "node count in network, default is 1000")
	NeighborCount = flag.Int64("neighbor_count", 20, "neighbor count in route table, default is 50")
	MaxTTL        = flag.Int64("max_ttl", 3, "max ttl, default is 3")
	LoopTimes     = flag.Int("loop_times", 20, "number of loop times, default is 20")
)

// Node the simulation of the node
type Node struct {
	id       int
	name     string
	neighbor []int
	bingo    bool
	ttl      int
	counter  int
}

func gotask() (int, float32) {

	nodeCount := int(*NodeCount)
	var nodes []*Node
	// 初始化节点
	nodes = initRouteTable(nodeCount, nodes)

	// 随机取个节点让这个节点广播消息
	random := rand.Intn(nodeCount)
	node := nodes[random]
	node.bingo = true
	broadcast(node, nodes)

	// 获取消息覆盖率
	bingoCnt := 0
	msgCnt := 0
	for _, v := range nodes {
		msgCnt += v.counter
		if v.bingo == true {
			bingoCnt++
		}
	}
	fmt.Printf("rate of coverage：%v, avg of node msg cnt %v\n", float32(bingoCnt)/float32(nodeCount), float32(msgCnt)/float32(nodeCount))
	return bingoCnt, float32(msgCnt) / float32(nodeCount)
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

	// 疯狂打乱路由表
	for k := 0; k < 10; k++ {
		// 遍历所有节点
		for i := 0; i < nodeCount; i++ {
			node := nodes[i]
			// 取随机数slice[len(node.neighbor) - 1]
			rand.Seed(time.Now().UnixNano())
			randomList := rand.Perm(len(node.neighbor) - 1)

			// 随便取几个其他节点同步路由
			for i := 0; i < int(math.Sqrt(float64(len(node.neighbor)))); i++ {
				id := node.neighbor[randomList[i]]
				tar := nodes[id]
				syncRoute(node, tar, nodes)
			}

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
		bingo:    false,
		ttl:      0,
		counter:  0,
	}

	return node
}

func broadcast(node *Node, nodes []*Node) {
	maxTTL := int(*MaxTTL)
	if node.ttl <= maxTTL {
		for _, v := range node.neighbor {
			n := nodes[v]
			n.counter++
			// 弃掉重复消息
			if n.id != node.id && !n.bingo {
				n.bingo = true
				n.ttl = node.ttl + 1
				broadcast(n, nodes)
			}
		}
	}
	return
}

// node 来同步 target 的 neighbor
func syncRoute(node *Node, target *Node, nodes []*Node) {

	neighborCount := int(*NeighborCount)
	// 如果target的neighbor数小于 default cnt，则全都同步到node
	if len(target.neighbor) < neighborCount {
		// TODO: 这个需要改 不应该是全同步过来
		for id := range target.neighbor {
			node.neighbor = addNewNode(node.neighbor, id, neighborCount)
		}
		// 把target自身id也放进来
		node.neighbor = addNewNode(node.neighbor, target.id, neighborCount)

		// 把node的id放到所有存在的节点的neighbor里
		for id := range target.neighbor {
			n := nodes[id]
			n.neighbor = addNewNode(n.neighbor, node.id, neighborCount)
		}
		target.neighbor = addNewNode(target.neighbor, node.id, neighborCount)
		return
	}

	// 如果target的neighbor数很多，则取相对短的那一半target.neighbor
	// target.neighbor = shuffle(target.neighbor)
	ret := getNearestNode(node, target.neighbor, nodes)
	for _, retID := range ret {
		node.neighbor = addNewNode(node.neighbor, int(retID), neighborCount)
		tempnode := nodes[int(retID)]
		tempnode.neighbor = addNewNode(tempnode.neighbor, node.id, neighborCount)
	}

	target.neighbor = addNewNode(target.neighbor, node.id, neighborCount)
	return
}

// 取距离相对短的那一半ids
func getNearestNode(node *Node, ids []int, nodes []*Node) nodeIdx {
	var ret nodeIdx
	var tmp nodeIdx
	var tmpMap = make(map[int]int)
	// fmt.Println("nearest id:", len(ids))
	for _, id := range ids {
		tempnode := nodes[id]
		nodeNameInt := int(hash(node.name))
		tempnodeNameInt := int(hash(tempnode.name))
		distance := nodeNameInt ^ tempnodeNameInt
		// distance := node.id ^ id
		tmp = append(tmp, distance)
		sort.Sort(tmp)
		// 把距离和id映射保存起来
		tmpMap[distance] = id
		// 如果满足条件 就在slice和map删掉最长距离记录
		if len(tmp) > len(ids)/2 {
			delete(tmpMap, tmp[len(tmp)-1])
			tmp = tmp[:len(tmp)-1]
		}
	}
	for _, v := range tmpMap {
		ret = append(ret, v)
	}

	return ret
}

func hash(str string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(str))
	return h.Sum32()
}

// 如果ids的长度 >= limit 则随机取limit-1个，然后把id加进去
func addNewNode(ids []int, id int, limit int) []int {
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

func generateRandomNumber(start int, end int, count int) []int {
	if end < start || (end-start) < count {
		return nil
	}

	nums := make([]int, 0)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for len(nums) < count {
		num := r.Intn((end - start)) + start

		exist := false
		for _, v := range nums {
			if v == num {
				exist = true
				break
			}
		}

		if !exist {
			nums = append(nums, num)
		}
	}

	return nums
}

func main() {
	flag.Parse()
	fmt.Printf("Usage: [-node_count] [-neighbor_count] [-max_ttl] [-loop_times]\n")

	

	total := 0
	msgCntTotal := float32(0)

	for i := 0; i < *LoopTimes; i++ {
		count, msgCntPerNode := gotask()
		total += count
		msgCntTotal += msgCntPerNode
	}

	fmt.Printf("The average rate of coverage：%v, avg of node msg cnt %v \n", float32(total)/(float32(*LoopTimes*(*NodeCount))), msgCntTotal/float32(*LoopTimes))

}
