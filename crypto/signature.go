// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec"
)

// Signature is a btcec.PublicKey wrapper
type Signature btcec.Signature

// Sign calculates an ECDSA signature of messageHash using privateKey.
func Sign(privKey *PrivateKey, messageHash []byte) (*Signature, error) {
	if len(messageHash) != HashSize {
		return nil, fmt.Errorf("hash must be be exactly %d bytes (%d)", HashSize, len(messageHash))
	}
	btcecSig, err := (*btcec.PrivateKey)(privKey).Sign(messageHash)
	return (*Signature)(btcecSig), err
}

// VerifySignature verifies that the given public key created signature over messageHash.
func (sig *Signature) VerifySignature(pubKey *PublicKey, messageHash []byte) bool {
	return (*btcec.Signature)(sig).Verify(messageHash, (*btcec.PublicKey)(pubKey))
}
