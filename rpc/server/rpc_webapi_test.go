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
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/vm"
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
		subscribeBlocks:  true,
		bus:              testWebAPIBus,
		newBlocksQueue:   list.New(),
	}
	grpcSvr := grpc.NewServer()
	rpcpb.RegisterWebApiServer(grpcSvr, was)

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

func _TestClientListenNewBlocks(t *testing.T) {
	rpcAddr := "127.0.0.1:19111"
	conn, err := grpc.Dial(rpcAddr, grpc.WithInsecure())
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
	for {
		block, err := stream.Recv()
		if err == io.EOF {
			t.Fatalf("%v.ListenAndReadNewBlock(_) = _, %v", client, err)
			break
		}
		if err != nil {
			t.Fatalf("%v.ListenAndReadNewBlock(_) = _, %v", client, err)
		}
		logger.Infof("block hegiht: %d, size: %d", block.Height, block.Size_)
	}
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
			AppendVin(types.NewCoinBaseTxIn()).
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
	detail, err := detailTxOut(txHash, tx.Vout[0], 0, testDetailReader)
	if err != nil {
		t.Fatal(err)
	}
	disasmScript := "OP_DUP OP_HASH160 ce86056786e3415530f8cc739fb414a87435b4b6" +
		" OP_EQUALVERIFY OP_CHECKSIG"
	if detail.Addr != testAddr || detail.Value != testAmount ||
		detail.ScriptDisasm != disasmScript ||
		detail.Type != rpcpb.TxOutDetail_pay_to_pubkey_hash {
		t.Fatalf("normal detail tx out, want: %s %d %s %v, got: %+v", testAddr,
			testAmount, disasmScript, rpcpb.TxOutDetail_pay_to_pubkey_hash, detail)
	}

	// pay to token issue
	name, sym, decimal, supply := "fox", "FOX", uint32(3), uint64(1000000)
	tag := txlogic.NewTokenTag(name, sym, decimal, supply)
	txOut, _ = txlogic.MakeIssueTokenVout(testAddr, tag)
	tx = types.NewTx(0, 4455, 1000).AppendVout(txOut)
	txHash, _ = tx.TxHash()
	detail, err = detailTxOut(txHash, tx.Vout[0], 0, testDetailReader)
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
	for i := uint32(0); i < decimal; i++ {
		wantValue *= 10
	}
	if detail.Addr != testAddr || detail.Value != wantValue ||
		detail.ScriptDisasm != disasmScript ||
		detail.Type != rpcpb.TxOutDetail_token_issue ||
		tokenPbTag.Name != name || tokenPbTag.Symbol != sym ||
		tokenPbTag.Decimal != uint32(decimal) || tokenPbTag.Supply != supply {
		t.Fatalf("detail token issue tx out, want: %s %d %s %v, got: %+v",
			testAddr, wantValue, disasmScript, rpcpb.TxOutDetail_token_issue, detail)
	}
	detailStr := `{
  "addr": "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o",
  "value": 1000000000,
  "script_pub_key": "76a914ce86056786e3415530f8cc739fb414a87435b4b688ac044e616d657503666f78750653796d626f6c7503464f587506416d6f756e74750840420f00000000007508446563696d616c7375010375",
  "script_disasm": "OP_DUP OP_HASH160 ce86056786e3415530f8cc739fb414a87435b4b6 OP_EQUALVERIFY OP_CHECKSIG 4e616d65 OP_DROP 666f78 OP_DROP 53796d626f6c OP_DROP 464f58 OP_DROP 416d6f756e74 OP_DROP 40420f0000000000 OP_DROP 446563696d616c73 OP_DROP 03 OP_DROP",
  "type": 3,
  "Appendix": {
    "TokenIssueInfo": {
      "token_tag": {
        "name": "fox",
        "symbol": "FOX",
        "supply": 1000000,
        "decimal": 3
      }
    }
  }
}`
	bytes, _ := json.MarshalIndent(detail, "", "  ")
	if detailStr != string(bytes) {
		t.Errorf("token issue vout want: %s, got: %s", detailStr, string(bytes))
	}

	// pay to token transfer
	tid := txlogic.NewTokenID(txHash, 0)
	txOut, _ = txlogic.MakeTokenVout(testAddr, tid, testAmount)
	tx = types.NewTx(0, 4455, 100).AppendVout(txOut)
	txHash, _ = tx.TxHash()
	detail, err = detailTxOut(txHash, tx.Vout[0], 0, testDetailReader)
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
		gotTid != txlogic.EncodeOutPoint(txlogic.ConvOutPoint((*types.OutPoint)(tid))) {
		t.Fatalf("detail token transfer tx out, want: %s %d %s %v, got: %+v",
			testAddr, testAmount, disasmScript, rpcpb.TxOutDetail_token_transfer, detail)
	}
	bytes, _ = json.MarshalIndent(detail, "", "  ")
	detailStr = `{
  "addr": "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o",
  "value": 12345,
  "script_pub_key": "76a914ce86056786e3415530f8cc739fb414a87435b4b688ac0b546f6b656e54784861736875205c3d215a24ba0f218705696135a17bc879a90515191bd20677d728ec02de8176750d546f6b656e54784f75744964787504000000007506416d6f756e747508393000000000000075",
  "script_disasm": "OP_DUP OP_HASH160 ce86056786e3415530f8cc739fb414a87435b4b6 OP_EQUALVERIFY OP_CHECKSIG 546f6b656e547848617368 OP_DROP 5c3d215a24ba0f218705696135a17bc879a90515191bd20677d728ec02de8176 OP_DROP 546f6b656e54784f7574496478 OP_DROP 00000000 OP_DROP 416d6f756e74 OP_DROP 3930000000000000 OP_DROP",
  "type": 4,
  "Appendix": {
    "TokenTransferInfo": {
      "token_id": "4yN36qpbC4xZVaZ353Czt5TaowNovq55CpDQomCpQXDKR61chHh"
    }
  }
}`
	if detailStr != string(bytes) {
		t.Errorf("token transfer vout want: %s, len: %d, got: %s, len: %d",
			detailStr, len(detailStr), string(bytes), len(string(bytes)))
	}
}

