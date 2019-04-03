// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/script"
)

// HasContractVout return true if tx has a vout with contract creation or call
func HasContractVout(tx *types.Transaction) bool {
	for _, o := range tx.Vout {
		sc := script.NewScriptFromBytes(o.ScriptPubKey)
		if sc.IsContractPubkey() {
			return true
		}
	}
	return false
}

// HashContractCreation return true if tx has a vout with Op Creation script pubkey
func HashContractCreation(tx *types.Transaction) (bool, error) {
	for _, o := range tx.Vout {
		sc := script.NewScriptFromBytes(o.ScriptPubKey)
		if sc.IsContractPubkey() {
			p, err := sc.ParseContractParams()
			if err != nil {
				return false, err
			}
			if p.Receiver != nil {
				return true, nil
			}
		}
	}
	return false, nil
}

// HashContractCall return true if tx has a vout with Op Call script pubkey
func HashContractCall(tx *types.Transaction) bool {
	return true
}

// ContainsContractVin return true if tx has a vin with Op Spend script sig
func ContainsContractVin(tx *types.Transaction) bool {
	return true
}

// HasOpCreateOrCall return true if tx has a vout with script pubkey that
// includes Op Creation  or Op Call
func HasOpCreateOrCall(tx *types.Transaction) bool {
	return true
}

// ConvertToBoxTransaction converts Transaction to BoxTransaction
func ConvertToBoxTransaction(tx *types.Transaction) (*types.BoxTransaction, error) {
	return nil, nil
}
