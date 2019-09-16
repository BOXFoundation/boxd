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
	"github.com/BOXFoundation/boxd/util/bloom"
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
		vmTx, err1 := ExtractVMTransaction(tx)
		if err1 != nil {
			err = err1
			invalidTx = tx
			break
		}
		if vmTx == nil {
			sumGas += core.TransferFee
			continue
		}
		if vmTx.Nonce() != stateDB.GetNonce(*vmTx.From())+1 {
			err = fmt.Errorf("incorrect nonce(%d, %d in statedb) in tx: %s",
				vmTx.Nonce(), stateDB.GetNonce(*vmTx.From()), vmTx.OriginTxHash())
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
		gasThisTx := vmTx.GasPrice().Uint64() * gasUsedPerTx
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

	return receipts, usedGas, utxoTxs, nil
}

func (sp *StateProcessor) notifyInvalidTx(tx *types.Transaction) {
	sp.bc.bus.Publish(eventbus.TopicInvalidTx, tx)
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
	deployed := tx.Type() == types.ContractCreationType
	if deployed {
		contractAddr = types.CreateAddress(*tx.From(), tx.Nonce())
	} else {
		contractAddr = tx.To()
	}
	senderNonce := statedb.GetNonce(*tx.From())
	if !fail && len(Transfers) > 0 {
		internalTxs, err := createUtxoTx(senderNonce, utxoSet, bc.contractAddrFilter, bc.db)
		if err != nil {
			logger.Warn(err)
			return nil, 0, 0, nil, err
		}
		txs = append(txs, internalTxs...)
	} else if fail && tx.Value().Uint64() > 0 { // tx failed
		internalTxs, err := createRefundTx(tx, senderNonce, utxoSet, contractAddr)
		if err != nil {
			logger.Warn(err)
			return nil, 0, 0, nil, err
		}
		txs = append(txs, internalTxs)
	}
	txhash := tx.OriginTxHash()
	logs := statedb.GetLogs(*txhash)
	logsBytes, _ := json.Marshal(logs)
	logger.Infof("tx %s contract logs: %s", txhash, string(logsBytes))
	receipt := types.NewReceipt(tx.OriginTxHash(), contractAddr, deployed, fail,
		gasUsed, statedb.GetLogs(*txhash))

	return receipt, gasUsed, gasRemainingFee, txs, nil
}

func createUtxoTx(
	senderNonce uint64, utxoSet *UtxoSet, contractAddrFilter bloom.Filter,
	db storage.Reader,
) ([]*types.Transaction, error) {

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
				tx, err := makeTx(v, senderNonce, begin, end, utxoSet, contractAddrFilter, db)
				if err != nil {
					logger.Error("create utxo tx error: ", err)
					return nil, err
				}
				txs = append(txs, tx)
			}
		} else {
			tx, err := makeTx(v, senderNonce, 0, len(v), utxoSet, contractAddrFilter, db)
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
	utxoWrap, ok := utxoSet.utxoMap[outPoint]
	if !ok {
		return nil, errors.New("contract utxo does not exist")
	}
	if utxoWrap.Value() < vmtx.Value().Uint64() {
		contractAddrB, _ := types.NewContractAddressFromHash(contractAddr[:])
		logger.Errorf("contractAddr %s balance: %d, vmtx value: %d",
			contractAddrB, utxoWrap.Value(), vmtx.Value().Uint64())
		return nil, errors.New("Insufficient balance of smart contract")
	}
	value := utxoWrap.Value() - vmtx.Value().Uint64()
	utxoWrap.SetValue(value)
	utxoSet.utxoMap[outPoint] = utxoWrap

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
	transferInfos []*TransferInfo, senderNonce uint64, voutBegin int, voutEnd int,
	utxoSet *UtxoSet, contractAddrFilter bloom.Filter, db storage.Reader,
) (*types.Transaction, error) {

	from := &transferInfos[0].from
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
			return nil, fmt.Errorf("makeTx] fetch from contract %x utxo error: %v or nil", from, err)
		}
		utxoSet.utxoMap[*inOp] = wrap
		fromUtxoWrap = wrap
	}
	var vouts []*types.TxOut
	for i := voutBegin; i < voutEnd; i++ {
		to := &transferInfos[i].to
		if *to == types.ZeroAddressHash {
			return nil, fmt.Errorf("makeTx] to contract is zero address")
		}
		var spk *script.Script
		if IsContractAddr(to, contractAddrFilter, db, utxoSet) {
			spk, _ = script.MakeContractScriptPubkey(from, to, 1, 0, 0)
			outOp := types.NewOutPoint(types.NormalizeAddressHash(to), 0)
			utxoSet.contractUtxos[*outOp] = struct{}{}
			// update contract utxowrap in utxoSet
			if _, ok := utxoSet.utxoMap[*outOp]; !ok {
				wrap, err := fetchUtxoWrapFromDB(db, outOp)
				if err != nil || wrap == nil {
					return nil, fmt.Errorf("makeTx] fetch to contract %x utxo error: %v or nil", to, err)
				}
				utxoSet.utxoMap[*outOp] = wrap
			}
		} else {
			spk = script.PayToPubKeyHashScript(to.Bytes())
		}
		toValue := transferInfos[i].value
		vout := types.NewTxOut(toValue, *spk)
		vouts = append(vouts, vout)
		fromValue := fromUtxoWrap.Value() - toValue
		if fromValue > fromUtxoWrap.Value() {
			return nil, fmt.Errorf("makeTx] balance of from %x is insufficient,"+
				" now %d, need %d to %s", from[:], fromUtxoWrap.Value(), toValue, to[:])
		}
		fromUtxoWrap.SetValue(fromValue)
	}
	vin := &types.TxIn{
		PrevOutPoint: *inOp,
		ScriptSig:    *script.MakeContractScriptSig(senderNonce),
	}
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vin)
	tx.Vout = append(tx.Vout, vouts...)
	return tx, nil
}

// ExtractVMTransaction extract Transaction to VMTransaction
func ExtractVMTransaction(
	tx *types.Transaction, ownerTxs ...*types.Transaction,
) (*types.VMTransaction, error) {
	// check
	contractVout, err := txlogic.CheckAndGetContractVout(tx)
	if err != nil {
		return nil, err
	}
	if contractVout == nil { // non-contract tx
		return nil, nil
	}
	txHash, _ := tx.TxHash()
	// take only one contract vout in a transaction
	p, t, e := script.NewScriptFromBytes(contractVout.ScriptPubKey).ParseContractParams()
	if e != nil {
		return nil, e
	}
	if tx.Data == nil || len(tx.Data.Content) == 0 {
		return nil, core.ErrContractDataNotFound
	}
	gasPrice := core.FixedGasPrice
	if IsCoinBase(tx) || IsDynastySwitch(tx) {
		gasPrice = 0
	}
	vmTx := types.NewVMTransaction(big.NewInt(int64(contractVout.Value)),
		p.GasLimit, gasPrice, p.Nonce, txHash, t, tx.Data.Content).WithFrom(p.From)
	if t == types.ContractCallType {
		vmTx.WithTo(p.To)
	}
	return vmTx, nil
}
