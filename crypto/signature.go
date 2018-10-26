// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec"
)

// Signature is a btcec.Signature wrapper
type Signature btcec.Signature

// Signer is the interface which will generate a Signature from HashType
type Signer interface {
	Sign(messageHash *HashType) (*Signature, error)
}

// Sign calculates an ECDSA signature of messageHash using privateKey.
func Sign(privKey *PrivateKey, messageHash *HashType) (*Signature, error) {
	btcecSig, err := (*btcec.PrivateKey)(privKey).Sign(messageHash[:])
	return (*Signature)(btcecSig), err
}

// VerifySignature verifies that the given public key created signature over messageHash.
func (sig *Signature) VerifySignature(pubKey *PublicKey, messageHash *HashType) bool {
	return (*btcec.Signature)(sig).Verify(messageHash[:], (*btcec.PublicKey)(pubKey))
}

// IsEqual returns if the passed signature is equivalent to this signature
func (sig *Signature) IsEqual(otherSig *Signature) bool {
	return (*btcec.Signature)(sig).IsEqual((*btcec.Signature)(otherSig))
}

// Serialize returns the ECDSA signature in the DER format.
func (sig *Signature) Serialize() []byte {
	return (*btcec.Signature)(sig).Serialize()
}

// SigFromBytes returns signature from raw bytes in DER format
func SigFromBytes(sigStr []byte) (*Signature, error) {
	sig, err := btcec.ParseDERSignature(sigStr, secp256k1Curve)
	return (*Signature)(sig), err
}

// SignCompact sign a byte array message using private key, and generate
// a signature with recover params, which can be used to recover public key
func SignCompact(privKey *PrivateKey, digest []byte) ([]byte, error) {
	return btcec.SignCompact(secp256k1Curve, (*btcec.PrivateKey)(privKey), digest, true)
}

// RecoverCompact tries to recover public key from message digest and signatures
func RecoverCompact(digest, sig []byte) (*PublicKey, bool) {
	publicKey, onCurve, err := btcec.RecoverCompact(secp256k1Curve, sig, digest)
	if !onCurve || err != nil {
		fmt.Printf("recover err: %v", err)
		return nil, false
	}
	return (*PublicKey)(publicKey), true
}
