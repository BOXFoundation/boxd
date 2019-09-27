// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/BOXFoundation/boxd/crypto"
	"github.com/facebookgo/ensure"
)

func TestTxInConvertWithProtoMessage(t *testing.T) {
	prevOutPoint := NewOutPoint(&crypto.HashType{0x0010}, 0)
	txIn := NewTxIn(prevOutPoint, nil, 0)
	var txIn1 = &TxIn{}
	msg, err := txIn.ToProtoMessage()
	ensure.Nil(t, err)
	err = txIn1.FromProtoMessage(msg)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, txIn, txIn1)
}

func TestOutPointConvertWithProtoMessage(t *testing.T) {
	outPoint := NewOutPoint(&crypto.HashType{0x0011}, 0)
	var outPoint1 = &OutPoint{}
	msg, err := outPoint.ToProtoMessage()
	ensure.Nil(t, err)
	err = outPoint1.FromProtoMessage(msg)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, outPoint, outPoint1)
}

func TestTxConvertWithProtoMessage(t *testing.T) {
	var prevOutPoint = NewOutPoint(&crypto.HashType{0x0012}, 0)
	var value uint64 = 111222
	var lockTime int64 = 12345678900000000
	tx := NewTransaction(*prevOutPoint, value, lockTime)
	tx1 := &Transaction{}
	msg, err := tx.ToProtoMessage()
	ensure.Nil(t, err)
	err = tx1.FromProtoMessage(msg)
	ensure.Nil(t, err)
	tx.hash, _ = calcProtoMsgDoubleHash(msg)
	ensure.DeepEqual(t, tx, tx1)
}

func TestTxMashal(t *testing.T) {
	hash, idx, value := crypto.DoubleHashH([]byte{12, 23, 45}), uint32(2), uint64(100)
	codeStr := "6060604052346000575b60398060166000396000f30060606040525b600b5b5b5" +
		"65b0000a165627a7a723058209cedb722bf57a30e3eb00eeefc392103ea791a2001deed29f" +
		"5c3809ff10eb1dd0029"
	spk, _ := hex.DecodeString("76a9147612d02534bbd20bf32c414858b5fc190924c8a388ac")
	ssg, err := hex.DecodeString("473045022100fdd77e7a42cf037162482317cc577eb1fe7" +
		"157709857e683e6efde34894a45c702200518543955a224b8a9b8a9ab9d7a501fbca2f92f8" +
		"ec65a4c3c0d4fa155a705702102735de561cf489c82efb2ad27aae2011bb55b44f4b49b831" +
		"397d3c100fe41429f")
	if err != nil {
		t.Fatal(err)
	}
	tx := NewTx(0, 0x5544, 0).
		AppendVin(NewTxIn(NewOutPoint(&hash, idx), ssg, 100)).
		AppendVout(NewTxOut(value, spk))
	content, _ := hex.DecodeString(codeStr)
	tx.Data = NewData(ContractDataType, content)
	bytes, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "Version": 0,
  "Vin": [
    {
      "PrevOutPoint": {
        "Hash": "f9bea81d88b81db5a32099bdb938c3202e1b5db63a0068ee868a43c35d7d125e",
        "Index": 2
      },
      "Sequence": 100
    }
  ],
  "Vout": [
    {
      "Value": 100,
      "ScriptPubKey": "76a9147612d02534bbd20bf32c414858b5fc190924c8a388ac"
    }
  ],
  "Data": {
    "Type": 1,
    "Content": "6060604052346000575b60398060166000396000f30060606040525b600b5b5b565b0000a165627a7a723058209cedb722bf57a30e3eb00eeefc392103ea791a2001deed29f5c3809ff10eb1dd0029"
  },
  "Magic": 21828,
  "LockTime": 0
}`
	if want != string(bytes) {
		t.Fatalf("want: %s, got: %s", want, string(bytes))
	}
	mdata, err := tx.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("marshal size: %d, rough size: %d", len(mdata), tx.RoughSize())
	// marshal size: 271, rough size: 278

	// common pay to pubkey tx
	tx = NewTx(0, 0x5544, 0).
		AppendVin(NewTxIn(NewOutPoint(&hash, idx), ssg, 0)).
		AppendVout(NewTxOut(5599558, spk)).
		AppendVout(NewTxOut(100, spk))
	mdata, err = tx.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("marshal size: %d, rough size: %d", len(mdata), tx.RoughSize())
	// marshal size: 218, rough size: 228
}
