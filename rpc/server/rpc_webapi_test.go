// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"container/list"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/jbenet/goprocess"
	"google.golang.org/grpc"
)

const (
	blockCnt = 10
)

var (
	rpcAddr net.Addr
	starter sync.Once

	testWebAPIBus = eventbus.Default()
)

func init() {
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
}

type webapiServerTest struct {
	webapiServer
}

func setupWebAPIMockSvr() {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	origServer := NewServer(goprocess.WithSignals(os.Interrupt), nil, nil, nil, nil,
		testWebAPIBus)
	origServer.server = grpc.NewServer()
	registerWebapi(origServer)

	go origServer.server.Serve(lis)
	rpcAddr = lis.Addr()
	time.Sleep(200 * time.Millisecond)
}

func sendWebAPITestBlocks() {
	blocks := newTestBlock(blockCnt)
	for _, b := range blocks {
		testWebAPIBus.Publish(eventbus.TopicRPCSendNewBlock, b)
		time.Sleep(10 * time.Millisecond)
	}
	// stop current test case
	time.Sleep(2 * time.Second)
	proc, _ := os.FindProcess(os.Getpid())
	proc.Signal(os.Interrupt)
}

func NoTestListenAndReadNewBlock(t *testing.T) {
	// start grpc server
	starter.Do(setupWebAPIMockSvr)
	// start send blocks goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		sendWebAPITestBlocks()
	}()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// create grpc stub
			conn, err := grpc.Dial(rpcAddr.String(), grpc.WithInsecure())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			client := rpcpb.NewWebApiClient(conn)

			blockReq := &rpcpb.ListenBlockRequest{}
			stream, err := client.ListenAndReadNewBlock(context.Background(), blockReq)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < blockCnt+1; i++ {
				block, err := stream.Recv()
				if err == io.EOF {
					t.Fatalf("%v.ListenAndReadNewBlock(_) = _, %v", client, err)
					break
				}
				if err != nil {
					t.Fatalf("%v.ListenAndReadNewBlock(_) = _, %v", client, err)
				}
				if uint32(i) != block.Height {
					t.Fatalf("block height mismatch, want: %d, got: %d", i, block.Height)
				}

				bytes, err := json.MarshalIndent(block, "", "  ")
				t.Logf("[%d] recv block: %s", idx, string(bytes))
			}

		}(i)
	}
	wg.Wait()
}

func TestBlocksQueue(t *testing.T) {
	var newBlockMutex sync.RWMutex
	newBlocksQueue := list.New()

	go func() {
		for i := 0; i < 100; i++ {
			newBlockMutex.Lock()
			if newBlocksQueue.Len() == 100 {
				newBlocksQueue.Remove(newBlocksQueue.Front())
			}
			newBlocksQueue.PushBack(i)
			newBlockMutex.Unlock()
			time.Sleep(100 * time.Millisecond)
		}
	}()

	go func() {
		var elm *list.Element
		for {
			newBlockMutex.RLock()
			if newBlocksQueue.Len() != 0 {
				elm = newBlocksQueue.Front()
				newBlockMutex.RUnlock()
				break
			}
			newBlockMutex.RUnlock()
			time.Sleep(100 * time.Millisecond)
		}

		for {
			// move to next element
			for {
				newBlockMutex.RLock()
				next := elm.Next()
				t.Logf("next: %v", next)
				if next != nil {
					elm = next
					newBlockMutex.RUnlock()
					break
				}
				newBlockMutex.RUnlock()
				time.Sleep(500 * time.Millisecond)
			}
			t.Logf("value in queue: %d", elm.Value)
			time.Sleep(500 * time.Millisecond)
		}
	}()

	time.Sleep(2 * time.Second)
}

func newTestBlock(count int) []*types.Block {
	var blocks []*types.Block

	genesisBlock := &chain.GenesisBlock
	blocks = append(blocks, genesisBlock)

	prevBlock := genesisBlock
	for i := 0; i < count-1; i++ {
		b := types.NewBlock(prevBlock)
		blocks = append(blocks, b)
		prevBlock = b
	}

	return blocks
}