type TestDetailBlockChainReader struct{}

var (
	testDetailReader = new(TestDetailBlockChainReader)
)

func (r *TestDetailBlockChainReader) LoadTxByHash(crypto.HashType) (*types.Transaction, error) {
	from, to, amount := testAddr, "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ", uint64(300)
	prevHash := hashFromUint64(1)
	return genTestTx(from, to, amount, &prevHash), nil
}

func (r *TestDetailBlockChainReader) GetDataFromDB([]byte) ([]byte, error) {
	return nil, nil
}

func (r *TestDetailBlockChainReader) GetEvmByHeight(msg types.Message, height uint32) (*vm.EVM, func() error, error) {
	return nil, nil, nil
}

func (r *TestDetailBlockChainReader) NonceByHeight(address *types.AddressHash, height uint32) (uint64, error) {
	return 0, nil
}

func (r *TestDetailBlockChainReader) ReadBlockFromDB(*crypto.HashType) (*types.Block, int, error) {
	addr, amount := "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ", uint64(50000)
	coinBaseTx := types.NewTx(0, 4455, 100).
		AppendVin(types.NewCoinBaseTxIn()).
		AppendVout(txlogic.MakeVout(addr, amount))
	_, tx, _ := r.LoadBlockInfoByTxHash(crypto.HashType{})
	block := types.NewBlock(&chain.GenesisBlock).AppendTx(coinBaseTx, tx)
	block.Header.Height = 10
	return block, 10240, nil
}

func (r *TestDetailBlockChainReader) LoadBlockInfoByTxHash(
	hash crypto.HashType,
) (*types.Block, *types.Transaction, error) {
	return nil, nil, nil
}

func (r *TestDetailBlockChainReader) EternalBlock() *types.Block {
	block, _, _ := r.ReadBlockFromDB(&crypto.HashType{})
	block.Header.Height = 1
	return block
}

