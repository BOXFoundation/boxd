// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
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
	time.Sleep(100 * time.Millisecond)
}

func sendWebAPITestBlocks() {
	blocks := newTestBlock(blockCnt)
	for _, b := range blocks {
		testWebAPIBus.Publish(eventbus.TopicRPCSendNewBlock, b)
		time.Sleep(10 * time.Millisecond)
	}
	// stop current test case
	time.Sleep(100 * time.Millisecond)
	proc, _ := os.FindProcess(os.Getpid())
	proc.Signal(os.Interrupt)
}

func TestListenAndReadNewBlock(t *testing.T) {
	// start grpc server
	starter.Do(setupWebAPIMockSvr)
	// start send blocks goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		sendWebAPITestBlocks()
	}()
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
			break
		}
		if err != nil {
			t.Fatalf("%v.ListenAndReadNewBlock(_) = _, %v", client, err)
		}
		if uint32(i) != block.Height {
			t.Fatalf("block height mismatch, want: %d, got: %d", i, block.Height)
		}

		bytes, err := json.MarshalIndent(block, "", "  ")
		t.Logf("recv block: %s", string(bytes))
	}
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
