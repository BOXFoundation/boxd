// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"crypto/rand"
	"io"
	"testing"

	"github.com/facebookgo/ensure"
)

func TestSignMessage(t *testing.T) {
	privKey, pubKey, err := NewKeyPair()
	ensure.Nil(t, err)

	// serialize & deserialize
	pubKeyBytes := pubKey.Serialize()
	pubKey2, _ := PublicKeyFromBytes(pubKeyBytes)
	ensure.DeepEqual(t, pubKey, pubKey2)

	message := "dummy test message"
	msgHash := DoubleHashH([]byte(message))
	messageHash := &msgHash
	sig, err := Sign(privKey, messageHash)
	ensure.Nil(t, err)

	ensure.True(t, sig.VerifySignature(pubKey, messageHash))

	// use another public key
	_, pubKeyNew, err := NewKeyPair()
	ensure.Nil(t, err)
	ensure.False(t, sig.VerifySignature(pubKeyNew, messageHash))

	// serialize & deserialize
	sigBytes := sig.Serialize()
	sig2, _ := SigFromBytes(sigBytes)
	ensure.DeepEqual(t, sig, sig2)
}

func TestRecoverCompact(t *testing.T) {
	buf := make([]byte, 256)
	count, err := io.ReadFull(rand.Reader, buf)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, count, len(buf))

	privKey, pubKey, err := NewKeyPair()
	ensure.Nil(t, err)
	sig, err := SignCompact(privKey, buf)
	ensure.Nil(t, err)

	pubKeyRecovered, ok := RecoverCompact(buf, sig)
	ensure.True(t, ok)
	ensure.DeepEqual(t, pubKey, pubKeyRecovered)
}
