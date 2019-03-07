// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/crypto"
	"golang.org/x/crypto/ripemd160"
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
			ab, err := NewAddress(aa.String())
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(aa, ab) {
				t.Errorf("NewAddressPubKeyHash() = %v, want %v", aa, ab)
			}

			ac, err := NewAddress(tt.addr)
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

func TestNewAddressFromPubKeyHash(t *testing.T) {
	tests := []struct {
		name    string
		pkhash  []byte
		addr    string
		errWant error
	}{
		{
			name:    "Min Bytes",
			pkhash:  bytes.Repeat([]byte{0}, ripemd160.Size),
			addr:    "b1ToofJ9HTLVywiTeyZmJ5dFUFmy24ksUhm",
			errWant: nil,
		},
		{
			name:    "Max Bytes",
			pkhash:  bytes.Repeat([]byte{255}, ripemd160.Size),
			addr:    "b1s9QeQSaAWxrm9bjzz6cZkXFtHDxc97wG4",
			errWant: nil,
		},
		{
			name: "Normal Bytes",
			pkhash: []byte{
				0x0e, 0xf0, 0x30, 0x10, 0x7f, 0xd2, 0x6e, 0x0b, 0x6b, 0xf4,
				0x05, 0x12, 0xbc, 0xa2, 0xce, 0xb1, 0xdd, 0x80, 0xad, 0xaa},
			addr:    "b1VAnrX665aeExMaPeW6pk3FZKCLuywUaHw",
			errWant: nil,
		},
		{
			name:    "Incorrent Length",
			pkhash:  bytes.Repeat([]byte{1}, ripemd160.Size+1),
			addr:    "",
			errWant: core.ErrInvalidPKHash,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewAddressPubKeyHash(test.pkhash)
			if err != test.errWant {
				t.Errorf("newAddressPubKeyHash() error = %v, errWant %v", err, test.errWant)
				return
			} else if err != nil {
				return
			}
			if err == nil && got.String() != test.addr {
				t.Errorf("newAddressPubKeyHash() = %v, want %v", got, test.addr)
			}
		})
	}
}

func TestNewAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		pkhash  []byte
		errWant error
	}{
		{
			name:    "Regular",
			addr:    "b1ToofJ9HTLVywiTeyZmJ5dFUFmy24ksUhm",
			pkhash:  bytes.Repeat([]byte{0}, ripemd160.Size),
			errWant: nil,
		},
		{
			name:    "Invalid Encoding",
			addr:    "b1ToofJ9HTLVyw=TeyZmJ5dFUFmy24ksUhm",
			pkhash:  nil,
			errWant: crypto.ErrInvalidBase58Encoding,
		},
		{
			name:    "Invalid Length",
			addr:    "b1ToofJ9HTLVywiTeyZmJ5dFUFmy24ksUh",
			pkhash:  nil,
			errWant: core.ErrInvalidAddressString,
		},
		{
			name:    "Invalid Prefix 1",
			addr:    "c1ToofJ9HTLVywiTeyZmJ5dFUFmy24ksUhm",
			pkhash:  nil,
			errWant: core.ErrInvalidAddressString,
		},
		{
			name:    "Invalid Prefix 2",
			addr:    "b1sEiXMHKDdq19dDiCbDjv72Csy9F6VNidZ",
			pkhash:  nil,
			errWant: core.ErrInvalidAddressString,
		},
		{
			name:    "Invalid checksum",
			addr:    "b1ToofJ9HTLVywiTeyZmJ5dFUFmy24ksUhn",
			pkhash:  nil,
			errWant: crypto.ErrInvalidBase58Checksum,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			addr, err := NewAddress(test.addr)
			if err != test.errWant {
				t.Errorf("NewAddress() = %v, want: %v", err, test.errWant)
				return
			} else if err != nil {
				return
			}
			if !bytes.Equal(addr.Hash(), test.pkhash) {
				t.Errorf("NewAddress() got pkhash: %v, want: %v", addr.Hash(), test.pkhash)
			}
			if !bytes.Equal(addr.Hash160()[:], test.pkhash) {
				t.Errorf("NewAddress() got Hash160: %v, want: %v", addr.Hash160(), test.pkhash)
			}
		})
	}
}
