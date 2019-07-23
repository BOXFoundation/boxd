// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"encoding/hex"
	"errors"
	"fmt"

	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
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
func (sp *StateProcessor) Process(
	block *types.Block, stateDB *state.StateDB, utxoSet *UtxoSet,
) (types.Receipts, uint64, uint64, []*types.Transaction, error) {

	var (
		header          = block.Header
		usedGas         = new(uint64)
		gasRemainingFee = new(uint64)
		receipts        types.Receipts
		utxoTxs         []*types.Transaction
		err             error
	)
	for i, tx := range block.Txs {
		vmTx, err1 := sp.bc.ExtractVMTransactions(tx)
		if err1 != nil {
			err = err1
			break
		}
		if vmTx == nil {
			continue
		}
		if vmTx.Nonce() != stateDB.GetNonce(*vmTx.From())+1 {
			return nil, 0, 0, nil, fmt.Errorf("incorrect nonce(%d, %d in statedb) in tx: %s",
				vmTx.Nonce(), stateDB.GetNonce(*vmTx.From()), vmTx.OriginTxHash())
		}
		thash, err1 := tx.TxHash()
		if err1 != nil {
			err = err1
			break
		}
		if block.Hash == nil {
			stateDB.Prepare(*thash, crypto.HashType{}, i)
		} else {
			stateDB.Prepare(*thash, *block.Hash, i)
		}
		receipt, gasUsedPerTx, gasRemainingFeePerTx, txs, err1 :=
			ApplyTransaction(vmTx, header, sp.bc, stateDB, sp.cfg, utxoSet)
		if err1 != nil {
			err = err1
			break
		}
		if txs != nil {
			utxoTxs = append(utxoTxs, txs...)
		}
		*usedGas += gasUsedPerTx
		logger.Errorf("===================gasUsedPerTx: %d", gasUsedPerTx)
		*gasRemainingFee += gasRemainingFeePerTx
		receipt.WithTxIndex(uint32(i)).WithBlockHash(block.BlockHash()).
			WithBlockHeight(block.Header.Height)
		receipts = append(receipts, receipt)
	}

	if err != nil {
		return nil, 0, 0, nil, err
	}

	return receipts, *usedGas, *gasRemainingFee, utxoTxs, nil
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment.
func ApplyTransaction(
	tx *types.VMTransaction, header *types.BlockHeader, bc *BlockChain,
	statedb *state.StateDB, cfg vm.Config, utxoSet *UtxoSet,
) (*types.Receipt, uint64, uint64, []*types.Transaction, error) {

	var txs []*types.Transaction
	defer func() {
		Transfers = make(map[types.AddressHash][]*TransferInfo)
	}()
	context := NewEVMContext(tx, header, bc)
	vmenv := vm.NewEVM(context, statedb, cfg)
	//logger.Infof("ApplyMessage tx: %+v, header: %+v", tx, header)
	ret, gasUsed, gasRemainingFee, fail, gasRefundTx, err := ApplyMessage(vmenv, tx)
	if err != nil {
		logger.Warn(err)
		return nil, 0, 0, nil, err
	}
	logger.Infof("result for ApplyMessage msg %s, gasUsed: %d, gasRemainingFee:"+
		" %d, failed: %t, return: %s", tx, gasUsed, gasRemainingFee, fail, hex.EncodeToString(ret))
	if gasRefundTx != nil {
		txHash, _ := gasRefundTx.TxHash()
		logger.Infof("gasRefund tx: %s", txHash)
		txs = append(txs, gasRefundTx)
	}

	var contractAddr *types.AddressHash
	if tx.Type() == types.ContractCreationType {
		contractAddr = types.CreateAddress(*tx.From(), tx.Nonce())
	} else {
		contractAddr = tx.To()
	}
	if !fail && len(Transfers) > 0 {
		internalTxs, err := createUtxoTx(utxoSet)
		if err != nil {
			logger.Warn(err)
			return nil, 0, 0, nil, err
		}
		txs = append(txs, internalTxs...)
	} else if fail && tx.Value().Uint64() > 0 { // tx failed
		internalTxs, err := createRefundTx(tx, utxoSet, contractAddr)
		if err != nil {
			logger.Warn(err)
			return nil, 0, 0, nil, err
		}
		txs = append(txs, internalTxs)
	}
	txhash := tx.OriginTxHash()
	receipt := types.NewReceipt(tx.OriginTxHash(), contractAddr, fail, gasUsed, statedb.GetLogs(*txhash))

	return receipt, gasUsed, gasRemainingFee, txs, nil
}

func createUtxoTx(utxoSet *UtxoSet) ([]*types.Transaction, error) {

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
				tx, err := makeTx(v, begin, end, utxoSet)
				if err != nil {
					logger.Error("create utxo tx error: ", err)
					return nil, err
				}
				txs = append(txs, tx)
			}
		} else {
			tx, err := makeTx(v, 0, len(v), utxoSet)
			if err != nil {
				logger.Error("create utxo tx error: ", err)
				return nil, err
			}
			txs = append(txs, tx)
		}
	}
	return txs, nil
}

