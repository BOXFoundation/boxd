// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"github.com/btcsuite/btcd/btcec"
)

// PublicKey is a btcec.PublicKey wrapper
type PublicKey btcec.PublicKey

// PublicKeySize defines length of public serialize
const PublicKeySize = btcec.PubKeyBytesLenCompressed

// Serialize get the serialized format of public key
func (p *PublicKey) Serialize() []byte {
	return (*btcec.PublicKey)(p).SerializeCompressed()
}

// PublicKeyFromBytes returns public key from raw bytes
func PublicKeyFromBytes(publicKeyStr []byte) (*PublicKey, error) {
	publicKey, err := btcec.ParsePubKey(publicKeyStr, secp256k1Curve)
	return (*PublicKey)(publicKey), err
}
