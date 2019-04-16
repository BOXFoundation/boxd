// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
)

func TestMakeUnsignedTx(t *testing.T) {

	from := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o"
	to := []string{
		"b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ",
		"b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7",
	}
	amounts := []uint64{100, 200}
	changeAmt := uint64(200)
	prevHash1 := hashFromUint64(1)
	utxoValue1, utxoValue2 := uint64(200), uint64(400)
	op, uw := types.NewOutPoint(&prevHash1, 0), NewUtxoWrap(from, 2, utxoValue1)
	utxo1 := MakePbUtxo(op, uw)
	prevHash2 := hashFromUint64(2)
	op, uw = types.NewOutPoint(&prevHash2, 0), NewUtxoWrap(from, 3, utxoValue2)
	utxo2 := MakePbUtxo(op, uw)

	tx, err := MakeUnsignedTx(from, to, amounts, changeAmt, utxo1, utxo2)
	if err != nil {
		t.Fatal(err)
	}

	txStr := `{
  "Version": 0,
  "Vin": [
    {
      "PrevOutPoint": {
        "Hash": "276abb0e0c27f6a7a9b482579dd9861deccdab04b10c4f3e117549bd6b3f5308",
        "Index": 0
      },
      "ScriptSig": "",
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "18bc65f0e5c91ffef96a3e4bc923bc31a82ce58bebd05105d4074d3b7264e63f",
        "Index": 0
      },
      "ScriptSig": "",
      "Sequence": 0
    }
  ],
  "Vout": [
    {
      "value": 100,
      "script_pub_key": "dqkUUFcMxzuxilH8QVPuxo0h0RBdMm6IrA=="
    },
    {
      "value": 200,
      "script_pub_key": "dqkUrj6W0AhljbZN1PjfLXNu28a+HDGIrA=="
    },
    {
      "value": 200,
      "script_pub_key": "dqkUzoYFZ4bjQVUw+Mxzn7QUqHQ1tLaIrA=="
    }
  ],
  "Data": null,
  "Magic": 0,
  "LockTime": 0
}`

	bytes, _ := json.MarshalIndent(tx, "", "  ")
	if string(bytes) != txStr {
		t.Fatalf("want: %s, got: %s", txStr, string(bytes))
	}
}

func hashFromUint64(n uint64) crypto.HashType {
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, n)
	return crypto.DoubleHashH(bs)
}

func TestMakeUnsignedSplitAddrTx(t *testing.T) {

	from := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o"
	addrs := []string{
		"b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ",
		"b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7",
	}
	weights := []uint64{3, 7}
	changeAmt := uint64(200)
	prevHash1 := hashFromUint64(1)
	utxoValue1, utxoValue2 := uint64(200), uint64(400)
	op, uw := types.NewOutPoint(&prevHash1, 0), NewUtxoWrap(from, 2, utxoValue1)
	utxo1 := MakePbUtxo(op, uw)
	prevHash2 := hashFromUint64(2)
	op, uw = types.NewOutPoint(&prevHash2, 0), NewUtxoWrap(from, 3, utxoValue2)
	utxo2 := MakePbUtxo(op, uw)

	tx, splitAddr, err := MakeUnsignedSplitAddrTx(from, addrs, weights, changeAmt,
		utxo1, utxo2)
	if err != nil {
		t.Fatal(err)
	}
	wantSplitAddr := "b2NRN8v4ArV7B5R77xpBBq2HKrV6xUdyUbV"
	if splitAddr != wantSplitAddr {
		t.Fatalf("aplit addr want: %s, got: %s", wantSplitAddr, splitAddr)
	}

	txStr := `{
  "Version": 0,
  "Vin": [
    {
      "PrevOutPoint": {
        "Hash": "276abb0e0c27f6a7a9b482579dd9861deccdab04b10c4f3e117549bd6b3f5308",
        "Index": 0
      },
      "ScriptSig": "",
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "18bc65f0e5c91ffef96a3e4bc923bc31a82ce58bebd05105d4074d3b7264e63f",
        "Index": 0
      },
      "ScriptSig": "",
      "Sequence": 0
    }
  ],
  "Vout": [
    {
      "script_pub_key": "ahRBIDWMx7+U00Rev3pzsNaKjE7uvhRQVwzHO7GKUfxBU+7GjSHREF0ybgEDFK4+ltAIZY22TdT43y1zbtvGvhwxAQc="
    },
    {
      "value": 200,
      "script_pub_key": "dqkUzoYFZ4bjQVUw+Mxzn7QUqHQ1tLaIrA=="
    }
  ],
  "Data": null,
  "Magic": 0,
  "LockTime": 0
}`

	bytes, _ := json.MarshalIndent(tx, "", "  ")
	if string(bytes) != txStr {
		t.Fatalf("want: %s, got: %s", txStr, string(bytes))
	}
}

