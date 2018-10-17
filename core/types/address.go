// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/ripemd160"
)

var addressTypeP2PKHPrefix = [2]byte{0x13, 0x26}
var addressTypeP2SHPrefix = [2]byte{0x13, 0x2b}

// Address is an interface type for any type of destination a transaction output may spend to.
type Address interface {
	String() string
	EncodeAddress() string
	ScriptAddress() []byte
}

// AddressPubKeyHash is an Address for a pay-to-pubkey-hash (P2PKH) transaction.
type AddressPubKeyHash struct {
	hash [ripemd160.Size]byte
}

// NewAddressPubKeyHash returns a new AddressPubKeyHash.  pkHash mustbe 20 bytes.
func NewAddressPubKeyHash(pkHash []byte) (*AddressPubKeyHash, error) {
	return newAddressPubKeyHash(pkHash)
}

// NewAddressFromPubKey returns a new AddressPubKeyHash derived from an ecdsa public key
func NewAddressFromPubKey(pubKey *crypto.PublicKey) (*AddressPubKeyHash, error) {
	pkHash := crypto.Ripemd160(crypto.Sha256(pubKey.Serialize()))
	return newAddressPubKeyHash(pkHash)
}

func newAddressPubKeyHash(pkHash []byte) (*AddressPubKeyHash, error) {
	// Check for a valid pubkey hash length.
	if len(pkHash) != ripemd160.Size {
		return nil, core.ErrInvalidPKHash
	}

	addr := &AddressPubKeyHash{}
	copy(addr.hash[:], pkHash)
	return addr, nil
}

// EncodeAddress returns the string encoding of a pay-to-pubkey-hash address.
func (a *AddressPubKeyHash) EncodeAddress() string {
	return encodeAddress(a.hash[:])
}

// ScriptAddress returns the bytes to be included in a txout script to pay to a pubkey hash.
func (a *AddressPubKeyHash) ScriptAddress() []byte {
	return a.hash[:]
}

// String returns a human-readable string for the pay-to-pubkey-hash address.
func (a *AddressPubKeyHash) String() string {
	return a.EncodeAddress()
}

// Hash160 returns the underlying array of the pubkey hash.
func (a *AddressPubKeyHash) Hash160() *[ripemd160.Size]byte {
	return &a.hash
}

func encodeAddress(hash []byte) string {
	b := make([]byte, 0, len(hash)+2)
	b = append(b, addressTypeP2PKHPrefix[:]...)
	b = append(b, hash[:]...)
	return base58CheckEncode(b)
}

func checksum(input []byte) (cksum [4]byte) {
	h := crypto.Sha256(crypto.Sha256(input))
	copy(cksum[:], h[:4])
	return
}

func base58CheckEncode(input []byte) string {
	b := make([]byte, 0, len(input)+4)
	b = append(b, input[:]...)
	checksum := checksum(input)
	b = append(b, checksum[:]...)
	return base58.Encode(b)
}
