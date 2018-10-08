// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/BOXFoundation/boxd/core/types"
)

// txValidateItem holds a transaction along with which input to validate.
type txValidateItem struct {
	txInIndex int
	txIn      *types.TxIn
	tx        *types.Transaction
	// sigHashes *txscript.TxSigHashes
}

// txValidator provides a type which asynchronously validates transaction
// inputs.  It provides several channels for communication and a processing
// function that is intended to be in run multiple goroutines.
type txValidator struct {
	validateChan chan *txValidateItem
	quitChan     chan struct{}
	resultChan   chan error
	utxoUnspent  *UtxoUnspentCache
	// flags        txscript.ScriptFlags
	// sigCache     *txscript.SigCache
	// hashCache    *txscript.HashCache
}
