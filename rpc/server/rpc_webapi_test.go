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
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/jbenet/goprocess"
	"google.golang.org/grpc"
)

const (
	blockCnt = 10

	testAddr   = "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o"
	testAmount = uint64(12345)
)

var (
	rpcAddr net.Addr
	starter sync.Once

	testWebAPIBus = eventbus.Default()
)

func init() {
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
}

func setupWebAPIMockSvr() {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	br := new(TestDetailBlockChainReader)
	was := &webapiServer{
		ChainBlockReader: br,
		proc:             goprocess.WithSignals(os.Interrupt),
		newBlocksQueue:   list.New(),
	}
	grpcSvr := grpc.NewServer()
	rpcpb.RegisterWebApiServer(grpcSvr, was)
	testWebAPIBus.Subscribe(eventbus.TopicRPCSendNewBlock, was.receiveNewBlockMsg)

	go grpcSvr.Serve(lis)
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

func TestListenAndReadNewBlock(t *testing.T) {
	// start grpc server
	starter.Do(setupWebAPIMockSvr)
	// start send blocks goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		sendWebAPITestBlocks()
	}()

	var wg sync.WaitGroup
	clientCnt := 4
	for i := 0; i < clientCnt; i++ {
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
			for i := 1; i < blockCnt+1; i++ {
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
				//bytes, err := json.MarshalIndent(block, "", "  ")
				//t.Logf("[%d] recv block: %s", idx, string(bytes))
			}
		}(i)
	}
	wg.Wait()
}

func newTestBlock(count int) []*types.Block {
	var blocks []*types.Block
	prevBlock := &chain.GenesisBlock

	miner, coinBaseAmount := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o", uint64(50000)
	for i := 0; i < count; i++ {
		coinBaseTx := types.NewTx(0, 4455, 100).
			AppendVin(txlogic.NewCoinBaseTxIn()).
			AppendVout(txlogic.MakeVout(miner, coinBaseAmount))
		b := types.NewBlock(prevBlock).AppendTx(coinBaseTx)
		blocks = append(blocks, b)
		prevBlock = b
	}

	return blocks
}

