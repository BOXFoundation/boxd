// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import "github.com/BOXFoundation/boxd/crypto"

var (
	testPrivKey, testPubKey, _ = crypto.NewKeyPair()
	testPubKeyBytes            = testPubKey.Serialize()
	testPubKeyHash             = crypto.Hash160(testPubKeyBytes)
)
