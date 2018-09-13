package crypto

import (
	"github.com/btcsuite/btcd/btcec"
)

var (
	curve = btcec.S256()
)

// PrivateKey is a btcec.PrivateKey wrapper
type PrivateKey btcec.PrivateKey

// KeyPairFromBytes returns a private and public key pair from private key passed as a byte slice privKeyBytes
func KeyPairFromBytes(privKeyBytes []byte) (*PrivateKey, *PublicKey) {
	privKey, pubKey := btcec.PrivKeyFromBytes(curve, privKeyBytes)
	return (*PrivateKey)(privKey), (*PublicKey)(pubKey)
}

// NewKeyPair returns a new private and public key pair
func NewKeyPair() (*PrivateKey, *PublicKey, error) {
	btcecPrivKey, err := btcec.NewPrivateKey(curve)
	if err != nil {
		return nil, nil, err
	}
	privKey := (*PrivateKey)(btcecPrivKey)
	return privKey, privKey.PubKey(), nil
}

// PubKey returns the PublicKey corresponding to this private key.
func (p *PrivateKey) PubKey() *PublicKey {
	return (*PublicKey)((*btcec.PrivateKey)(p).PubKey())
}