func createRefundTx(
	vmtx *types.VMTransaction, utxoSet *UtxoSet, contractAddr *types.AddressHash,
) (*types.Transaction, error) {

	hash := types.NormalizeAddressHash(contractAddr)
	outPoint := *types.NewOutPoint(hash, 0)
	utxoWrap, ok := utxoSet.utxoMap[outPoint]
	if !ok {
		return nil, errors.New("contract utxo does not exist")
	}
	if utxoWrap.Value() < vmtx.Value().Uint64() {
		return nil, errors.New("Insufficient balance of smart contract")
	}
	value := utxoWrap.Value() - vmtx.Value().Uint64()
	utxoWrap.SetValue(value)
	utxoSet.utxoMap[outPoint] = utxoWrap

	vin := &types.TxIn{
		PrevOutPoint: outPoint,
		ScriptSig:    *script.MakeContractScriptSig(),
	}
	vout := &corepb.TxOut{
		Value:        vmtx.Value().Uint64(),
		ScriptPubKey: *script.PayToPubKeyHashScript(vmtx.From().Bytes()),
	}
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vin)
	tx.Vout = append(tx.Vout, vout)
	return tx, nil
}

func makeTx(
	transferInfos []*TransferInfo, voutBegin int, voutEnd int, utxoSet *UtxoSet,
) (*types.Transaction, error) {

	hash := types.NormalizeAddressHash(&transferInfos[0].from)
	if hash.IsEqual(&zeroHash) {
		return nil, errors.New("Invalid contract address")
	}
	var utxoWrap *types.UtxoWrap
	outPoint := types.NewOutPoint(hash, 0)
	if utxoWrap = utxoSet.utxoMap[*outPoint]; utxoWrap == nil {
		logger.Errorf("outpoint hash: %v, index: %d", outPoint.Hash[:], outPoint.Index)
		return nil, fmt.Errorf("contract utxo outpoint %+v does not exist", outPoint)
	}
	var vouts []*corepb.TxOut
	for i := voutBegin; i < voutEnd; i++ {
		to := transferInfos[i].to
		addrScript := *script.PayToPubKeyHashScript(to.Bytes())
		vout := &corepb.TxOut{
			Value:        transferInfos[i].value.Uint64(),
			ScriptPubKey: addrScript,
		}
		vouts = append(vouts, vout)
		value := utxoWrap.Value() - transferInfos[i].value.Uint64()
		if value > utxoWrap.Value() {
			return nil, errors.New("Insufficient balance of smart contract")
		}
		utxoWrap.SetValue(value)
	}
	utxoSet.utxoMap[*outPoint] = utxoWrap
	vin := &types.TxIn{
		PrevOutPoint: *outPoint,
		ScriptSig:    *script.MakeContractScriptSig(),
	}
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vin)
	tx.Vout = append(tx.Vout, vouts...)
	return tx, nil
}
