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

	privKey, pubKey, _ = crypto.NewKeyPair()
	pubKeyBytes        = pubKey.Serialize()
	pubKeyHash         = crypto.Hash160(pubKeyBytes)
)

// test script not dependent on a tx
func TestNonTxScriptEvaluation(t *testing.T) {
	script := NewScript().AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPADD).AddOpCode(OP14).AddOpCode(OPEQUAL)
	err := script.evaluate(nil, 0)
	ensure.Nil(t, err)
	script2 := NewScriptFromBytes(*script)
	ensure.DeepEqual(t, script2, script)

	script = NewScript().AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPADD).AddOpCode(OP11).AddOpCode(OPEQUAL)
	err = script.evaluate(nil, 0)
	ensure.NotNil(t, err)

	script = NewScript().AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPADD).AddOpCode(OP11).AddOpCode(OPEQUALVERIFY)
	err = script.evaluate(nil, 0)
	ensure.NotNil(t, err)

	script = NewScript().AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPSUB).AddOpCode(OP2).AddOpCode(OPEQUAL)
	err = script.evaluate(nil, 0)
	ensure.Nil(t, err)

	script = NewScript().AddOpCode(OP6).AddOpCode(OPDUP).AddOpCode(OPSUB).AddOpCode(OP0).AddOpCode(OPEQUAL)
	err = script.evaluate(nil, 0)
	ensure.Nil(t, err)
}

func genP2PKHScript() (*Script, *Script, []byte) {
	// locking script: OPDUP, OPHASH160, pubKeyHash, OPEQUALVERIFY, OPCHECKSIG
	scriptPubKey := NewScript().AddOpCode(OPDUP).AddOpCode(OPHASH160).AddOperand(pubKeyHash).AddOpCode(OPEQUALVERIFY).AddOpCode(OPCHECKSIG)

	hash, _ := CalcTxHashForSig([]byte(*scriptPubKey), tx, 0)
	sig, _ := crypto.Sign(privKey, hash)
	sigBytes := sig.Serialize()
	// unlocking script: sig, pubKey
	scriptSig := NewScript().AddOperand(sigBytes).AddOperand(pubKeyBytes)

	return scriptSig, scriptPubKey, sigBytes
}

// test p2pkh script
func TestP2PKH(t *testing.T) {
	scriptSig, scriptPubKey, _ := genP2PKHScript()
	err := Validate(scriptSig, scriptPubKey, tx, 0)
	ensure.Nil(t, err)
}

func genP2SHScript() (*Script, *Script) {
	// redeem script
	redeemScript := NewScript().AddOperand(pubKeyBytes).AddOpCode(OPCHECKSIG)
	redeemScriptHash := crypto.Hash160(*redeemScript)

	// locking script
	scriptPubKey := NewScript().AddOpCode(OPHASH160).AddOperand(redeemScriptHash).AddOpCode(OPEQUAL)

	// Note: use redeemScript, not scriptPubKey, because the former is checked against signature with OP_CHECKSIG
	hash, _ := CalcTxHashForSig([]byte(*redeemScript), tx, 0)
	sig, _ := crypto.Sign(privKey, hash)
	sigBytes := sig.Serialize()
	// unlocking script: signature <redeemScript>
	// Note: <redeemScript> is serialized, i.e., AddOperand not AddScript
	scriptSig := NewScript().AddOperand(sigBytes).AddOperand(*redeemScript)

	return scriptSig, scriptPubKey
}

// test p2pkh script
func TestP2SH(t *testing.T) {
	scriptSig, scriptPubKey := genP2SHScript()
	err := Validate(scriptSig, scriptPubKey, tx, 0)
	ensure.Nil(t, err)
}

func TestDisasm(t *testing.T) {
	script := NewScript().AddOpCode(OP8).AddOpCode(OP6).AddOpCode(OPADD).AddOpCode(OP14).AddOpCode(OPEQUAL)
	ensure.DeepEqual(t, script.disasm(), "OP_8 OP_6 OP_ADD OP_14 OP_EQUAL")

	// not enough data to push
	script.AddOpCode(OPPUSHDATA1)
	ensure.DeepEqual(t, script.disasm(), "OP_8 OP_6 OP_ADD OP_14 OP_EQUAL [Error: OP_PUSHDATA1 has not enough data]")

	scriptSig, scriptPubKey, sigBytes := genP2PKHScript()
	expectedScriptStrs := []string{hex.EncodeToString(sigBytes), hex.EncodeToString(pubKeyBytes), "OP_CODESEPARATOR",
		"OP_DUP", "OP_HASH160", hex.EncodeToString(pubKeyHash), "OP_EQUALVERIFY", "OP_CHECKSIG"}
	catScript := NewScript().AddScript(scriptSig).AddOpCode(OPCODESEPARATOR).AddScript(scriptPubKey)
	ensure.DeepEqual(t, catScript.disasm(), strings.Join(expectedScriptStrs, " "))
}

func TestIsPayToScriptHash(t *testing.T) {
	p2SHScriptBytes := []byte{
		byte(OPHASH160),
		0x14,                         // 160-bit redeemp script hash length: 20 bytes
		0x00, 0x01, 0x02, 0x03, 0x04, // 160-bit redeemp script hash: begining
		0x05, 0x06, 0x07, 0x08, 0x09,
		0x0A, 0x0B, 0x0C, 0x0D, 0x0E,
		0x0F, 0x10, 0x11, 0x12, 0x13, // 160-bit redeemp script hash: end
		byte(OPEQUAL),
	}
	p2SHScript := NewScriptFromBytes(p2SHScriptBytes)
	ensure.True(t, p2SHScript.IsPayToScriptHash())

	p2SHScriptBytes[0] = byte(OPHASH256)
	p2SHScript = NewScriptFromBytes(p2SHScriptBytes)
	ensure.False(t, p2SHScript.IsPayToScriptHash())
	// recover
	p2SHScriptBytes[0] = byte(OPHASH160)

	p2SHScriptBytes[len(p2SHScriptBytes)-1] = byte(OPEQUALVERIFY)
	p2SHScript = NewScriptFromBytes(p2SHScriptBytes)
	ensure.False(t, p2SHScript.IsPayToScriptHash())
	// recover
	p2SHScriptBytes[len(p2SHScriptBytes)-1] = byte(OPEQUAL)

	p2SHScriptBytes[1] = 0x15
	p2SHScript = NewScriptFromBytes(p2SHScriptBytes)
	ensure.False(t, p2SHScript.IsPayToScriptHash())
	// recover
	p2SHScriptBytes[1] = 0x14

	p2SHScriptBytes = append(p2SHScriptBytes[:5], p2SHScriptBytes[6:]...)
	p2SHScript = NewScriptFromBytes(p2SHScriptBytes)
	ensure.False(t, p2SHScript.IsPayToScriptHash())
}
