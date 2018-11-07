// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"reflect"
	"testing"
)

func TestNewAddressPubKeyHash(t *testing.T) {

	tests := []struct {
		name   string
		pkHash []byte
		addr   string
	}{
		{
			name: "test pubkey hash address",
			pkHash: []byte{
				0x0e, 0xf0, 0x30, 0x10, 0x7f, 0xd2, 0x6e, 0x0b, 0x6b, 0xf4,
				0x05, 0x12, 0xbc, 0xa2, 0xce, 0xb1, 0xdd, 0x80, 0xad, 0xaa},
			addr: "b1VAnrX665aeExMaPeW6pk3FZKCLuywUaHw",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aa, err := NewAddressPubKeyHash(tt.pkHash)
			if err != nil {
				t.Error(err)
			}
			ab, err := ParseAddress(aa.String())
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(aa, ab) {
				t.Errorf("NewAddressPubKeyHash() = %v, want %v", aa, ab)
			}

			ac, err := ParseAddress(tt.addr)
			if err != nil {
				t.Error(err)
			}
			ad := ac.String()
			if !reflect.DeepEqual(tt.addr, ad) {
				t.Errorf("NewAddressPubKeyHash() = %v, want %v", tt.addr, ad)
			}
		})
	}
}

func Test_newAddressPubKeyHash(t *testing.T) {
	type args struct {
		pkHash []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *AddressPubKeyHash
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newAddressPubKeyHash(tt.args.pkHash)
			if (err != nil) != tt.wantErr {
				t.Errorf("newAddressPubKeyHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newAddressPubKeyHash() = %v, want %v", got, tt.want)
			}
		})
	}
}
