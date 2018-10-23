// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"testing"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/facebookgo/ensure"
)

// test script not dependent on a tx
func TestNonTxScriptEvaluation(t *testing.T) {
	script := NewScript()
	script.AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPADD).AddOpCode(OP14).AddOpCode(OPEQUAL)
	err := script.Evaluate(nil, 0)
	ensure.Nil(t, err)
	script2 := NewScriptFromBytes(*script)
	ensure.DeepEqual(t, script2, script)

	script = NewScript()
	script.AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPADD).AddOpCode(OP11).AddOpCode(OPEQUAL)
	err = script.Evaluate(nil, 0)
	ensure.NotNil(t, err)

	script = NewScript()
	script.AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPADD).AddOpCode(OP11).AddOpCode(OPEQUALVERIFY)
	err = script.Evaluate(nil, 0)
	ensure.NotNil(t, err)

	script = NewScript()
	script.AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPSUB).AddOpCode(OP2).AddOpCode(OPEQUAL)
	err = script.Evaluate(nil, 0)
	ensure.Nil(t, err)

	script = NewScript()
	script.AddOpCode(OP6).AddOpCode(OPDUP).AddOpCode(OPSUB).AddOpCode(OP0).AddOpCode(OPEQUAL)
	err = script.Evaluate(nil, 0)
	ensure.Nil(t, err)
}

// test p2pkh script
func TestP2PKH(t *testing.T) {
	// TODO: to be wrapped in a helper function
	outPoint := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 0,
	}
	txIn := &types.TxIn{
		PrevOutPoint: outPoint,
		ScriptSig:    []byte{},
		Sequence:     0,
	}
	vIn := []*types.TxIn{
		txIn,
	}
	txOut := &types.TxOut{
		Value:        1,
		ScriptPubKey: []byte{},
	}
	vOut := []*types.TxOut{txOut}
	tx := &types.Transaction{
		Version:  1,
		Vin:      vIn,
		Vout:     vOut,
		Magic:    1,
		LockTime: 0,
	}

	privKey, pubKey, _ := crypto.NewKeyPair()
	pubKeyStr := pubKey.Serialize()
	pubKeyHash := crypto.Hash160(pubKeyStr)

	scriptPubKey := NewScript().AddOpCode(OPDUP).AddOpCode(OPHASH160).AddOperand(pubKeyHash).AddOpCode(OPEQUALVERIFY).AddOpCode(OPCHECKSIG)
	hash, _ := CalcTxHashForSig([]byte(*scriptPubKey), tx, 0)
	sig, _ := crypto.Sign(privKey, hash)
	sigStr := sig.Serialize()

	// P2PKH script: sig, pubKey, OPCODESEPARATOR, OPDUP, OPHASH160, pubKeyHash, OPEQUALVERIFY, OPCHECKSIG
	script := NewScript()
	script.AddOperand(sigStr).AddOperand(pubKeyStr).AddOpCode(OPCODESEPARATOR).AddScript(scriptPubKey)
	err := script.Evaluate(tx, 0)
	ensure.Nil(t, err)
}
