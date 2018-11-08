// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"testing"

	"github.com/BOXFoundation/boxd/crypto"
	"github.com/facebookgo/ensure"
)

var (
	tokenName   = "box"
	tokenSupply = uint64(3000000000000)

	tokentTxHashStr = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tokenTxOutIdx   = uint32(1)
)

func TestIssueToken(t *testing.T) {
	params := &IssueParams{Name: tokenName, TotalSupply: tokenSupply}
	script := IssueTokenScript(testPubKeyHash, params)

	ensure.True(t, script.IsTokenIssue())
	ensure.True(t, script.P2PKHScriptPrefix().IsPayToPubKeyHash())

	params2, err := script.GetIssueParams()
	ensure.Nil(t, err)
	ensure.DeepEqual(t, params2, params)
}

func TestTransferToken(t *testing.T) {
	tokenTxHash := &crypto.HashType{}
	err := tokenTxHash.SetString(tokentTxHashStr)
	ensure.Nil(t, err)

	params := &TransferParams{}
	params.Hash = *tokenTxHash
	params.Index = tokenTxOutIdx
	params.Amount = tokenSupply
	script := TransferTokenScript(testPubKeyHash, params)

	ensure.True(t, script.IsTokenTransfer())
	ensure.True(t, script.P2PKHScriptPrefix().IsPayToPubKeyHash())

	params2, err := script.GetTransferParams()
	ensure.Nil(t, err)
	ensure.DeepEqual(t, params2, params)
}
