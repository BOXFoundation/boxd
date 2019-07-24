// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"reflect"
	"testing"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
)

func TestNewIssueTokenUtxoWrap(t *testing.T) {
	addr := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o"
	name, sym, deci := "box token", "BOX", uint32(8)
	tag := NewTokenTag(name, sym, deci, 10000)
	uw, err := NewIssueTokenUtxoWrap(addr, tag, 1)
	if err != nil {
		t.Fatal(err)
	}
	sc := script.NewScriptFromBytes(uw.Script())
	if !sc.IsTokenIssue() {
		t.Fatal("expect token issue script")
	}
	param, _ := sc.GetIssueParams()
	if param.Name != name || param.Symbol != sym || param.Decimals != uint8(deci) {
		t.Fatalf("issue params want: %+v, got: %+v", tag, param)
	}
}

func TestMakeSplitAddrVout(t *testing.T) {
	addrs := []string{
		"b1YMx5kufN2qELzKaoaBWzks2MZknYqqPnh",
		"b1b1ncaV56DBPkSjhUVHePDErSESrBRUnyU",
		"b1nfRQofEAHAkayCvwAfr4EVxhZEpQWhp8N",
		"b1oKfcV9tsiBjaTT4DxwANsrPi6br76vjqc",
	}
	weights := []uint32{1, 2, 3, 4}
	addresses := make([]types.Address, len(addrs))
	for i, addr := range addrs {
		addresses[i], _ = types.ParseAddress(addr)
	}
	out := MakeSplitAddrVout(addresses, weights)

	sc := script.NewScriptFromBytes(out.ScriptPubKey)
	if !sc.IsSplitAddrScript() {
		t.Fatalf("vout should be split addr script")
	}
	as, ws, err := sc.ParseSplitAddrScript()
	if err != nil {
		t.Fatal(err)
	}
	aas := make([]string, 0)
	for _, a := range as {
		aas = append(aas, a.String())
	}
	if !reflect.DeepEqual(aas, addrs) {
		t.Fatalf("addrs want: %v, got %v", addrs, aas)
	}
	if !reflect.DeepEqual(ws, weights) {
		t.Fatalf("weights want: %v, got %v", weights, ws)
	}
}

func TestEncodeDecodeOutPoint(t *testing.T) {
	hashStr, index := "92755682dde3c99e1482391c5fb957d5415dd2083689fb55892194f8a93dab93", uint32(1)
	hash := new(crypto.HashType)
	hash.SetString(hashStr)
	op := NewPbOutPoint(hash, index)
	// encode
	encodeStr := EncodeOutPoint(op)
	wantEncodeOp := "5uhMzn4A3qdonGiEb9WpkqCFNNUX4tnCcnH3LcQX7pLqxGXzb6o"
	if encodeStr != wantEncodeOp {
		t.Fatalf("encode op, want: %s, got: %s", wantEncodeOp, encodeStr)
	}
	// decode
	op2, err := DecodeOutPoint(encodeStr)
	if err != nil {
		t.Fatal(err)
	}
	hash.SetBytes(op2.Hash)
	if hashStr != hash.String() || index != op2.Index {
		t.Fatalf("decode op, want: %s:%d, got: %s:%d", hashStr, index, hash, op2.Index)
	}
}

func TestMakeSplitAddress(t *testing.T) {

	var tests = []struct {
		addrs     []string
		weights   []uint32
		splitAddr string
	}{
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1e2dqxSPtEyZNGEEh7GBaK4pPYaZecFu3H"},
			[]uint32{5, 5},
			"b2WeyKZJ3WJjsWcxpAkM6gnm24LWzaqsoQX",
		},
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1e2dqxSPtEyZNGEEh7GBaK4pPYaZecFu3H"},
			[]uint32{3, 7},
			"b2QBrAPAySqzUSg8BTR4QGzaK5qfC2qu8hY",
		},
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1bgU3pRjrW2AXZ5DtumFJwrh69QTsErhAD"},
			[]uint32{1, 2},
			"b2KXUNtf25cz9MjDZSnVBaBVq17AUW3wHjR",
		},
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1WkepSk6YEeBwd3UxBLoQPHJ5k74c1ZGcg",
				"b1r7TPQS5MFe1wn65ZSuiy77e9fqaB2qvyt"},
			[]uint32{1, 2, 3},
			"b2SyjrVSQXjRF8gy6czR3MZj4HEakocJEQN",
		},
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1afgd4BC3Y81ni3ds2YETikEkprG9Bxo98",
				"b1r7TPQS5MFe1wn65ZSuiy77e9fqaB2qvyt"},
			[]uint32{2, 3, 4},
			"b2dz4ipBPpaUsFqFCVJqGPhCjijkvyCCN3s",
		},
		{
			[]string{"b1e4se6C2bwX3vTWHcHmT9s87kxrBGEJkEn", "b1bgU3pRjrW2AXZ5DtumFJwrh69QTsErhAD",
				"b1YVsw5ZUfPNmANEhpBezDMfg2Xvj2U2Wrk"},
			[]uint32{2, 2, 4},
			"b2HEtVD58nKXcfnJiSAabHV8ebmaSdLCP4k",
		},
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1bgU3pRjrW2AXZ5DtumFJwrh69QTsErhAD",
				"b1r7TPQS5MFe1wn65ZSuiy77e9fqaB2qvyt", "b1r7TPQS5MFe1wn65ZSuiy77e9fqaB2qvyt"},
			[]uint32{1, 2, 3},
			"b2dDT8boDTZebiDzkKXpFjqb8FiFJMGS8D4",
		},
	}
	txHash := new(crypto.HashType)
	txHash.SetString("fc7de81e29e3bcb127d63f9576ec2fffd7bd560aac2dcbe31c119799e3f51b92")
	for _, tc := range tests {
		addresses := make([]types.Address, len(tc.addrs))
		for i, addr := range tc.addrs {
			addresses[i], _ = types.ParseAddress(addr)
		}
		splitAddr := MakeSplitAddress(txHash, 0, addresses, tc.weights)
		if splitAddr.String() != tc.splitAddr {
			t.Errorf("split addr for %v want: %s, got: %s", tc.addrs, tc.splitAddr, splitAddr)
		}
	}
}
