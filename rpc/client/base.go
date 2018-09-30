// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package client

import (
	"github.com/BOXFoundation/Quicksilver/core"
	"github.com/BOXFoundation/Quicksilver/core/types"
)

func getScriptAddress(pubKeyHash []byte) ([]byte, error) {
	addr, err := types.NewAddressPubKeyHash(pubKeyHash, 0x00)
	if err != nil {
		return nil, err
	}
	return core.PayToPubKeyHashScript(addr.ScriptAddress())
}