func TestMakeContractTx(t *testing.T) {
	outAddr := "b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x"
	hash, idx, value := hashFromUint64(1), uint32(2), uint64(100)

	// normal vin, contract creation vout
	gas, gasPrice := uint64(200), uint64(5)
	codeStr := "6060604052346000575b60398060166000396000f30060606040525b600b5b5b565b0000a165627a7a723058209cedb722bf57a30e3eb00eeefc392103ea791a2001deed29f5c3809ff10eb1dd0029"
	code, _ := hex.DecodeString(codeStr)
	cvout, _ := MakeContractCreationVout(value, gas, gasPrice, code)
	tx := types.NewTx(0, 0x5544, 0).
		AppendVin(MakeVin(types.NewOutPoint(&hash, idx), 0)).
		AppendVout(cvout)
	//bytes, _ := json.MarshalIndent(tx, "", "  ")
	//t.Logf("contract vout tx: %s", string(bytes))
	if len(tx.Vin[0].ScriptSig) != 0 ||
		tx.Vin[0].PrevOutPoint.Hash != hash ||
		tx.Vin[0].PrevOutPoint.Index != idx ||
		tx.Vout[0].Value != value {
		t.Fatalf("contract vout tx want sig: %v, prev outpoint: %v, value: %d, got: %+v",
			tx.Vin[0].ScriptSig, tx.Vin[0].PrevOutPoint, value, tx)
	}

	// normal vin, contract call vout
	receiver := "b5WYphc4yBPH18gyFthS1bHyRcEvM6xANuT"
	codeStr = "60fe47b10000000000000000000000000000000000000000000000000000000000000006"
	code, _ = hex.DecodeString(codeStr)
	cvout, _ = MakeContractCallVout(receiver, value, gas, gasPrice, code)
	tx = types.NewTx(0, 0x5544, 0).
		AppendVin(MakeVin(types.NewOutPoint(&hash, idx), 0)).
		AppendVout(cvout)
	//bytes, _ := json.MarshalIndent(tx, "", "  ")
	//t.Logf("contract vout tx: %s", string(bytes))
	if len(tx.Vin[0].ScriptSig) != 0 ||
		tx.Vin[0].PrevOutPoint.Hash != hash ||
		tx.Vin[0].PrevOutPoint.Index != idx ||
		tx.Vout[0].Value != value {
		t.Fatalf("contract vout tx want sig: %v, prev outpoint: %v, value: %d, got: %+v",
			tx.Vin[0].ScriptSig, tx.Vin[0].PrevOutPoint, value, tx)
	}

	// contract vin, normal vout
	tx = types.NewTx(0, 0x5544, 0).
		AppendVin(MakeContractVin(types.NewOutPoint(&hash, idx), 0)).
		AppendVout(MakeVout(outAddr, value))
	//bytes, _ = json.MarshalIndent(tx, "", "  ")
	//t.Logf("contract vin tx: %s", string(bytes))
	if len(tx.Vin[0].ScriptSig) != 1 || tx.Vin[0].ScriptSig[0] != byte(script.OPCONTRACT) ||
		tx.Vin[0].PrevOutPoint.Hash != hash || tx.Vin[0].PrevOutPoint.Index != idx ||
		tx.Vout[0].Value != value {
		t.Fatalf("contract vin tx want sig: %v, prev outpoint: %v, value: %d, got: %+v",
			tx.Vin[0].ScriptSig, tx.Vin[0].PrevOutPoint, value, tx)
	}
}
