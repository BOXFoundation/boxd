// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"reflect"
	"testing"

	"github.com/BOXFoundation/Quicksilver/core/types"
	"github.com/BOXFoundation/Quicksilver/crypto"
)

func TestUtxoSet_FindUtxo(t *testing.T) {
	key := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 1,
	}
	value := &(UtxoEntry{
		output: types.TxOut{
			Value:        100000000,
			ScriptPubKey: []byte{},
		},
		BlockHeight: 10000,
		IsCoinBase:  false,
		IsSpent:     false,
	})
	type fields struct {
		utxoMap map[types.OutPoint]*UtxoEntry
	}
	type args struct {
		outPoint types.OutPoint
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *UtxoEntry
	}{
		{
			"tests",
			fields{
				map[types.OutPoint]*UtxoEntry{
					key: value,
				},
			},
			args{
				outPoint: types.OutPoint{
					Hash:  crypto.HashType{0x0010},
					Index: 1,
				},
			},
			&UtxoEntry{
				output: types.TxOut{
					Value:        100000000,
					ScriptPubKey: []byte{},
				},
				BlockHeight: 10000,
				IsCoinBase:  false,
				IsSpent:     false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UtxoSet{
				utxoMap: tt.fields.utxoMap,
			}
			if got := u.FindUtxo(tt.args.outPoint); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UtxoSet.FindUtxo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUtxoSet_AddUtxo(t *testing.T) {
	key := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 12,
	}
	value := &(UtxoEntry{
		output: types.TxOut{
			Value:        100000000,
			ScriptPubKey: []byte{},
		},
		BlockHeight: 10000,
		IsCoinBase:  false,
		IsSpent:     false,
	})
	outPoint := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 12,
	}
	txin := &types.TxIn{
		PrevOutPoint: outPoint,
		ScriptSig:    []byte{},
		Sequence:     1,
	}
	vin := []*types.TxIn{
		txin,
	}
	txout := &types.TxOut{
		Value:        1,
		ScriptPubKey: []byte{},
	}
	vout := []*types.TxOut{txout}
	tx := &types.MsgTx{
		Version:  1,
		Vin:      vin,
		Vout:     vout,
		Magic:    1,
		LockTime: 1,
	}
	type fields struct {
		utxoMap map[types.OutPoint]*UtxoEntry
	}
	type args struct {
		tx          *types.MsgTx
		txOutIdx    uint32
		blockHeight int32
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"tests",
			fields{
				map[types.OutPoint]*UtxoEntry{
					key: value,
				},
			},
			args{
				tx:          tx,
				txOutIdx:    0,
				blockHeight: 12345,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UtxoSet{
				utxoMap: tt.fields.utxoMap,
			}
			if err := u.AddUtxo(tt.args.tx, tt.args.txOutIdx, tt.args.blockHeight); (err != nil) != tt.wantErr {
				t.Errorf("UtxoSet.AddUtxo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUtxoSet_RemoveUtxo(t *testing.T) {
	key := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 1,
	}
	value := &(UtxoEntry{
		output: types.TxOut{
			Value:        100000000,
			ScriptPubKey: []byte{},
		},
		BlockHeight: 10000,
		IsCoinBase:  false,
		IsSpent:     false,
	})
	type fields struct {
		utxoMap map[types.OutPoint]*UtxoEntry
	}
	type args struct {
		outPoint types.OutPoint
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"tests",
			fields{
				map[types.OutPoint]*UtxoEntry{
					key: value,
				},
			},
			args{
				outPoint: types.OutPoint{
					Hash:  crypto.HashType{0x0010},
					Index: 1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UtxoSet{
				utxoMap: tt.fields.utxoMap,
			}
			u.RemoveUtxo(tt.args.outPoint)
		})
	}
}

func TestUtxoSet_ApplyTx(t *testing.T) {
	key := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 1,
	}
	value := &(UtxoEntry{
		output: types.TxOut{
			Value:        100000000,
			ScriptPubKey: []byte{},
		},
		BlockHeight: 10000,
		IsCoinBase:  false,
		IsSpent:     false,
	})
	outpoint := types.OutPoint{
		Hash:  crypto.HashType{0x0011},
		Index: 2,
	}
	txin := &types.TxIn{
		PrevOutPoint: outpoint,
		ScriptSig:    []byte{},
		Sequence:     2,
	}
	vin := []*types.TxIn{
		txin,
	}
	txout := &types.TxOut{
		Value:        11111,
		ScriptPubKey: []byte{},
	}
	vout := []*types.TxOut{
		txout,
	}
	tx := &types.MsgTx{
		Version:  2,
		Vin:      vin,
		Vout:     vout,
		Magic:    123,
		LockTime: 123,
	}
	type fields struct {
		utxoMap map[types.OutPoint]*UtxoEntry
	}
	type args struct {
		tx          *types.MsgTx
		blockHeight int32
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"tests",
			fields{
				map[types.OutPoint]*UtxoEntry{
					key: value,
				},
			},
			args{
				tx:          tx,
				blockHeight: 1234567,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UtxoSet{
				utxoMap: tt.fields.utxoMap,
			}
			if err := u.ApplyTx(tt.args.tx, tt.args.blockHeight); (err != nil) != tt.wantErr {
				t.Errorf("UtxoSet.ApplyTx() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUtxoSet_ApplyBlock(t *testing.T) {
	key := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 1,
	}
	value := &(UtxoEntry{
		output: types.TxOut{
			Value:        100000000,
			ScriptPubKey: []byte{},
		},
		BlockHeight: 10000,
		IsCoinBase:  true,
		IsSpent:     false,
	})
	outpoint := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 1,
	}
	txin := &types.TxIn{
		PrevOutPoint: outpoint,
		ScriptSig:    []byte{},
		Sequence:     1,
	}
	vin := []*types.TxIn{
		txin,
	}
	txout := &types.TxOut{
		Value:        1,
		ScriptPubKey: []byte{},
	}
	vout := []*types.TxOut{
		txout,
	}
	msgtx := &types.MsgTx{
		Version:  1,
		Vin:      vin,
		Vout:     vout,
		Magic:    1,
		LockTime: 1,
	}
	txs := []*types.MsgTx{
		msgtx,
	}
	block := &types.Block{
		Hash: &crypto.HashType{0x0010},
		MsgBlock: &types.MsgBlock{
			Header: &types.BlockHeader{
				Version:       1,
				PrevBlockHash: crypto.HashType{0x0010},
				TxsRoot:       crypto.HashType{0x0010},
				TimeStamp:     123,
				Magic:         1,
			},
			Txs: txs,
		},
		Height: 10000,
	}

	type fields struct {
		utxoMap map[types.OutPoint]*UtxoEntry
	}
	type args struct {
		block *types.Block
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"tests",
			fields{
				map[types.OutPoint]*UtxoEntry{
					key: value,
				},
			},
			args{
				block,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UtxoSet{
				utxoMap: tt.fields.utxoMap,
			}
			if err := u.ApplyBlock(tt.args.block); (err != nil) != tt.wantErr {
				t.Errorf("UtxoSet.ApplyBlock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUtxoSet_RevertTx(t *testing.T) {
	key := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 1,
	}
	value := &(UtxoEntry{
		output: types.TxOut{
			Value:        100000000,
			ScriptPubKey: []byte{},
		},
		BlockHeight: 10000,
		IsCoinBase:  true,
		IsSpent:     false,
	})
	outpoint := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 1,
	}
	txin := &types.TxIn{
		PrevOutPoint: outpoint,
		ScriptSig:    []byte{},
		Sequence:     1,
	}
	vin := []*types.TxIn{txin}
	txout := &types.TxOut{
		Value:        1,
		ScriptPubKey: []byte{},
	}
	vout := []*types.TxOut{
		txout,
	}
	tx := &types.MsgTx{
		Version:  1,
		Vin:      vin,
		Vout:     vout,
		Magic:    1,
		LockTime: 1,
	}

	type fields struct {
		utxoMap map[types.OutPoint]*UtxoEntry
	}
	type args struct {
		tx          *types.MsgTx
		blockHeight int32
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"tests",
			fields{
				map[types.OutPoint]*UtxoEntry{
					key: value,
				},
			},
			args{
				tx:          tx,
				blockHeight: 111111,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UtxoSet{
				utxoMap: tt.fields.utxoMap,
			}
			if err := u.RevertTx(tt.args.tx, tt.args.blockHeight); (err != nil) != tt.wantErr {
				t.Errorf("UtxoSet.RevertTx() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUtxoSet_RevertBlock(t *testing.T) {
	key := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 1,
	}
	value := &(UtxoEntry{
		output: types.TxOut{
			Value:        100000000,
			ScriptPubKey: []byte{},
		},
		BlockHeight: 10000,
		IsCoinBase:  true,
		IsSpent:     false,
	})
	outpoint := types.OutPoint{
		Hash:  crypto.HashType{0x0010},
		Index: 1,
	}
	txin := &types.TxIn{
		PrevOutPoint: outpoint,
		ScriptSig:    []byte{},
		Sequence:     1,
	}
	vin := []*types.TxIn{
		txin,
	}
	txout := &types.TxOut{
		Value:        1,
		ScriptPubKey: []byte{},
	}
	vout := []*types.TxOut{
		txout,
	}
	tx := &types.MsgTx{
		Version:  1,
		Vin:      vin,
		Vout:     vout,
		Magic:    123,
		LockTime: 1234,
	}
	txs := []*types.MsgTx{
		tx,
	}
	block := &types.Block{
		Hash: &crypto.HashType{0x0010},
		MsgBlock: &types.MsgBlock{
			Header: &types.BlockHeader{
				Version:       1,
				PrevBlockHash: crypto.HashType{0x0010},
				TxsRoot:       crypto.HashType{0x0010},
				TimeStamp:     1234,
				Magic:         1,
			},
			Txs: txs,
		},
		Height: 1234,
	}
	type fields struct {
		utxoMap map[types.OutPoint]*UtxoEntry
	}
	type args struct {
		block *types.Block
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"tests",
			fields{
				map[types.OutPoint]*UtxoEntry{
					key: value,
				},
			},
			args{
				block,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UtxoSet{
				utxoMap: tt.fields.utxoMap,
			}
			if err := u.RevertBlock(tt.args.block); (err != nil) != tt.wantErr {
				t.Errorf("UtxoSet.RevertBlock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
