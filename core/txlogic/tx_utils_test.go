package txlogic

import (
	"testing"

	"github.com/BOXFoundation/boxd/script"
)

func TestNewIssueTokenUtxoWrap(t *testing.T) {
	addr := "b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o"
	name, sym, deci := "box token", "BOX", uint8(8)
	tag := NewTokenTag(name, sym, deci)
	uw, err := NewIssueTokenUtxoWrap(addr, tag, 1, 10000)
	if err != nil {
		t.Fatal(err)
	}
	sc := script.NewScriptFromBytes(uw.Output.ScriptPubKey)
	if !sc.IsTokenIssue() {
		t.Fatal("expect token issue script")
	}
	param, _ := sc.GetIssueParams()
	if param.Name != name || param.Symbol != sym || param.Decimals != deci {
		t.Fatalf("issue params want: %+v, got: %+v", tag, param)
	}
}
