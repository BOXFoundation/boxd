// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage"
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
) (types.Receipts, uint64, []*types.Transaction, error) {

	var (
		usedGas   uint64
		sumGas    uint64
		receipts  types.Receipts
		utxoTxs   []*types.Transaction
		invalidTx *types.Transaction
		err       error
	)
	header := block.Header
	for i, tx := range block.Txs {
		if sumGas > core.MaxBlockGasLimit {
			logger.Warnf("block %s:%d used very higher gas, now used %d, index: %d/%d)",
				block.BlockHash(), block.Header.Height, sumGas, i, len(block.Txs))
			return nil, 0, nil, core.ErrOutOfBlockGasLimit
		}
		vmTx, err1 := ExtractVMTransaction(tx, stateDB)
		if err1 != nil {
			err = err1
			invalidTx = tx
			break
		}
		if vmTx == nil {
			sumGas += core.TransferFee + tx.ExtraFee()
			continue
		}
		if vmTx.Nonce() != stateDB.GetNonce(*vmTx.From())+1 {
			err = fmt.Errorf("incorrect nonce(%d, %d in statedb) for %x in tx: %s",
				vmTx.Nonce(), stateDB.GetNonce(*vmTx.From()), vmTx.From()[:], vmTx.OriginTxHash())
			invalidTx = tx
			break
		}
		thash, err1 := tx.TxHash()
		if err1 != nil {
			err = err1
			invalidTx = tx
			break
		}
		if block.Hash == nil {
			stateDB.Prepare(*thash, crypto.HashType{}, i)
		} else {
			stateDB.Prepare(*thash, *block.Hash, i)
		}
		receipt, gasUsedPerTx, _, txs, err1 :=
			ApplyTransaction(vmTx, header, sp.bc, stateDB, sp.cfg, utxoSet)
		if err1 != nil {
			err = err1
			invalidTx = tx
			break
		}
		if len(txs) > 0 {
			utxoTxs = append(utxoTxs, txs...)
		}
		gasThisTx := vmTx.GasPrice().Uint64()*gasUsedPerTx + tx.ExtraFee()
		usedGas += gasThisTx
		sumGas += gasThisTx
		receipt.WithTxIndex(uint32(i)).WithBlockHash(block.BlockHash()).
			WithBlockHeight(block.Header.Height)
		receipts = append(receipts, receipt)
	}

	if err != nil {
		sp.notifyInvalidTx(invalidTx)
		return nil, 0, nil, err
	}

	logger.Infof("block %s used gas %d in state process, total gas: %d",
		block.BlockHash(), usedGas, sumGas)
	return receipts, usedGas, utxoTxs, nil
}

