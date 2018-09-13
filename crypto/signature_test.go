package crypto

import (
	"testing"
)

func TestSignMessage(t *testing.T) {
	privKey, pubKey, err := NewKeyPair()
	if err != nil {
		t.Errorf("Error generating new key pair: %s", err)
		return
	}

	message := "dummy test message"
	messageHash := Sha256(Sha256([]byte(message)))
	sig, err := Sign(privKey, messageHash)

	if err != nil {
		t.Errorf("Error signing: %s", err)
		return
	}
	if !sig.VerifySignature(pubKey, messageHash) {
		t.Error("Signature verification failed")
	}

	// use another public key
	_, pubKey2, err := NewKeyPair()
	if err != nil {
		t.Errorf("Error generating new key pair: %s", err)
		return
	}
	if sig.VerifySignature(pubKey2, messageHash) {
		t.Error("Signature verification succeeded, but should have failed because of wrong public key")
	}
}
