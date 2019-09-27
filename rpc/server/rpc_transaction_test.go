// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rpc

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
)

type TestWalletAgent struct {
}

func (twa *TestWalletAgent) Balance(addr *types.AddressHash, tid *types.TokenID) (uint64, error) {
	return 0, nil
}

func (twa *TestWalletAgent) Utxos(
	addr *types.AddressHash, tid *types.TokenID, amount uint64,
) (utxos []*rpcpb.Utxo, err error) {

	count := uint64(2)
	if amount == 0 {
		amount = 1000000
	}
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
	fromAddr, _ := types.NewAddress("b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o")
	from := fromAddr.Hash160()
	toAddr1, _ := types.ParseAddress("b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7")
	toAddr2, _ := types.ParseAddress("b1UP5pbfJgZrF1ezoSHLdvkxvgF2BYLtGva")
	to := []*types.AddressHash{toAddr1.Hash160(), toAddr2.Hash160()}
	amounts := []uint64{24000, 38000}

	wa := new(TestWalletAgent)
	tx, _, err := rpcutil.MakeUnsignedTx(wa, from, to, amounts)
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
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 1
      },
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 2
      },
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 3
      },
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 4
      },
      "Sequence": 0
    }
  ],
  "Vout": [
    {
      "Value": 24000,
      "ScriptPubKey": "76a914ae3e96d008658db64dd4f8df2d736edbc6be1c3188ac"
    },
    {
      "Value": 38000,
      "ScriptPubKey": "76a914064b377c9555b83a43d05c773cef7c3a6209154f88ac"
    },
    {
      "Value": 1590500,
      "ScriptPubKey": "76a914ce86056786e3415530f8cc739fb414a87435b4b688ac"
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
}

func TestMakeUnsignedCombineTx(t *testing.T) {
	fromAddr, _ := types.NewAddress("b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o")
	from := fromAddr.Hash160()
	wa := new(TestWalletAgent)
	tx, _, err := rpcutil.MakeUnsignedCombineTx(wa, from)
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
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 1
      },
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 2
      },
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 3
      },
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "f180242e73f4818bf6e9bea63984ff73a23349376a0b7b6ddfa85fb01df16724",
        "Index": 4
      },
      "Sequence": 0
    }
  ],
  "Vout": [
    {
      "Value": 200000,
      "ScriptPubKey": "76a914ce86056786e3415530f8cc739fb414a87435b4b688ac"
    }
  ],
  "Data": null,
  "Magic": 0,
  "LockTime": 0
}`
	// 200000 = 1000000/2/2*5-20*21000
	if string(txBytes) != wantTxStr {
		t.Fatalf("want: %s, len: %d, got: %s, len: %d", wantTxStr, len(wantTxStr),
			string(txBytes), len(string(txBytes)))
	}
}

func hashFromUint64(n uint64) crypto.HashType {
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, n)
	return crypto.DoubleHashH(bs)
}
