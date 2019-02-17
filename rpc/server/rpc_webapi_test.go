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
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txlogic"
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

			blockReq := &rpcpb.ListenBlocksReq{}
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
		for i := 0; i < 30; i++ {
			newBlockMutex.Lock()
			if newBlocksQueue.Len() == 30 {
				newBlocksQueue.Remove(newBlocksQueue.Front())
			}
			newBlocksQueue.PushBack(i)
			newBlockMutex.Unlock()
			time.Sleep(100 * time.Millisecond)
		}
	}()

	ch := make(chan bool)
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
				time.Sleep(200 * time.Millisecond)
			}
			t.Logf("value in queue: %d", elm.Value)
			if elm.Value == 10 {
				<-ch
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()
	ch <- true
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

func TestDetailTxOut(t *testing.T) {
	addr, amount := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o", uint64(12345)
	// normal txout
	txOut := txlogic.MakeVout(addr, amount)
	tx := types.NewTx(0, 4455, time.Now().Unix()).AppendVout(txOut)
	txHash, _ := tx.TxHash()
	detail, err := detailTxOut(txHash, tx.Vout[0], 0)
	if err != nil {
		t.Fatal(err)
	}
	disasmScript := "OP_DUP OP_HASH160 ce86056786e3415530f8cc739fb414a87435b4b6" +
		" OP_EQUALVERIFY OP_CHECKSIG"
	if detail.Addr != addr || detail.Value != amount ||
		detail.ScriptDisasm != disasmScript ||
		detail.Type != rpcpb.TxOutDetail_pay_to_pubkey {
		t.Fatalf("normal detail tx out, want: %s %d %s %v, got: %+v", addr, amount,
			disasmScript, rpcpb.TxOutDetail_pay_to_pubkey, detail)
	}

	// pay to token issue
	name, sym, decimal, supply := "fox", "FOX", uint8(3), uint64(1000000)
	tag := types.NewTokenTag(name, sym, decimal)
	txOut, _ = txlogic.MakeIssueTokenVout(addr, tag, supply)
	tx = types.NewTx(0, 4455, time.Now().Unix()).AppendVout(txOut)
	txHash, _ = tx.TxHash()
	detail, err = detailTxOut(txHash, tx.Vout[0], 0)
	if err != nil {
		t.Fatal(err)
	}
	disasmScript = "OP_DUP OP_HASH160 ce86056786e3415530f8cc739fb414a87435b4b6" +
		" OP_EQUALVERIFY OP_CHECKSIG 4e616d65 OP_DROP 666f78 OP_DROP 53796d626f6c" +
		" OP_DROP 464f58 OP_DROP 416d6f756e74 OP_DROP 40420f0000000000 OP_DROP" +
		" 446563696d616c73 OP_DROP 03 OP_DROP"
	tokenIssueInfo, ok := detail.Appendix.(*rpcpb.TxOutDetail_TokenIssueInfo)
	if !ok {
		t.Fatalf("detail token issue, Appendix want Type: *TxOutDetail_TokenIssueInfo"+
			", got: %T, value: %+v", detail.Appendix, detail.Appendix)
	}
	tokenPbTag := tokenIssueInfo.TokenIssueInfo.TokenTag
	wantValue := supply
	for i := uint8(0); i < decimal; i++ {
		wantValue *= 10
	}
	if detail.Addr != addr || detail.Value != wantValue ||
		detail.ScriptDisasm != disasmScript ||
		detail.Type != rpcpb.TxOutDetail_token_issue ||
		tokenPbTag.Name != name || tokenPbTag.Symbol != sym ||
		tokenPbTag.Decimals != uint32(decimal) || tokenPbTag.Supply != supply {
		t.Fatalf("detail token issue tx out, want: %s %d %s %v, got: %+v",
			addr, wantValue, disasmScript, rpcpb.TxOutDetail_token_issue, detail)
	}

	// pay to token transfer
	tid := types.NewTokenID(txHash, 0)
	txOut, _ = txlogic.MakeTokenVout(addr, tid, amount)
	tx = types.NewTx(0, 4455, time.Now().Unix()).AppendVout(txOut)
	txHash, _ = tx.TxHash()
	detail, err = detailTxOut(txHash, tx.Vout[0], 0)
	if err != nil {
		t.Fatal(err)
	}
	tokenTransferInfo, ok := detail.Appendix.(*rpcpb.TxOutDetail_TokenTransferInfo)
	if !ok {
		t.Fatalf("detail token transfer tx out, Appendix want Type: "+
			"*TxOutDetail_TokenIssueInfo, got: %T, value: %+v",
			detail.Appendix, detail.Appendix)
	}
	gotTid := tokenTransferInfo.TokenTransferInfo.TokenId
	if detail.Addr != addr || detail.Value != amount ||
		//detail.ScriptDisasm != disasmScript || // need reverse hash string
		detail.Type != rpcpb.TxOutDetail_token_transfer ||
		!reflect.DeepEqual(gotTid.Hash, tid.Hash[:]) ||
		gotTid.Index != tid.Index {
		t.Fatalf("detail token transfer tx out, want: %s %d %s %v, got: %+v",
			addr, amount, disasmScript, rpcpb.TxOutDetail_token_transfer, detail)
	}
}
