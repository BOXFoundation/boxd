// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/state"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/vm"
)

// define const.
const (
	VoutLimit = 1000
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
type StateProcessor struct {
	bc  *BlockChain
	cfg vm.Config
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(bc *BlockChain) *StateProcessor {
	return &StateProcessor{
		bc:  bc,
		cfg: bc.vmConfig,
	}
}

// Process processes the state changes using the statedb.
func (sp *StateProcessor) Process(block *types.Block, stateDB *state.StateDB) (uint64, []*types.Transaction, error) {

	header := block.Header
	usedGas := new(uint64)
	var utxoTxs []*types.Transaction
	for _, tx := range block.Txs {
		vmTx, err := sp.bc.ExtractVMTransactions(tx)
		if err != nil {
			return 0, nil, err
		}
		// statedb.Prepare(tx.Hash(), block.Hash(), i)
		gasUsedPerTx, txs, err := ApplyTransaction(vmTx, header, sp.bc, stateDB, sp.cfg)
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
func ApplyTransaction(tx *types.VMTransaction, header *types.BlockHeader, bc *BlockChain, statedb *state.StateDB,
	cfg vm.Config) (uint64, []*types.Transaction, error) {
	var txs []*types.Transaction
	context := NewEVMContext(tx, header, bc)
	vmenv := vm.NewEVM(context, statedb, cfg)
	_, gasRemaining, success, gasRefundTx, err := ApplyMessage(vmenv, tx)
	if err != nil {
		return 0, nil, err
	}
	if gasRefundTx != nil {
		txs = append(txs, gasRefundTx)
	}
	if success && len(Transfers) > 0 {
		txs = createUtxoTx()
		Transfers = nil
	}
	return gasRemaining, txs, nil
}

func createUtxoTx() []*types.Transaction {
	var txs []*types.Transaction
	for _, v := range Transfers {
		if len(v) > VoutLimit {
			txNumber := len(v)/VoutLimit + 1
			for i := 0; i < txNumber; i++ {
				var end int
				begin := i * VoutLimit
				if (i+1)*VoutLimit < len(v) {
					end = (i + 1) * VoutLimit
				} else {
					end = len(v) - begin
				}
				tx := makeTx(v, begin, end)
				txs = append(txs, tx)
			}
		} else {
			tx := makeTx(v, 0, len(v))
			txs = append(txs, tx)
		}
	}
	return txs
}

func makeTx(transferInfos []*TransferInfo, voutBegin int, voutEnd int) *types.Transaction {
	var vouts []*corepb.TxOut
	for i := voutBegin; i < voutEnd; i++ {
		to := transferInfos[i].to
		addrScript := *script.PayToPubKeyHashScript(to.Bytes())
		vout := &corepb.TxOut{
			Value:        transferInfos[i].value.Uint64(),
			ScriptPubKey: addrScript,
		}
		vouts = append(vouts, vout)
	}
	vin := &types.TxIn{
		PrevOutPoint: types.OutPoint{},
		ScriptSig:    *script.MakeContractScriptSig(),
	}
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vin)
	tx.Vout = append(tx.Vout, vouts...)
	return tx
}
