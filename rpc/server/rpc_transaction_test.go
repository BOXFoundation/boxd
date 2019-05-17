// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
)

type TestWalletAgent struct {
}

func (twa *TestWalletAgent) Balance(addr string, tid *types.TokenID) (uint64, error) {
	return 0, nil
}

func (twa *TestWalletAgent) Utxos(
	addr string, tid *types.TokenID, amount uint64,
) (utxos []*rpcpb.Utxo, err error) {

	count := uint64(2)
	ave := amount / count

	for h, remain := uint32(0), amount; remain <= amount; h++ {
		//value := ave/2 + uint64(rand.Intn(int(ave)/2))
		value := ave / 2
		// outpoint
		//hash := hashFromUint64(uint64(time.Now().UnixNano()))
		hash := hashFromUint64(100)
		op := types.NewOutPoint(&hash, h%10)
		// utxo wrap
		uw := txlogic.NewUtxoWrap(addr, h, value)
		utxo := txlogic.MakePbUtxo(op, uw)
		remain -= value
		utxos = append(utxos, utxo)
	}
	return
}

func TestMakeUnsignedTx(t *testing.T) {
	from := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o"
	to := []string{
		"b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7",
		"b1UP5pbfJgZrF1ezoSHLdvkxvgF2BYLtGva",
	}
	amounts := []uint64{24000, 38000}
	fee := uint64(1000)

	wa := new(TestWalletAgent)
	tx, utxos, err := rpcutil.MakeUnsignedTx(wa, from, to, amounts, fee)
	if err != nil {
		t.Fatal(err)
	}
	rawMsgs, err := MakeTxRawMsgsForSign(tx, utxos...)
	if err != nil {
		t.Fatal(err)
	}
	txBytes, _ := json.MarshalIndent(tx, "", "  ")

	wantTxStr := `{
  "Version": 0,
  "Vin": [
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 0
      },
      "ScriptSig": "",
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 1
      },
      "ScriptSig": "",
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 2
      },
      "ScriptSig": "",
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 3
      },
      "ScriptSig": "",
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 4
      },
      "ScriptSig": "",
      "Sequence": 0
    }
  ],
  "Vout": [
    {
      "value": 24000,
      "script_pub_key": "dqkUrj6W0AhljbZN1PjfLXNu28a+HDGIrA=="
    },
    {
      "value": 38000,
      "script_pub_key": "dqkUBks3fJVVuDpD0Fx3PO98OmIJFU+IrA=="
    },
    {
      "value": 15750,
      "script_pub_key": "dqkUzoYFZ4bjQVUw+Mxzn7QUqHQ1tLaIrA=="
    }
  ],
  "Data": null,
  "Magic": 0,
  "LockTime": 0
}`
	if string(txBytes) != wantTxStr {
		t.Fatalf("want: %s, len: %d, got: %s, len: %d", wantTxStr, len(wantTxStr),
			string(txBytes), len(string(txBytes)))
	}

	wantRawMsgs := []string{
		"123f0a220a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1121976a914ce86056786e3415530f8cc739fb414a87435b4b688ac12260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100112260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100212260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100312260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f110041a1f08c0bb01121976a914ae3e96d008658db64dd4f8df2d736edbc6be1c3188ac1a1f08f0a802121976a914064b377c9555b83a43d05c773cef7c3a6209154f88ac1a1e08867b121976a914ce86056786e3415530f8cc739fb414a87435b4b688ac",
		"12240a220a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f112410a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f11001121976a914ce86056786e3415530f8cc739fb414a87435b4b688ac12260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100212260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100312260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f110041a1f08c0bb01121976a914ae3e96d008658db64dd4f8df2d736edbc6be1c3188ac1a1f08f0a802121976a914064b377c9555b83a43d05c773cef7c3a6209154f88ac1a1e08867b121976a914ce86056786e3415530f8cc739fb414a87435b4b688ac",
		"12240a220a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f112260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100112410a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f11002121976a914ce86056786e3415530f8cc739fb414a87435b4b688ac12260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100312260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f110041a1f08c0bb01121976a914ae3e96d008658db64dd4f8df2d736edbc6be1c3188ac1a1f08f0a802121976a914064b377c9555b83a43d05c773cef7c3a6209154f88ac1a1e08867b121976a914ce86056786e3415530f8cc739fb414a87435b4b688ac",
		"12240a220a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f112260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100112260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100212410a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f11003121976a914ce86056786e3415530f8cc739fb414a87435b4b688ac12260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f110041a1f08c0bb01121976a914ae3e96d008658db64dd4f8df2d736edbc6be1c3188ac1a1f08f0a802121976a914064b377c9555b83a43d05c773cef7c3a6209154f88ac1a1e08867b121976a914ce86056786e3415530f8cc739fb414a87435b4b688ac",
		"12240a220a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f112260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100112260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100212260a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f1100312410a240a202467f11db05fa8df6d7b0b6a374933a273ff8439a6bee9f68b81f4732e2480f11004121976a914ce86056786e3415530f8cc739fb414a87435b4b688ac1a1f08c0bb01121976a914ae3e96d008658db64dd4f8df2d736edbc6be1c3188ac1a1f08f0a802121976a914064b377c9555b83a43d05c773cef7c3a6209154f88ac1a1e08867b121976a914ce86056786e3415530f8cc739fb414a87435b4b688ac",
	}
	for i := 0; i < len(rawMsgs); i++ {
		if hex.EncodeToString(rawMsgs[i]) != wantRawMsgs[i] {
			t.Fatalf("for raw msg %d, want: %s, got: %s", i, wantRawMsgs[i], rawMsgs[i])
		}
	}
}

/* todo
func TestMakeUnsignedContractTx(t *testing.T) {
	addr:="b1YLUNwJD124sv9piRvkqcmfcujTZtHhHSz"
	amount:=uint64(200)
	gasPrice:=uint64(10)
	gasLimit:=uint64(40)
	code:="608060405260f7806100126000396000f3fe6080604052600436106039576000357c0100000000000000000000000000000000000000000000000000000000900480632e1a7d4d14603b575b005b348015604657600080fd5b50607060048036036020811015605b57600080fd5b81019080803590602001909291905050506072565b005b6127108111151515608257600080fd5b3373ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f1935050505015801560c7573d6000803e3d6000fd5b505056fea165627a7a7230582041951f9857bb67cda6bccbb59f6fdbf38eeddc244530e577d8cad6194941d38c0029"

	byteCode,err:=hex.DecodeString(code)
	if err!=nil{
		t.Fatal(err)
	}

	wa := new(TestWalletAgent)
	tx, utxos, err := rpcutil.MakeUnsignedContractTx(wa,addr,amount,gasLimit,gasPrice,byteCode)
	if err != nil {
		t.Fatal(err)
	}
	rawMsgs, err := MakeTxRawMsgsForSign(tx, utxos...)
	if err != nil {
		t.Fatal(err)
	}
	txBytes, _ := json.MarshalIndent(tx, "", "  ")

	wantTxStr:=``


	if string(txBytes)!=wantTxStr{
		t.Fatalf("want: %s, len: %d, got: %s, len: %d", wantTxStr, len(wantTxStr),
			string(txBytes), len(string(txBytes)))
	}
	wantRawMsgs:=""
	if hex.EncodeToString(rawMsgs[0])!=wantRawMsgs{
		t.Fatalf("for raw msg  want: %s, got: %s",  wantRawMsgs, rawMsgs)
	}

}
*/

func hashFromUint64(n uint64) crypto.HashType {
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, n)
	return crypto.DoubleHashH(bs)
}
