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
func (sp *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (uint64, error) {

	header := block.Header
	usedGas := new(uint64)

	for _, tx := range block.Txs {
		boxTx, err := ExtractBoxTransactions(tx, statedb.DB())
		if err != nil {
			return 0, err
		}
		// statedb.Prepare(tx.Hash(), block.Hash(), i)
		gasUsedPerTx, err := ApplyTransaction(boxTx, header, sp.bc, statedb, cfg)
		if err != nil {
			return 0, err
		}
		*usedGas += gasUsedPerTx
	}
	return *usedGas, nil
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment.
func ApplyTransaction(tx *types.BoxTransaction, header *types.BlockHeader, bc *BlockChain, statedb *state.StateDB, cfg vm.Config) (uint64, error) {
	context := NewEVMContext(tx, header, bc)
	vmenv := vm.NewEVM(context, statedb, cfg)
	_, gas, _, err := ApplyMessage(vmenv, tx)
	if err != nil {
		return 0, err
	}
	return gas, nil
}