func (sp *StateProcessor) notifyInvalidTx(tx *types.Transaction) {
	sp.bc.bus.Publish(eventbus.TopicInvalidTx, tx, true)
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment.
func ApplyTransaction(
	tx *types.VMTransaction, header *types.BlockHeader, bc *BlockChain,
	statedb *state.StateDB, cfg vm.Config, utxoSet *UtxoSet,
) (*types.Receipt, uint64, uint64, []*types.Transaction, error) {

	var txs []*types.Transaction
	ctx := NewEVMContext(tx, header, bc)
	vmenv := vm.NewEVM(ctx, statedb, cfg)
	//logger.Infof("ApplyMessage tx: %+v, header: %+v", tx, header)
	ret, gasUsed, gasRemainingFee, fail, gasRefundTx, err := ApplyMessage(vmenv, tx)
	if !fail && err != nil {
		logger.Errorf("Failed to apply message with tx: %+v", tx)
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
	deployed := tx.Type() == types.ContractCreationType
	if !fail {
		addNewContractUtxos(vmenv.ContractCreatedItems, utxoSet, statedb, header.Height)
	}
	if deployed {
		contractAddr = types.CreateAddress(*tx.From(), tx.Nonce())
		// if contract address had some boxes in normal utxo in pay2pubk txs,
		// move the value to contract utxo
		if !fail {
			origBal := statedb.GetBalance(*contractAddr).Uint64() - tx.Value().Uint64()
			outPoint := types.NewOutPoint(types.NormalizeAddressHash(contractAddr), 0)
			utxoSet.utxoMap[*outPoint].AddValue(origBal)
			logger.Infof("contract %x origBal: %d, new utxo wrap value: %d",
				contractAddr[:], origBal, utxoSet.utxoMap[*outPoint].Value())
		}
	} else {
		contractAddr = tx.To()
	}
	senderNonce := statedb.GetNonce(*tx.From())
	// handle utxo of contract created newly
	if !fail && len(vmenv.Transfers) > 0 {
		// create internal txs
		internalTxs, err := createUtxoTx(vmenv.Transfers, senderNonce, utxoSet, bc.db, statedb)
		if err != nil {
			logger.Error(err)
			return nil, 0, 0, nil, err
		}
		txs = append(txs, internalTxs...)
	} else if fail && tx.Value().Uint64() > 0 { // tx failed
		internalTxs, err := createRefundTx(tx, senderNonce, utxoSet, contractAddr)
		if err != nil {
			logger.Error(err)
			return nil, 0, 0, nil, err
		}
		txs = append(txs, internalTxs)
	}
	txhash := tx.OriginTxHash()
	logs := statedb.GetLogs(*txhash)
	logsBytes, _ := json.Marshal(logs)
	logger.Infof("tx %s contract logs: %s", txhash, string(logsBytes))
	var errMsg string
	if fail && len(ret) != 0 {
		errMsg = string(ret)
	}
	receipt := types.NewReceipt(tx.OriginTxHash(), contractAddr, deployed, fail,
		gasUsed, errMsg, statedb.GetLogs(*txhash))
	// append internal txs hashes to receipt
	internalHashes := make([]*crypto.HashType, 0, len(txs))
	for _, tx := range txs {
		hash, _ := tx.TxHash()
		internalHashes = append(internalHashes, hash)
	}
	receipt.ApppendInternalTxs(internalHashes...)

	return receipt, gasUsed, gasRemainingFee, txs, nil
}

func createUtxoTx(
	transfers map[types.AddressHash][]*vm.TransferInfo, senderNonce uint64,
	utxoSet *UtxoSet, db storage.Reader, statedb *state.StateDB,
) ([]*types.Transaction, error) {

	var txs []*types.Transaction
	for _, v := range transfers {
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
				tx, err := makeTx(v, senderNonce, begin, end, utxoSet, db, statedb)
				if err != nil {
					logger.Error("create utxo tx error: ", err)
					return nil, err
				}
				txs = append(txs, tx)
			}
		} else {
			tx, err := makeTx(v, senderNonce, 0, len(v), utxoSet, db, statedb)
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
	vmtx *types.VMTransaction, senderNonce uint64, utxoSet *UtxoSet,
	contractAddr *types.AddressHash,
) (*types.Transaction, error) {

	hash := types.NormalizeAddressHash(contractAddr)
	outPoint := *types.NewOutPoint(hash, 0)
	// subtract value from contract
	deploy := vmtx.Type() == types.ContractCreationType
	if !deploy {
		utxoWrap, ok := utxoSet.utxoMap[outPoint]
		if !ok {
			return nil, fmt.Errorf("make refund tx error: contract %x utxo does not exist",
				contractAddr[:])
		}
		if utxoWrap.Value() < vmtx.Value().Uint64() {
			return nil, fmt.Errorf("make refund tx error: %x balance: %d, need refund: %d",
				contractAddr[:], utxoWrap.Value(), vmtx.Value().Uint64())
		}
		value := utxoWrap.Value() - vmtx.Value().Uint64()
		utxoWrap.SetValue(value)
		utxoSet.utxoMap[outPoint] = utxoWrap
	}
	// make tx
	vin := &types.TxIn{
		PrevOutPoint: outPoint,
		ScriptSig:    *script.MakeContractScriptSig(senderNonce),
	}
	vout := &types.TxOut{
		Value:        vmtx.Value().Uint64(),
		ScriptPubKey: *script.PayToPubKeyHashScript(vmtx.From().Bytes()),
	}
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vin)
	tx.Vout = append(tx.Vout, vout)
	return tx, nil
}

func makeTx(
	transferInfos []*vm.TransferInfo, senderNonce uint64, voutBegin int, voutEnd int,
	utxoSet *UtxoSet, db storage.Reader, statedb *state.StateDB,
) (*types.Transaction, error) {

	from := &transferInfos[0].From
	hash := types.NormalizeAddressHash(from)
	if hash.IsEqual(&zeroHash) {
		return nil, errors.New("makeTx] Invalid contract address for from")
	}
	inOp := types.NewOutPoint(hash, 0)
	utxoSet.contractUtxos[*inOp] = struct{}{}
	fromUtxoWrap, ok := utxoSet.utxoMap[*inOp]
	if !ok {
		wrap, err := fetchUtxoWrapFromDB(db, inOp)
		if err != nil || wrap == nil {
			return nil, fmt.Errorf("makeTx] fetch from contract %x utxo error: %v or nil", from[:], err)
		}
		utxoSet.utxoMap[*inOp] = wrap
		fromUtxoWrap = wrap
	}
	var vouts []*types.TxOut
	for i := voutBegin; i < voutEnd; i++ {
		info := transferInfos[i]
		to := &info.To
		if *to == types.ZeroAddressHash {
			return nil, fmt.Errorf("makeTx] to contract is zero address")
		}
		var spk *script.Script
		if statedb.IsContractAddr(*to) {
			spk, _ = script.MakeContractScriptPubkey(from, to, 1, 0, 0)
			outOp := types.NewOutPoint(types.NormalizeAddressHash(to), 0)
			utxoSet.contractUtxos[*outOp] = struct{}{}
			// update contract utxowrap in utxoSet
			if _, ok := utxoSet.utxoMap[*outOp]; !ok {
				wrap := GetContractUtxoFromStateDB(statedb, to)
				if wrap == nil {
					return nil, fmt.Errorf("makeTx] fetch utxo of contract to %x return nil", to[:])
				}
				utxoSet.utxoMap[*outOp] = wrap
			}
			utxoSet.utxoMap[*outOp].AddValue(info.Value)
		} else {
			spk = script.PayToPubKeyHashScript(to.Bytes())
		}
		toValue := info.Value
		vout := types.NewTxOut(toValue, *spk)
		vouts = append(vouts, vout)
		fromValue := fromUtxoWrap.Value() - toValue
		if fromValue > fromUtxoWrap.Value() {
			return nil, fmt.Errorf("makeTx] balance of from %x is insufficient,"+
				" now %d, need %d to %s", from[:], fromUtxoWrap.Value(), toValue, to[:])
		}
		fromUtxoWrap.SetValue(fromValue)
	}
	tx := types.NewTx(0, 0, 0).
		AppendVin(types.NewTxIn(inOp, *script.MakeContractScriptSig(senderNonce), 0)).
		AppendVout(vouts...)
	return tx, nil
}

func addNewContractUtxos(
	items []*vm.ContractCreatedItem, utxoSet *UtxoSet, statedb *state.StateDB, height uint32,
) {
	for _, item := range items {
		op := types.NewOutPoint(types.NormalizeAddressHash(&item.Address), 0)
		utxoSet.contractUtxos[*op] = struct{}{}
		if utxoWrap, ok := utxoSet.utxoMap[*op]; ok {
			// maybe utxo exist already in utxo because other tx that refer to this contract in the same block
			logger.Infof("create contract utxo %v: %s added", op, utxoWrap)
			utxoWrap.AddValue(item.Value)
			continue
		}

		scriptPubk, _ := script.MakeContractScriptPubkey(&item.Caller, &item.Address,
			1, item.Nonce, 0)
		value := item.Value
		// new contract in contract
		if statedb.IsContractAddr(item.Caller) {
			// value is set when creating internal txs: createUtxoTxs
			value = 0
		}
		utxoWrap := types.NewUtxoWrap(value, *scriptPubk, height)
		utxoSet.utxoMap[*op] = utxoWrap
		logger.Infof("create contract utxo %v: %s", op, utxoWrap)
	}
}

// ExtractVMTransaction extract Transaction to VMTransaction
func ExtractVMTransaction(tx *types.Transaction, stateDB *state.StateDB) (*types.VMTransaction, error) {
	txHash, _ := tx.TxHash()
	// check
	if txlogic.GetTxType(tx, stateDB) != types.ContractTx {
		return nil, nil
	}
	// take only one contract vout in a transaction
	contractVout := txlogic.GetContractVout(tx, stateDB)
	p, t, e := script.NewScriptFromBytes(contractVout.ScriptPubKey).ParseContractParams()
	if e != nil {
		return nil, e
	}
	var input []byte
	if tx.Data != nil && len(tx.Data.Content) != 0 {
		input = tx.Data.Content
	}
	gasPrice := core.FixedGasPrice
	if IsCoinBase(tx) || IsInternalContract(tx) {
		gasPrice = 0
	}
	vmTx := types.NewVMTransaction(big.NewInt(int64(contractVout.Value)),
		p.GasLimit, gasPrice, p.Nonce, txHash, t, input).WithFrom(p.From)
	if t == types.ContractCallType {
		vmTx.WithTo(p.To)
	}
	return vmTx, nil
}
