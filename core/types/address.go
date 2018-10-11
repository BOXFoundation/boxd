// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"errors"

	"github.com/BOXFoundation/boxd/crypto"

	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/ripemd160"
)

// Address is an interface type for any type of destination a transaction output may spend to.
type Address interface {
	String() string
	EncodeAddress() string
	ScriptAddress() []byte
}

// AddressPubKeyHash is an Address for a pay-to-pubkey-hash (P2PKH) transaction.
type AddressPubKeyHash struct {
	hash  [ripemd160.Size]byte
	netID byte
}

// NewAddressPubKeyHash returns a new AddressPubKeyHash.  pkHash mustbe 20 bytes.
func NewAddressPubKeyHash(pkHash []byte, netID byte) (*AddressPubKeyHash, error) {
	return newAddressPubKeyHash(pkHash, netID)
}

// NewAddressFromPubKey returns a new AddressPubKeyHash derived from an ecdsa public key
func NewAddressFromPubKey(pubKey *crypto.PublicKey, netID byte) (*AddressPubKeyHash, error) {
	pkHash := crypto.Ripemd160(crypto.Sha256(pubKey.Serialize()))
	return newAddressPubKeyHash(pkHash, netID)
}

func newAddressPubKeyHash(pkHash []byte, netID byte) (*AddressPubKeyHash, error) {
	// Check for a valid pubkey hash length.
	if len(pkHash) != ripemd160.Size {
		return nil, errors.New("pkHash must be 20 bytes")
	}

	addr := &AddressPubKeyHash{netID: netID}
	copy(addr.hash[:], pkHash)
	return addr, nil
}

// EncodeAddress returns the string encoding of a pay-to-pubkey-hash address.
func (a *AddressPubKeyHash) EncodeAddress() string {
	return encodeAddress(a.hash[:], a.netID)
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

func encodeAddress(hash160 []byte, netID byte) string {
	return base58.CheckEncode(hash160[:ripemd160.Size], netID)
}
