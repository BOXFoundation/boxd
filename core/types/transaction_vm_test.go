// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"testing"

	"github.com/BOXFoundation/boxd/crypto"
)

func TestReceiptsCodeDec(t *testing.T) {
	var tests = []struct {
		txHashStr, blockHashStr, contractAddrStr string
		ti, bh                                   uint32
		gasUsed                                  uint64
		failed                                   bool
	}{
		{
			"8b7ffbaad0576ea921c7789135a96c1c1a5c82c7de1c3df11bcc7046a47000ef",
			"03e42d0ebbba0dc5d6b58ebb691e3ea789920ec6ce39fd48a542fa41d5f73262",
			"b5WYphc4yBPH18gyFthS1bHyRcEvM6xANuT",
			1, 10, 2300, true,
		},
		{
			"1111111111111111111111111111111111111111111111111111111111111111",
			"22222222222222222222222222222222222222222222",
			"b5nKQMQZXDuZqiFcbZ4bvrw2GoJkgTvcMod",
			0, 12300, 420000, true,
		},
		{
			"3333333333333333333333333333333333333333333333333333333333333333",
			"4444444444444444444444444444444444444444444444444444444444444444",
			"b5WYphc4yBPH18gyFthS1bHyRcEvM6xANuT",
			4, 86000, 49000, false,
		},
	}
	var (
		txHash       = new(crypto.HashType)
		blockHash    = new(crypto.HashType)
		contractAddr *AddressHash
		err          error
	)
	// test receipt serialization deserialization
	txHash.SetString(tests[0].txHashStr)
	blockHash.SetString(tests[0].blockHashStr)
	contractAddress, _ := NewContractAddress(tests[0].contractAddrStr)
	contractAddr = contractAddress.Hash160()
	rc := NewReceipt(txHash, contractAddr, tests[0].failed, tests[0].gasUsed).
		WithTxIndex(tests[0].ti).
		WithBlockHash(blockHash).
		WithBlockHeight(tests[0].bh)
	data, err := rc.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	recRc := new(Receipt)
	if err := recRc.Unmarshal(data); err != nil {
		t.Fatal(err)
	}
	if recRc.Failed != tests[0].failed ||
		recRc.TxIndex != tests[0].ti ||
		recRc.GasUsed != tests[0].gasUsed {
		t.Fatalf("want: %+v, got: %+v", tests[0], recRc)
	}

	// test receipts serialization deserialization
	rcs := make(Receipts, 0, len(tests))
	for _, v := range tests {
		txHash.SetString(v.txHashStr)
		blockHash.SetString(v.blockHashStr)
		contractAddress, _ = NewContractAddress(v.contractAddrStr)
		contractAddr = contractAddress.Hash160()
		rc := NewReceipt(txHash, contractAddr, v.failed, v.gasUsed).WithTxIndex(v.ti)
		rcs.Append(rc)
	}
	data, err = rcs.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	recRcs := new(Receipts)
	if err := recRcs.Unmarshal(data); err != nil {
		t.Fatal(err)
	}
	vRcs := *recRcs
	for i := 0; i < len(tests); i++ {
		txHash.SetString(tests[i].txHashStr)
		blockHash.SetString(tests[i].blockHashStr)
		contractAddress, _ = NewContractAddress(tests[i].contractAddrStr)
		contractAddr = contractAddress.Hash160()
		if vRcs[i].Failed != tests[i].failed ||
			vRcs[i].TxIndex != tests[i].ti ||
			vRcs[i].GasUsed != tests[i].gasUsed {
			t.Fatalf("want: %+v, got: %+v", tests[i], vRcs[i])
		}
	}
}
