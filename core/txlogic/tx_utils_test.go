// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"reflect"
	"testing"

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
	weights := []uint64{1, 2, 3, 4}
	out := MakeSplitAddrVout(addrs, weights)

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
		weights   []uint64
		splitAddr string
	}{
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1e2dqxSPtEyZNGEEh7GBaK4pPYaZecFu3H"},
			[]uint64{5, 5},
			"b2f5cgAGvk5BTL35AUawcfCVDmVpzECpqt1",
		},
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1e2dqxSPtEyZNGEEh7GBaK4pPYaZecFu3H"},
			[]uint64{3, 7},
			"b2MTibyHLXk8CmmKrw8dJzYSemZwjxHtF67",
		},
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1bgU3pRjrW2AXZ5DtumFJwrh69QTsErhAD"},
			[]uint64{1, 2},
			"b2e94cGe8fk5VsaDxpLv7pxrK56XC5gGnhu",
		},
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1WkepSk6YEeBwd3UxBLoQPHJ5k74c1ZGcg",
				"b1r7TPQS5MFe1wn65ZSuiy77e9fqaB2qvyt"},
			[]uint64{1, 2, 3},
			"b2Nq13XAxsWPaTWko82ErntqHkHUbh4nvVg",
		},
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1afgd4BC3Y81ni3ds2YETikEkprG9Bxo98",
				"b1r7TPQS5MFe1wn65ZSuiy77e9fqaB2qvyt"},
			[]uint64{2, 3, 4},
			"b2RFDWeqUbgYE4Fqp2NumLqF7EC3ms9LReV",
		},
		{
			[]string{"b1e4se6C2bwX3vTWHcHmT9s87kxrBGEJkEn", "b1bgU3pRjrW2AXZ5DtumFJwrh69QTsErhAD",
				"b1YVsw5ZUfPNmANEhpBezDMfg2Xvj2U2Wrk"},
			[]uint64{2, 2, 4},
			"b2bE1z7CB2jEAkDRSEfAw8PNnAmQQfDMcft",
		},
		{
			[]string{"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x", "b1bgU3pRjrW2AXZ5DtumFJwrh69QTsErhAD",
				"b1r7TPQS5MFe1wn65ZSuiy77e9fqaB2qvyt", "b1r7TPQS5MFe1wn65ZSuiy77e9fqaB2qvyt"},
			[]uint64{1, 2, 3},
			"b2b38PF4WDYgGMDWBh7mGUFmft49qYVq9ks",
		},
	}
	for _, tc := range tests {
		splitAddr, err := MakeSplitAddr(tc.addrs, tc.weights)
		if err != nil {
			t.Fatal(err)
		}
		if splitAddr != tc.splitAddr {
			t.Errorf("split addr for %v want: %s, got: %s", tc.addrs, tc.splitAddr, splitAddr)
		}
	}
}
