// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"testing"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
)

func TestMakeUnsignedTx(t *testing.T) {

	fromAddr, _ := types.NewAddress("b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o")
	from := fromAddr.Hash160()
	toAddr1, _ := types.NewAddress("b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ")
	toAddr2, _ := types.NewAddress("b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7")
	to := []*types.AddressHash{toAddr1.Hash160(), toAddr2.Hash160()}
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
  "Type": 1,
  "Version": 0,
  "Vin": [
    {
      "PrevOutPoint": {
        "Hash": "276abb0e0c27f6a7a9b482579dd9861deccdab04b10c4f3e117549bd6b3f5308",
        "Index": 0
      },
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "18bc65f0e5c91ffef96a3e4bc923bc31a82ce58bebd05105d4074d3b7264e63f",
        "Index": 0
      },
      "Sequence": 0
    }
  ],
  "Vout": [
    {
      "Value": 100,
      "ScriptPubKey": "76a91450570cc73bb18a51fc4153eec68d21d1105d326e88ac"
    },
    {
      "Value": 200,
      "ScriptPubKey": "76a914ae3e96d008658db64dd4f8df2d736edbc6be1c3188ac"
    },
    {
      "Value": 200,
      "ScriptPubKey": "76a914ce86056786e3415530f8cc739fb414a87435b4b688ac"
    }
  ],
  "Data": null,
  "Magic": 0,
  "LockTime": 0
}`

	txType := GetTxType(tx, nil)
	if txType != types.PayToPubkTx {
		t.Fatalf("want tx type is %d, got: %d", types.PayToPubkTx, txType)
	}
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
	fromAddr, _ := types.NewAddress("b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o")
	from := fromAddr.Hash160()
	toAddr1, _ := types.ParseAddress("b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ")
	toAddr2, _ := types.ParseAddress("b1jh8DSdB6kB7N7RanrudV1hzzMCCcoX6L7")
	to := []*types.AddressHash{toAddr1.Hash160(), toAddr2.Hash160()}
	weights := []uint32{3, 7}
	changeAmt := uint64(200)
	prevHash1 := hashFromUint64(1)
	utxoValue1, utxoValue2 := uint64(200), uint64(400)
	op, uw := types.NewOutPoint(&prevHash1, 0), NewUtxoWrap(from, 2, utxoValue1)
	utxo1 := MakePbUtxo(op, uw)
	prevHash2 := hashFromUint64(2)
	op, uw = types.NewOutPoint(&prevHash2, 0), NewUtxoWrap(from, 3, utxoValue2)
	utxo2 := MakePbUtxo(op, uw)

	tx, err := MakeUnsignedSplitAddrTx(from, to, weights, changeAmt, utxo1, utxo2)
	if err != nil {
		t.Fatal(err)
	}
	txHash, _ := tx.TxHash()
	splitAddr := MakeSplitAddress(txHash, 0, to, weights)
	wantSplitAddr := "b2NZVEAZuT6maygpyNs9BfTS7GTr9oRgvBE"
	if splitAddr.String() != wantSplitAddr {
		t.Fatalf("aplit addr want: %s, got: %s", wantSplitAddr, splitAddr)
	}

	txStr := `{
  "Type": 8,
  "Version": 0,
  "Vin": [
    {
      "PrevOutPoint": {
        "Hash": "276abb0e0c27f6a7a9b482579dd9861deccdab04b10c4f3e117549bd6b3f5308",
        "Index": 0
      },
      "Sequence": 0
    },
    {
      "PrevOutPoint": {
        "Hash": "18bc65f0e5c91ffef96a3e4bc923bc31a82ce58bebd05105d4074d3b7264e63f",
        "Index": 0
      },
      "Sequence": 0
    }
  ],
  "Vout": [
    {
      "Value": 0,
      "ScriptPubKey": "6a0014ac8d67f0c8ed88f2d85e99009b1ab521cfc0f9551450570cc73bb18a51fc4153eec68d21d1105d326e040300000014ae3e96d008658db64dd4f8df2d736edbc6be1c310407000000"
    },
    {
      "Value": 200,
      "ScriptPubKey": "76a914ce86056786e3415530f8cc739fb414a87435b4b688ac"
    }
  ],
  "Data": null,
  "Magic": 0,
  "LockTime": 0
}`

	txType := GetTxType(tx, nil)
	if txType != types.SplitTx {
		t.Fatalf("want tx type is %d, got: %d", types.SplitTx, txType)
	}
	bytes, _ := json.MarshalIndent(tx, "", "  ")
	if string(bytes) != txStr {
		t.Fatalf("want: %s, got: %s", txStr, string(bytes))
	}
}

func TestMakeContractTx(t *testing.T) {
	fromAddr := "b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x"
	from, _ := types.NewAddress(fromAddr)
	hash, idx, value := hashFromUint64(1), uint32(2), uint64(100)

	// normal vin, contract creation vout
	gas, nonce := uint64(200), uint64(1)
	codeStr := "6060604052346000575b60398060166000396000f30060606040525b600b5b5b5" +
		"65b0000a165627a7a723058209cedb722bf57a30e3eb00eeefc392103ea791a2001deed29f" +
		"5c3809ff10eb1dd0029"
	code, _ := hex.DecodeString(codeStr)
	cvout, _ := MakeContractCreationVout(from.Hash160(), value, gas, nonce)
	tx := types.NewTx(0, 0x5544, 0).
		AppendVin(MakeVin(types.NewOutPoint(&hash, idx), 0)).
		AppendVout(cvout).
		WithData(types.ContractDataType, code)
	GetTxType(tx, nil)
	if len(tx.Vin[0].ScriptSig) != 0 ||
		tx.Vin[0].PrevOutPoint.Hash != hash ||
		tx.Vin[0].PrevOutPoint.Index != idx ||
		tx.Vout[0].Value != value ||
		tx.Type != types.ContractTx {
		t.Fatalf("contract vout tx want sig: %v, prev outpoint: %v, value: %d, got: %+v",
			tx.Vin[0].ScriptSig, tx.Vin[0].PrevOutPoint, value, tx)
	}

	// normal vin, contract call vout
	toAddr := "b5WYphc4yBPH18gyFthS1bHyRcEvM6xANuT"
	to, _ := types.NewContractAddress(toAddr)
	codeStr = "60fe47b10000000000000000000000000000000000000000000000000000000000000006"
	code, _ = hex.DecodeString(codeStr)
	cvout, _ = MakeContractCallVout(from.Hash160(), to.Hash160(), value, gas, nonce)
	tx = types.NewTx(0, 0x5544, 0).
		AppendVin(MakeVin(types.NewOutPoint(&hash, idx), 0)).
		AppendVout(cvout).
		WithData(types.ContractDataType, code)
	GetTxType(tx, nil)
	if len(tx.Vin[0].ScriptSig) != 0 ||
		tx.Vin[0].PrevOutPoint.Hash != hash ||
		tx.Vin[0].PrevOutPoint.Index != idx ||
		tx.Vout[0].Value != value ||
		tx.Type != types.ContractTx {
		t.Fatalf("contract vout tx want sig: %v, prev outpoint: %v, value: %d, got: %+v",
			tx.Vin[0].ScriptSig, tx.Vin[0].PrevOutPoint, value, tx)
	}

	// contract vin, normal vout
	tx = types.NewTx(0, 0x5544, 0).
		AppendVin(MakeContractVin(types.NewOutPoint(&hash, idx), 1, 0)).
		AppendVout(MakeVout(from.Hash160(), value))
	if len(tx.Vin[0].ScriptSig) != 10 ||
		tx.Vin[0].ScriptSig[0] != byte(script.OPCONTRACT) ||
		tx.Vin[0].PrevOutPoint.Hash != hash ||
		tx.Vin[0].PrevOutPoint.Index != idx ||
		tx.Vout[0].Value != value {
		t.Fatalf("contract vin tx want sig: %v, prev outpoint: %v, value: %d, got: %+v",
			tx.Vin[0].ScriptSig, tx.Vin[0].PrevOutPoint, value, tx)
	}
}

func TestMakeUnsignedTokenTx(t *testing.T) {
	issuer := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o"
	issuerAddr, _ := types.NewAddress(issuer)
	owner := "b1b8bzyci5VYUJVKRU2HRMMQiUXnoULkKAJ"
	ownerAddr, _ := types.NewAddress(owner)
	tag := NewTokenTag("abc token", "ABC", 6, 10000)
	changeAmt := uint64(123000)
	// gen utxo
	prevHash := hashFromUint64(1)
	op := types.NewOutPoint(&prevHash, 0)
	utxoValue := core.TransferFee + changeAmt
	uw := NewUtxoWrap(issuerAddr.Hash160(), 2, utxoValue)
	utxo := MakePbUtxo(op, uw)
	// token issue
	tx, index, err := MakeUnsignedTokenIssueTx(issuerAddr.Hash160(),
		ownerAddr.Hash160(), tag, changeAmt, utxo)
	if err != nil {
		t.Fatal(err)
	}
	GetTxType(tx, nil)
	sc := script.NewScriptFromBytes(tx.Vout[0].ScriptPubKey)
	param, err := sc.GetIssueParams()
	if err != nil {
		t.Fatal(err)
	}
	if param.Name != "abc token" || param.Symbol != "ABC" ||
		param.Decimals != 6 || param.TotalSupply != 10000 {
		t.Fatalf("issue token info: abc token, ABC, 6, 1000, got info: %v", param)
	}
	sc = script.NewScriptFromBytes(tx.Vout[1].ScriptPubKey)
	addr, err := sc.ExtractAddress()
	if err != nil {
		t.Fatal(err)
	}
	if addr.String() != issuer {
		t.Fatalf("change owner want: %s, got: %s", addr, issuer)
	}
	if index != 0 ||
		tx.Type != types.TokenIssueTx ||
		tx.Vout[1].Value != changeAmt {
		t.Fatalf("want tx type: %d, vout[1] value: %d, got: %d, %d",
			types.TokenIssueTx, changeAmt, tx.Type, tx.Vout[1].Value)
	}
	// token transfer
	issueTxHash, _ := tx.TxHash()
	to := "b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x"
	toAddr, _ := types.NewAddress(to)
	toAmt := []uint64{80000}
	tid := NewTokenID(issueTxHash, index)
	// gen token utxo
	op = types.NewOutPoint(issueTxHash, 0)
	uw = NewIssueTokenUtxoWrap(ownerAddr.Hash160(), tag, 2)
	tokenUtxo := MakePbUtxo(op, uw)
	tx, tokenRemain, err := MakeUnsignedTokenTransferTx(ownerAddr.Hash160(),
		[]*types.AddressHash{toAddr.Hash160()}, toAmt, tid, changeAmt, utxo, tokenUtxo)
	if err != nil {
		t.Fatal(err)
	}
	expectRemain := tag.Supply*uint64(math.Pow10(int(tag.Decimal))) - toAmt[0]
	if tokenRemain != expectRemain {
		t.Fatalf("token remain want: %d, got: %d", expectRemain, tokenRemain)
	}
	// vout 0
	sc = script.NewScriptFromBytes(tx.Vout[0].ScriptPubKey)
	transferParam, _ := sc.GetTransferParams()
	if transferParam.TokenID.OutPoint != types.OutPoint(*tid) ||
		transferParam.Amount != toAmt[0] {
		t.Fatalf("token transfer param want: %v, %d, got: %v, %d",
			tid, tokenRemain, transferParam.TokenID, transferParam.Amount)
	}
	// vout 1
	sc = script.NewScriptFromBytes(tx.Vout[1].ScriptPubKey)
	transferParam, err = sc.GetTransferParams()
	if err != nil {
		t.Fatal(err)
	}
	if transferParam.TokenID.OutPoint != types.OutPoint(*tid) ||
		transferParam.Amount != tokenRemain {
		t.Fatalf("token transfer param want: %v, %d, got: %v, %d",
			tid, tokenRemain, transferParam.TokenID, transferParam.Amount)
	}
	// vout 2
	if tx.Vout[2].Value != changeAmt {
		t.Fatalf("change amount want: %d, got: %d", changeAmt, tx.Vout[2].Value)
	}
	// tx type
	GetTxType(tx, nil)
	if tx.Type != types.TokenTransferTx {
		t.Fatalf("tx type want: %d, got: %d", types.TokenTransferTx, tx.Type)
	}
}
