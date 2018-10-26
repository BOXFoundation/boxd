// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/facebookgo/ensure"
)

var (
	// TODO: to be wrapped in a helper function
	outPoint = types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 0,
	}
	txIn = &types.TxIn{
		PrevOutPoint: outPoint,
		ScriptSig:    []byte{},
		Sequence:     0,
	}
	vIn = []*types.TxIn{
		txIn,
	}
	txOut = &corepb.TxOut{
		Value:        1,
		ScriptPubKey: []byte{},
	}
	vOut = []*corepb.TxOut{txOut}
	tx   = &types.Transaction{
		Version:  1,
		Vin:      vIn,
		Vout:     vOut,
		Magic:    1,
		LockTime: 0,
	}
)

// test script not dependent on a tx
func TestNonTxScriptEvaluation(t *testing.T) {
	script := NewScript().AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPADD).AddOpCode(OP14).AddOpCode(OPEQUAL)
	err := script.Evaluate(nil, 0)
	ensure.Nil(t, err)
	script2 := NewScriptFromBytes(*script)
	ensure.DeepEqual(t, script2, script)

	script = NewScript().AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPADD).AddOpCode(OP11).AddOpCode(OPEQUAL)
	err = script.Evaluate(nil, 0)
	ensure.NotNil(t, err)

	script = NewScript().AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPADD).AddOpCode(OP11).AddOpCode(OPEQUALVERIFY)
	err = script.Evaluate(nil, 0)
	ensure.NotNil(t, err)

	script = NewScript().AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPSUB).AddOpCode(OP2).AddOpCode(OPEQUAL)
	err = script.Evaluate(nil, 0)
	ensure.Nil(t, err)

	script = NewScript().AddOpCode(OP6).AddOpCode(OPDUP).AddOpCode(OPSUB).AddOpCode(OP0).AddOpCode(OPEQUAL)
	err = script.Evaluate(nil, 0)
	ensure.Nil(t, err)
}

func genP2PKHScript() (*Script, []byte, []byte, []byte) {
	privKey, pubKey, _ := crypto.NewKeyPair()
	pubKeyBytes := pubKey.Serialize()
	pubKeyHash := crypto.Hash160(pubKeyBytes)

	// locking script: OPDUP, OPHASH160, pubKeyHash, OPEQUALVERIFY, OPCHECKSIG
	scriptPubKey := NewScript().AddOpCode(OPDUP).AddOpCode(OPHASH160).AddOperand(pubKeyHash).AddOpCode(OPEQUALVERIFY).AddOpCode(OPCHECKSIG)
	hash, _ := CalcTxHashForSig([]byte(*scriptPubKey), tx, 0)
	sig, _ := crypto.Sign(privKey, hash)
	sigBytes := sig.Serialize()

	// P2PKH script: sig, pubKey, OPCODESEPARATOR, scriptPubKey
	return NewScript().AddOperand(sigBytes).AddOperand(pubKeyBytes).AddOpCode(OPCODESEPARATOR).AddScript(scriptPubKey),
		sigBytes, pubKeyBytes, pubKeyHash
}

// test p2pkh script
func TestP2PKH(t *testing.T) {
	script, _, _, _ := genP2PKHScript()
	err := script.Evaluate(tx, 0)
	ensure.Nil(t, err)
}

func TestDisasm(t *testing.T) {
	script := NewScript().AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPADD).AddOpCode(OP14).AddOpCode(OPEQUAL)
	ensure.DeepEqual(t, script.disasm(), "OP_8 OP_6 OP_ADD OP_14 OP_EQUAL")

	// not enough data to push
	script.AddOpCode(OPPUSHDATA1)
	ensure.DeepEqual(t, script.disasm(), "OP_8 OP_6 OP_ADD OP_14 OP_EQUAL [Error: OP_PUSHDATA1 has not enough data]")

	script, sigBytes, pubKeyBytes, pubKeyHash := genP2PKHScript()
	expectedScriptStrs := []string{hex.EncodeToString(sigBytes), hex.EncodeToString(pubKeyBytes), "OP_CODESEPARATOR",
		"OP_DUP", "OP_HASH160", hex.EncodeToString(pubKeyHash), "OP_EQUALVERIFY", "OP_CHECKSIG"}
	ensure.DeepEqual(t, script.disasm(), strings.Join(expectedScriptStrs, " "))
}