func genTestTx(from, to string, amount uint64, prevHash *crypto.HashType) *types.Transaction {
	// make tx
	tx := types.NewTx(0, 4455, 1000)
	// append vin
	op := types.NewOutPoint(prevHash, 0)
	tx.AppendVin(txlogic.MakeVin(op, 0))
	// append vout
	tx.AppendVout(txlogic.MakeVout(to, amount))
	return tx
}

func _TestDetailTxAndBlock(t *testing.T) {
	prevHash := hashFromUint64(1)
	from, to, amount := testAddr, "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ", uint64(300)
	tx := genTestTx(from, to, amount, &prevHash)
	// detail tx
	blockReader := new(TestDetailBlockChainReader)
	detail, err := detailTx(tx, blockReader, nil, false, true)
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
        "script_pub_key": "76a91450570cc73bb18a51fc4153eec68d21d1105d326e88ac",
        "script_disasm": "OP_DUP OP_HASH160 50570cc73bb18a51fc4153eec68d21d1105d326e OP_EQUALVERIFY OP_CHECKSIG",
        "type": 1,
        "Appendix": null
      },
      "prev_out_point": "2Kd3Ayy6YBECCW4KAZAdP1K7Y3XFMkoPEy5oq3JZWxeG6DoWzL7"
    }
  ],
  "vout": [
    {
      "addr": "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ",
      "value": 300,
      "script_pub_key": "76a91450570cc73bb18a51fc4153eec68d21d1105d326e88ac",
      "script_disasm": "OP_DUP OP_HASH160 50570cc73bb18a51fc4153eec68d21d1105d326e OP_EQUALVERIFY OP_CHECKSIG",
      "type": 1,
      "Appendix": null
    }
  ]
}`
	txDetailBytes, _ := json.MarshalIndent(detail, "", "  ")
	if string(txDetailBytes) != txDetailStr {
		t.Fatalf("tx detail want: %s, len: %d, got: %s, len: %d",
			txDetailStr, len(txDetailStr), string(txDetailBytes), len(string(txDetailBytes)))
	}

	// test block
	// gen coinbase tx
	coinBaseAmount := uint64(50000)
	coinBaseTx := types.NewTx(0, 4455, 100).
		AppendVin(types.NewCoinBaseTxIn()).
		AppendVout(txlogic.MakeVout(from, coinBaseAmount))
	block := types.NewBlock(&chain.GenesisBlock).AppendTx(coinBaseTx, tx)
	blockDetail, err := detailBlock(block, blockReader, nil, true)
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
          "script_pub_key": "76a914ce86056786e3415530f8cc739fb414a87435b4b688ac",
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
            "script_pub_key": "76a91450570cc73bb18a51fc4153eec68d21d1105d326e88ac",
            "script_disasm": "OP_DUP OP_HASH160 50570cc73bb18a51fc4153eec68d21d1105d326e OP_EQUALVERIFY OP_CHECKSIG",
            "type": 1,
            "Appendix": null
          },
          "prev_out_point": "2Kd3Ayy6YBECCW4KAZAdP1K7Y3XFMkoPEy5oq3JZWxeG6DoWzL7"
        }
      ],
      "vout": [
        {
          "addr": "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ",
          "value": 300,
          "script_pub_key": "76a91450570cc73bb18a51fc4153eec68d21d1105d326e88ac",
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

func TestConvOutPoint(t *testing.T) {
	hashStr := "c0e96e998eb01eea5d5acdaeb80acd943477e6119dcd82a419089331229c7453"
	hash := new(crypto.HashType)
	if err := hash.SetString(hashStr); err != nil {
		t.Fatal(err)
	}
	index := uint32(3)
	op := txlogic.NewPbOutPoint(hash, index)
	got := txlogic.EncodeOutPoint(op)
	want := "7TzjaadMY4QYchK4obssFrE7BSPs7HkShdfSHux9z8ddYJghQMm"
	if got != want {
		t.Fatalf("want: %s, got: %s", want, got)
	}
}
