// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"reflect"
	"testing"

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
