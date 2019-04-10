// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"github.com/BOXFoundation/boxd/core/state"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/vm"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
type StateProcessor struct {
	bc *BlockChain
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(bc *BlockChain) *StateProcessor {
	return &StateProcessor{
		bc: bc,
	}
}

// Process processes the state changes using the statedb.
func (sp *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (uint64, []*types.Transaction, error) {

	header := block.Header
	usedGas := new(uint64)
	var utxoTxs []*types.Transaction
	for _, tx := range block.Txs {
		sender, err := FetchOwnerOfOutPoint(&tx.Vin[0].PrevOutPoint, statedb.DB())
		if err != nil {
			return 0, nil, err
		}
		vmTx, err := ExtractVMTransactions(tx, sender)
		if err != nil {
			return 0, nil, err
		}
		// statedb.Prepare(tx.Hash(), block.Hash(), i)
		gasUsedPerTx, txs, err := ApplyTransaction(vmTx, header, sp.bc, statedb, cfg)
		if err != nil {
			return 0, nil, err
		}
		if txs != nil {
			utxoTxs = append(utxoTxs, txs...)
		}
		*usedGas += gasUsedPerTx
	}
	return *usedGas, utxoTxs, nil
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment.
func ApplyTransaction(tx *types.VMTransaction, header *types.BlockHeader, bc *BlockChain, statedb *state.StateDB, cfg vm.Config) (uint64, []*types.Transaction, error) {
	var txs []*types.Transaction
	context := NewEVMContext(tx, header, bc)
	vmenv := vm.NewEVM(context, statedb, cfg)
	_, gas, success, err := ApplyMessage(vmenv, tx)
	if err != nil {
		return 0, nil, err
	}
	if success && len(Transfers) > 0 {
		txs = createUtxoTx()
		Transfers = nil
		return gas, txs, nil
	}
	return gas, nil, nil
}

func createUtxoTx() []*types.Transaction {
	return nil
}
