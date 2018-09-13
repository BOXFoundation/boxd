package crypto

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec"
)

// Signature is a btcec.PublicKey wrapper
type Signature btcec.Signature

// Sign calculates an ECDSA signature of messageHash using privateKey.
func Sign(privKey *PrivateKey, messageHash []byte) (*Signature, error) {
	if len(messageHash) != 32 {
		return nil, fmt.Errorf("hash must be be exactly 32 bytes (%d)", len(messageHash))
	}
	btcecSig, err := (*btcec.PrivateKey)(privKey).Sign(messageHash)
	return (*Signature)(btcecSig), err
}

// VerifySignature verifies that the given public key created signature over messageHash.
func (sig *Signature) VerifySignature(pubKey *PublicKey, messageHash []byte) bool {
	return (*btcec.Signature)(sig).Verify(messageHash, (*btcec.PublicKey)(pubKey))
}
