// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utxo

import (
	"bytes"

	"github.com/BOXFoundation/boxd/script"
)

type filter interface {
	accept(scriptBytes []byte) bool
}

type addressFilter struct {
	scriptPrefix []byte
}

func (a *addressFilter) accept(scriptBytes []byte) bool {
	return bytes.HasPrefix(scriptBytes, a.scriptPrefix)
}

type p2pkhFilter struct {
}

func (*p2pkhFilter) accept(scriptBytes []byte) bool {
	sc := *script.NewScriptFromBytes(scriptBytes)
	if sc != nil && sc.IsPayToPubKeyHash() {
		return true
	}
	return false
}
