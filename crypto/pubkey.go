// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"github.com/btcsuite/btcd/btcec"
)

// PublicKey is a btcec.PublicKey wrapper
type PublicKey btcec.PublicKey

// Serialize get the serialized format of public key
func (p *PublicKey) Serialize() []byte {
	b := make([]byte, 0)
	b = append(b, p.X.Bytes()...)
	b = append(b, p.Y.Bytes()...)
	return b
}