func TestDetailTxOut(t *testing.T) {
	// normal txout
	txOut := txlogic.MakeVout(testAddr, testAmount)
	tx := types.NewTx(0, 4455, 1000).AppendVout(txOut)
	txHash, _ := tx.TxHash()
	detail, err := detailTxOut(txHash, tx.Vout[0], 0)
	if err != nil {
		t.Fatal(err)
	}
	disasmScript := "OP_DUP OP_HASH160 ce86056786e3415530f8cc739fb414a87435b4b6" +
		" OP_EQUALVERIFY OP_CHECKSIG"
	if detail.Addr != testAddr || detail.Value != testAmount ||
		detail.ScriptDisasm != disasmScript ||
		detail.Type != rpcpb.TxOutDetail_pay_to_pubkey {
		t.Fatalf("normal detail tx out, want: %s %d %s %v, got: %+v", testAddr,
			testAmount, disasmScript, rpcpb.TxOutDetail_pay_to_pubkey, detail)
	}

	// pay to token issue
	name, sym, decimal, supply := "fox", "FOX", uint8(3), uint64(1000000)
	tag := types.NewTokenTag(name, sym, decimal)
	txOut, _ = txlogic.MakeIssueTokenVout(testAddr, tag, supply)
	tx = types.NewTx(0, 4455, 1000).AppendVout(txOut)
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
	if detail.Addr != testAddr || detail.Value != wantValue ||
		detail.ScriptDisasm != disasmScript ||
		detail.Type != rpcpb.TxOutDetail_token_issue ||
		tokenPbTag.Name != name || tokenPbTag.Symbol != sym ||
		tokenPbTag.Decimals != uint32(decimal) || tokenPbTag.Supply != supply {
		t.Fatalf("detail token issue tx out, want: %s %d %s %v, got: %+v",
			testAddr, wantValue, disasmScript, rpcpb.TxOutDetail_token_issue, detail)
	}
	detailStr := `{
  "addr": "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o",
  "value": 1000000000,
  "script_pub_key": "dqkUzoYFZ4bjQVUw+Mxzn7QUqHQ1tLaIrAROYW1ldQNmb3h1BlN5bWJvbHUDRk9YdQZBbW91bnR1CEBCDwAAAAAAdQhEZWNpbWFsc3UBA3U=",
  "script_disasm": "OP_DUP OP_HASH160 ce86056786e3415530f8cc739fb414a87435b4b6 OP_EQUALVERIFY OP_CHECKSIG 4e616d65 OP_DROP 666f78 OP_DROP 53796d626f6c OP_DROP 464f58 OP_DROP 416d6f756e74 OP_DROP 40420f0000000000 OP_DROP 446563696d616c73 OP_DROP 03 OP_DROP",
  "type": 5,
  "Appendix": {
    "TokenIssueInfo": {
      "token_tag": {
        "name": "fox",
        "symbol": "FOX",
        "supply": 1000000,
        "decimals": 3
      }
    }
  }
}`
	bytes, _ := json.MarshalIndent(detail, "", "  ")
	if detailStr != string(bytes) {
		t.Errorf("token issue vout want: %s, got: %s", detailStr, string(bytes))
	}

	// pay to token transfer
	tid := types.NewTokenID(txHash, 0)
	txOut, _ = txlogic.MakeTokenVout(testAddr, tid, testAmount)
	tx = types.NewTx(0, 4455, 100).AppendVout(txOut)
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
	//disasmScript = fmt.Sprintf("OP_DUP OP_HASH160 ce86056786e3415530f8cc739fb414a87435b4b6 "+
	//	"OP_EQUALVERIFY OP_CHECKSIG 546f6b656e547848617368 OP_DROP %s OP_DROP"+
	//	" 546f6b656e54784f7574496478 OP_DROP 00000000 OP_DROP 416d6f756e74 OP_DROP"+
	//	" 3930000000000000 OP_DROP", hashT)
	// NOTE: in order to compare detail.ScriptDisasm,
	// token id in script pubkey needs reverse hash string
	if detail.Addr != testAddr || detail.Value != testAmount ||
		//detail.ScriptDisasm != disasmScript || // need reverse hash string
		detail.Type != rpcpb.TxOutDetail_token_transfer ||
		!reflect.DeepEqual(gotTid.Hash, tid.Hash[:]) ||
		gotTid.Index != tid.Index {
		t.Fatalf("detail token transfer tx out, want: %s %d %s %v, got: %+v",
			testAddr, testAmount, disasmScript, rpcpb.TxOutDetail_token_transfer, detail)
	}
	bytes, _ = json.MarshalIndent(detail, "", "  ")
	detailStr = `{
  "addr": "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o",
  "value": 12345,
  "script_pub_key": "dqkUzoYFZ4bjQVUw+Mxzn7QUqHQ1tLaIrAtUb2tlblR4SGFzaHUgXD0hWiS6DyGHBWlhNaF7yHmpBRUZG9IGd9co7ALegXZ1DVRva2VuVHhPdXRJZHh1BAAAAAB1BkFtb3VudHUIOTAAAAAAAAB1",
  "script_disasm": "OP_DUP OP_HASH160 ce86056786e3415530f8cc739fb414a87435b4b6 OP_EQUALVERIFY OP_CHECKSIG 546f6b656e547848617368 OP_DROP 5c3d215a24ba0f218705696135a17bc879a90515191bd20677d728ec02de8176 OP_DROP 546f6b656e54784f7574496478 OP_DROP 00000000 OP_DROP 416d6f756e74 OP_DROP 3930000000000000 OP_DROP",
  "type": 6,
  "Appendix": {
    "TokenTransferInfo": {
      "token_id": {
        "hash": "XD0hWiS6DyGHBWlhNaF7yHmpBRUZG9IGd9co7ALegXY="
      }
    }
  }
}`
	if detailStr != string(bytes) {
		t.Errorf("token transfer vout want: %s, got: %s", detailStr, string(bytes))
	}
}

type TestDetailBlockChainReader struct{}

func (r *TestDetailBlockChainReader) LoadTxByHash(crypto.HashType) (*types.Transaction, error) {
	from, to, amount := testAddr, "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ", uint64(300)
	prevHash := hashFromUint64(1)
	return genTestTx(from, to, amount, &prevHash), nil
}

func (r *TestDetailBlockChainReader) LoadBlockByHash(crypto.HashType) (*types.Block, error) {
	addr, amount := "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ", uint64(50000)
	coinBaseTx := types.NewTx(0, 4455, 100).
		AppendVin(txlogic.NewCoinBaseTxIn()).
		AppendVout(txlogic.MakeVout(addr, amount))
	tx, _ := r.LoadTxByHash(crypto.HashType{})
	block := types.NewBlock(&chain.GenesisBlock).AppendTx(coinBaseTx, tx)
	block.Height = 10
	return block, nil
}

func (r *TestDetailBlockChainReader) LoadBlockInfoByTxHash(
	hash crypto.HashType,
) (*types.Block, uint32, error) {
	return nil, 0, nil
}

func (r *TestDetailBlockChainReader) EternalBlock() *types.Block {
	block, _ := r.LoadBlockByHash(crypto.HashType{})
	block.Height = 1
	return block
}

func genTestTx(from, to string, amount uint64, prevHash *crypto.HashType) *types.Transaction {
	// make tx
	tx := types.NewTx(0, 4455, 1000)
	// append vin
	op, uw := types.NewOutPoint(prevHash, 0), txlogic.NewUtxoWrap(from, 10, amount+1)
	inUtxo := txlogic.MakePbUtxo(op, uw)
	tx.AppendVin(txlogic.MakeVin(inUtxo, 0))
	// append vout
	tx.AppendVout(txlogic.MakeVout(to, amount))
	return tx
}

