// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"testing"

	"github.com/BOXFoundation/boxd/crypto"
	"github.com/facebookgo/ensure"
)

func TestTxOutConvertWithProtoMessage(t *testing.T) {
	var value int64 = 123456
	txOut := NewTxOut(value)
	var txOut1 = &TxOut{}
	msg, err := txOut.ToProtoMessage()
	ensure.Nil(t, err)
	err = txOut1.FromProtoMessage(msg)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, txOut, txOut1)
}

func TestTxInConvertWithProtoMessage(t *testing.T) {
	var prevOutPoint = NewOutPoint(crypto.HashType{0x0010})
	txIn := NewTxIn(*prevOutPoint)
	var txIn1 = &TxIn{}
	msg, err := txIn.ToProtoMessage()
	ensure.Nil(t, err)
	err = txIn1.FromProtoMessage(msg)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, txIn, txIn1)
}

func TestOutPointConvertWithProtoMessage(t *testing.T) {
	outPoint := NewOutPoint(crypto.HashType{0x0011})
	var outPoint1 = &OutPoint{}
	msg, err := outPoint.ToProtoMessage()
	ensure.Nil(t, err)
	err = outPoint1.FromProtoMessage(msg)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, outPoint, outPoint1)
}

func TestTxConvertWithProtoMessage(t *testing.T) {
	var prevOutPoint = NewOutPoint(crypto.HashType{0x0012})
	var value int64 = 111222
	var lockTime int64 = 12345678900000000
	tx := NewTransaction(*prevOutPoint, value, lockTime)
	tx1 := &Transaction{}
	msg, err := tx.ToProtoMessage()
	ensure.Nil(t, err)
	err = tx1.FromProtoMessage(msg)
	ensure.Nil(t, err)
	tx.Hash, _ = calcProtoMsgDoubleHash(msg)
	ensure.DeepEqual(t, tx, tx1)
}
