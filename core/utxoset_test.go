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
			"test",
			fields{
				map[types.OutPoint]*UtxoEntry{
					types.OutPoint{
						Hash:  crypto.HashType{0x0010},
						Index: 1,
					},
					&(UtxoEntry{
						output: types.TxOut{
							Value:        100000000,
							ScriptPubKey: []byte{},
						},
						BlockHeight: 10000,
						IsCoinBase:  true,
						IsSpent:     true,
					}),
				},
			},
			args{
				outPoint: types.OutPoint{
					Hash:  crypto.HashType{0x0010},
					Index: 1,
				},
			},
			&UtxoEntry{
				utxoEntry: types.TxOut{
					Value:        100000000,
					ScriptPubKey: []byte{},
				},
				BlockHeight: 10000,
				IsCoinBase:  true,
				IsSpent:     true,
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
			"test",
			fields{
				map[types.OutPoint]*UtxoEntry{
					outPoint: types.OutPoint{
						Hash:  crypto.HashType{0x0010},
						Index: 1,
					},
					&(UtxoEntry{
						output: types.TxOut{
							Value:        100000000,
							ScriptPubKey: []byte{},
						},
						BlockHeight: 10000,
						IsCoinBase:  true,
						IsSpent:     true,
					}),
				},
			},
			args{
				tx: &types.MsgTx{
					Version: 1,
					Vin: []*types.TxIn{
						PrevOutPoint: types.OutPoint{
							ScriptSig: crypto.HashType{0x0010},
							Sequence:  1,
						},
					},
					Vout: []*types.TxOut{
						Value:        1,
						ScriptPubKey: []byte{},
					},
					Magic:    1,
					LockTime: 1,
				},
				txOutIdx:    1,
				blockHeight: 12345,
			},
			true,
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
			"test",
			fields{
				map[types.OutPoint]*UtxoEntry{
					outPoint: types.OutPoint{
						Hash:  crypto.HashType{0x0010},
						Index: 1,
					},
					&(UtxoEntry{
						output: types.TxOut{
							Value:        100000000,
							ScriptPubKey: []byte{},
						},
						BlockHeight: 10000,
						IsCoinBase:  true,
						IsSpent:     true,
					}),
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
			"test",
			fields{
				map[types.OutPoint]*UtxoEntry{
					outPoint: types.OutPoint{
						Hash:  crypto.HashType{0x0010},
						Index: 1,
					},
					&(UtxoEntry{
						output: types.TxOut{
							Value:        100000000,
							ScriptPubKey: []byte{},
						},
						BlockHeight: 10000,
						IsCoinBase:  true,
						IsSpent:     true,
					}),
				},
			},
			args{
				tx: &types.MsgTx{
					Version: 1,
					Vin: []*types.TxIn{
						PrevOutPoint: types.OutPoint{
							Hash:  crypto.HashType{0x0010},
							Index: 1,
						},
						ScriptSig: []byte{},
						Sequence:  1,
					},
					Vout: []*types.TxOut{
						Value:        1,
						ScriptPubKey: []byte{},
					},
					Magic:    12,
					LockTime: 1234,
				},
				blockHeight: 1,
			},
			true,
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
			"test",
			fields{
				map[types.OutPoint]*UtxoEntry{
					outPoint: types.OutPoint{
						Hash:  crypto.HashType{0x0010},
						Index: 1,
					},
					&(UtxoEntry{
						output: types.TxOut{
							Value:        100000000,
							ScriptPubKey: []byte{},
						},
						BlockHeight: 10000,
						IsCoinBase:  true,
						IsSpent:     true,
					}),
				},
			},
			args{
				block: types.Block{
					Hash: &crypto.HashType{0x0010},
					MsgBlock: &types.MsgBlock{
						Header: &types.BlockHeader{
							Version:       1,
							PrevBlockHash: crypto.HashType{0x0010},
							TxsRoot:       crypto.HashType{0x0010},
							TimeStamp:     123,
							Magic:         1,
						},
						Txs: []*types.MsgTx{
							Version: 1,
							Vin: []*types.TxIn{
								PrevOutPoint: types.OutPoint{
									Hash:  crypto.HashType{0x0010},
									Index: 1,
								},
								ScriptSig: []byte{},
								Sequence:  12345,
							},
							Vout: []*types.TxOut{
								Value:        1234,
								ScriptPubKey: []byte{},
							},
							Magic:    1,
							LockTime: 1234,
						},
					},
					Height: 1234,
				},
			},
			true,
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
			"test",
			fields{
				map[types.OutPoint]*UtxoEntry{
					outPoint: types.OutPoint{
						Hash:  crypto.HashType{0x0010},
						Index: 1,
					},
					&(UtxoEntry{
						output: types.TxOut{
							Value:        100000000,
							ScriptPubKey: []byte{},
						},
						BlockHeight: 10000,
						IsCoinBase:  true,
						IsSpent:     true,
					}),
				},
			},
			args{
				tx: &types.MsgTx{
					Version: 1,
					Vin: []*types.TxIn{
						PrevOutPoint: types.OutPoint{
							Hash:  crypto.HashType{0x0010},
							Index: 1,
						},
					},
					Vout: []*types.TxOut{
						Value:        1,
						ScriptPubKey: []byte{},
					},
					Magic:    12,
					LockTime: 1122,
				},
				blockHeight: 111111,
			},
			true,
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
			"test",
			fields{
				map[types.OutPoint]*UtxoEntry{
					outPoint: types.OutPoint{
						Hash:  crypto.HashType{0x0010},
						Index: 1,
					},
					&(UtxoEntry{
						output: types.TxOut{
							Value:        100000000,
							ScriptPubKey: []byte{},
						},
						BlockHeight: 10000,
						IsCoinBase:  true,
						IsSpent:     true,
					}),
				},
			},
			args{
				block: &types.Block{
					Hash: &crypto.HashType{0x0010},
					MsgBlock: &types.MsgBlock{
						Header: &types.BlockHeader{
							Version:       1,
							PrevBlockHash: crypto.HashType{0x0010},
							TxsRoot:       crypto.HashType{0x0010},
							TimeStamp:     1234,
							Magic:         1,
						},
						Txs: []*types.MsgTx{
							Version: 1,
							Vin: []*types.TxIn{
								PrevOutPoint: types.OutPoint{
									Hash:  crypto.HashType{0x0010},
									Index: 1,
								},
								ScriptSig: []byte{},
								Sequence:  1,
							},
							Vout: types.TxOut{
								Value:        1,
								ScriptPubKey: []byte{},
							},
							Magic:    123,
							LockTime: 1234,
						},
					},
					Height: 1234,
				},
			},
			true,
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
