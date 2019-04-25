// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/crypto"
	base58 "github.com/jbenet/go-base58"
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

func TestAddressPrefix(t *testing.T) {
	var tests = []struct {
		addr, hexAddr string
	}{
		{
			"b1111111111111111111111111111111111",
			"1324d9f2a535969f9668fbdad0494b4095e44d3532c800000000",
		},
		{
			"b2111111111111111111111111111111111",
			"13275629f0d8cbcb8bf89d79b0f9e2022d13480de40200000000",
		},
		{
			"b3111111111111111111111111111111111",
			"1329d2613c7c00f781883f1891aa78c3c44242e6953c00000000",
		},
		{
			"b4111111111111111111111111111111111",
			"132c4e98881f36237717e0b7725b0f855b713dbf467600000000",
		},
		{
			"b5111111111111111111111111111111111",
			"132ecacfd3c26b4f6ca78256530ba646f2a03897f7b000000000",
		},
		{
			"b6111111111111111111111111111111111",
			"133147071f65a07b623723f533bc3d0889cf3370a8ea00000000",
		},
	}
	for _, tc := range tests {
		rawBytes := base58.Decode(tc.addr)
		hexAddr := hex.EncodeToString(rawBytes)
		if tc.hexAddr != hexAddr {
			t.Errorf("hex addr for %s want: %s, got: %s", tc.addr, tc.hexAddr, hexAddr)
		}
	}
}

func TestAddressValidate(t *testing.T) {
	var tests = []struct {
		addr  string
		valid bool
	}{
		{
			"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x",
			true,
		},
		{
			"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x0",
			false,
		},
		{
			"b1bfGiSykHFaiCeXgYibFN141aBwZURsA9",
			false,
		},
		{
			"b1bfGiSykHFaiCeXgYibFN141aBwZURsA90",
			false,
		},
		{
			"b2KWSqUWZHTdP4g8kHkHtFtNc8Nofr1twqz",
			true,
		},
		{
			"b2KWSqUWZHTdP4g8kHkHtFtNc8Nofr1twq0",
			false,
		},
		{
			"b3KWSqUWZHTdP4g8kHkHtFtNc8Nofr1twq0",
			false,
		},
		{
			"b5WYphc4yBPH18gyFthS1bHyRcEvM6xANuT",
			true,
		},
		{
			"b5KWSqUWZHTdP4g8kHkHtFtNc8Nofr1twq0",
			false,
		},
	}

	for _, tc := range tests {
		_, err := ParseAddress(tc.addr)
		if err != nil && tc.valid {
			t.Error(err)
		}
	}
}

func TestMakeContractAddress(t *testing.T) {
	// parameters
	senderStr := "b1bfGiSykHFaiCeXgYibFN141aBwZURsA9x"
	nonce := uint64(6)
	sender, _ := NewAddress(senderStr)
	//
	ca, err := MakeContractAddress(sender, nonce)
	if err != nil {
		t.Fatal(err)
	}
	wantAddr := "b5hogno5HPcJcMav8gpG4AE1sHPCD6Driqr"
	if ca.String() != wantAddr {
		t.Fatalf("want: %s, got: %s", wantAddr, ca)
	}
}