func TestDetailTxAndBlock(t *testing.T) {
	prevHash := hashFromUint64(1)
	from, to, amount := testAddr, "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ", uint64(300)
	tx := genTestTx(from, to, amount, &prevHash)
	// detail tx
	blockReader := new(TestDetailBlockChainReader)
	detail, err := detailTx(tx, blockReader)
	if err != nil {
		t.Fatal(err)
	}
	txDetailStr := `{
  "hash": "e11ab659fe76e3db55e58716f241f8eaa5257358b851f6edb57f19ec480b32d8",
  "vin": [
    {
      "prev_out_detail": {
        "addr": "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ",
        "value": 300,
        "script_pub_key": "dqkUUFcMxzuxilH8QVPuxo0h0RBdMm6IrA==",
        "script_disasm": "OP_DUP OP_HASH160 50570cc73bb18a51fc4153eec68d21d1105d326e OP_EQUALVERIFY OP_CHECKSIG",
        "type": 1,
        "Appendix": null
      },
      "prev_out_point": {
        "hash": "CFM/a71JdRE+TwyxBKvN7B2G2Z1XgrSpp/YnDA67aic="
      }
    }
  ],
  "vout": [
    {
      "addr": "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ",
      "value": 300,
      "script_pub_key": "dqkUUFcMxzuxilH8QVPuxo0h0RBdMm6IrA==",
      "script_disasm": "OP_DUP OP_HASH160 50570cc73bb18a51fc4153eec68d21d1105d326e OP_EQUALVERIFY OP_CHECKSIG",
      "type": 1,
      "Appendix": null
    }
  ]
}`
	txDetailBytes, _ := json.MarshalIndent(detail, "", "  ")
	if string(txDetailBytes) != txDetailStr {
		t.Fatalf("tx detail want: %s, got: %s", txDetailStr, string(txDetailBytes))
	}

	// test block
	// gen coinbase tx
	coinBaseAmount := uint64(50000)
	coinBaseTx := types.NewTx(0, 4455, 100).
		AppendVin(txlogic.NewCoinBaseTxIn()).
		AppendVout(txlogic.MakeVout(from, coinBaseAmount))
	block := types.NewBlock(&chain.GenesisBlock).AppendTx(coinBaseTx, tx)
	blockDetail, err := detailBlock(block, blockReader)
	if err != nil {
		t.Fatal(err)
	}

	blockDetailStr := `{
  "height": 1,
  "hash": "c0e96e998eb01eea5d5acdaeb80acd943477e6119dcd82a419089331229c7453",
  "prev_block_hash": "286845d9b7580cac0bba6586c4973383ed0c29466ea8917dbfb0a0c51cb76635",
  "coin_base": "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o",
  "confirmed": true,
  "txs": [
    {
      "hash": "0ee737b264fefb5aa55cba25becdd29bcdb4f84426a4efbe7760a67986c761e1",
      "vin": [
        {}
      ],
      "vout": [
        {
          "addr": "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o",
          "value": 50000,
          "script_pub_key": "dqkUzoYFZ4bjQVUw+Mxzn7QUqHQ1tLaIrA==",
          "script_disasm": "OP_DUP OP_HASH160 ce86056786e3415530f8cc739fb414a87435b4b6 OP_EQUALVERIFY OP_CHECKSIG",
          "type": 1,
          "Appendix": null
        }
      ]
    },
    {
      "hash": "e11ab659fe76e3db55e58716f241f8eaa5257358b851f6edb57f19ec480b32d8",
      "vin": [
        {
          "prev_out_detail": {
            "addr": "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ",
            "value": 300,
            "script_pub_key": "dqkUUFcMxzuxilH8QVPuxo0h0RBdMm6IrA==",
            "script_disasm": "OP_DUP OP_HASH160 50570cc73bb18a51fc4153eec68d21d1105d326e OP_EQUALVERIFY OP_CHECKSIG",
            "type": 1,
            "Appendix": null
          },
          "prev_out_point": {
            "hash": "CFM/a71JdRE+TwyxBKvN7B2G2Z1XgrSpp/YnDA67aic="
          }
        }
      ],
      "vout": [
        {
          "addr": "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ",
          "value": 300,
          "script_pub_key": "dqkUUFcMxzuxilH8QVPuxo0h0RBdMm6IrA==",
          "script_disasm": "OP_DUP OP_HASH160 50570cc73bb18a51fc4153eec68d21d1105d326e OP_EQUALVERIFY OP_CHECKSIG",
          "type": 1,
          "Appendix": null
        }
      ]
    }
  ]
}`
	blockDetailBytes, _ := json.MarshalIndent(blockDetail, "", "  ")
	if blockDetailStr != string(blockDetailBytes) {
		t.Fatalf("block detail want: %s, got: %s", blockDetailStr, string(blockDetailBytes))
	}
}
