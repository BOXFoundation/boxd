// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"errors"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	acc "github.com/BOXFoundation/boxd/wallet/account"
)

//
var (
	ErrInsufficientInput      = errors.New("insufficient input amount")
	ErrInsufficientTokenInput = errors.New("insufficient input token amount")
	ErrInvalidArguments       = errors.New("invalid arguments")
)

// NewTxWithUtxos new a transaction
func NewTxWithUtxos(
	fromAcc *acc.Account, toAddrs []*types.AddressHash, utxos []*rpcpb.Utxo, amounts []uint64,
) (*types.Transaction, *rpcpb.Utxo, error) {
	tx, err := MakeUnsignedTx(fromAcc.AddressHash(), toAddrs, amounts, utxos...)
	if err != nil {
		return nil, nil, err
	}
	// sign vin
	if err := SignTxWithUtxos(tx, utxos, fromAcc); err != nil {
		return nil, nil, err
	}
	// change
	var change *rpcpb.Utxo
	changeAmt, _ := calcChangeAmount(amounts, utxos...)
	if changeAmt > 0 {
		txHash, _ := tx.TxHash()
		idx := uint32(len(tx.Vout)) - 1
		op, uw := types.NewOutPoint(txHash, idx), NewUtxoWrap(fromAcc.AddressHash(),
			0, changeAmt)
		change = MakePbUtxo(op, uw)
	}
	//
	return tx, change, nil
}

// NewSplitAddrTxWithUtxos new split address tx
func NewSplitAddrTxWithUtxos(
	acc *acc.Account, addrs []*types.AddressHash, weights []uint32, utxos []*rpcpb.Utxo,
) (tx *types.Transaction, change *rpcpb.Utxo, err error) {

	if len(addrs) != len(weights) {
		err = ErrInvalidArguments
		return
	}
	// make unsigned split addr tx
	tx, err = MakeUnsignedSplitAddrTx(acc.AddressHash(), addrs, weights, utxos...)
	if err != nil {
		return
	}
	// sign vin
	if err = SignTxWithUtxos(tx, utxos, acc); err != nil {
		return
	}
	// create change utxo
	changeAmt, _ := calcChangeAmount(nil, utxos...)
	if changeAmt > 0 {
		txHash, _ := tx.TxHash()
		idx := uint32(len(tx.Vout)) - 1
		op, uw := types.NewOutPoint(txHash, idx), NewUtxoWrap(acc.AddressHash(), 0, changeAmt)
		change = MakePbUtxo(op, uw)
	}
	return
}

// NewTokenIssueTxWithUtxos new token issue tx with utxos
func NewTokenIssueTxWithUtxos(
	fromAcc *acc.Account, to *types.AddressHash, tag *rpcpb.TokenTag,
	utxos ...*rpcpb.Utxo,
) (*types.Transaction, *types.TokenID, *rpcpb.Utxo, error) {

	tx, issueOutIndex, err := MakeUnsignedTokenIssueTx(fromAcc.AddressHash(), to,
		tag, utxos...)
	if err != nil {
		return nil, nil, nil, err
	}
	// sign vin
	if err = SignTxWithUtxos(tx, utxos, fromAcc); err != nil {
		return nil, nil, nil, err
	}
	// create change utxo
	txHash, _ := tx.TxHash()
	var change *rpcpb.Utxo
	changeAmt, _ := calcChangeAmount(nil, utxos...)
	if changeAmt > 0 {
		txHash, _ := tx.TxHash()
		idx := uint32(len(tx.Vout)) - 1
		op, uw := types.NewOutPoint(txHash, idx), NewUtxoWrap(fromAcc.AddressHash(),
			0, changeAmt)
		change = MakePbUtxo(op, uw)
	}
	return tx, NewTokenID(txHash, issueOutIndex), change, nil
}

// NewTokenTransferTxWithUtxos new token Transfer tx with utxos
// it returns tx, box change and token change
func NewTokenTransferTxWithUtxos(
	fromAcc *acc.Account, to []*types.AddressHash, amounts []uint64, tid *types.TokenID,
	utxos ...*rpcpb.Utxo,
) (*types.Transaction, *rpcpb.Utxo, *rpcpb.Utxo, error) {
	if len(to) != len(amounts) {
		return nil, nil, nil, ErrInvalidArguments
	}
	// unsigned tx
	tx, err := MakeUnsignedTokenTransferTx(fromAcc.AddressHash(), to,
		amounts, tid, utxos...)
	if err != nil {
		return nil, nil, nil, err
	}
	// sign vin
	if err = SignTxWithUtxos(tx, utxos, fromAcc); err != nil {
		return nil, nil, nil, err
	}
	// change
	var (
		boxChange   *rpcpb.Utxo
		tokenChange *rpcpb.Utxo
		txHash      *crypto.HashType
	)
	changeAmt, _ := calcChangeAmount(nil, utxos...)
	tokenRemain, _ := calcTokenChangeAmount(tid, amounts, utxos...)
	if changeAmt > 0 || tokenRemain > 0 {
		txHash, _ = tx.TxHash()
	}
	if changeAmt > 0 {
		idx := uint32(len(tx.Vout)) - 1
		op, uw := types.NewOutPoint(txHash, idx), NewUtxoWrap(fromAcc.AddressHash(),
			0, changeAmt)
		boxChange = MakePbUtxo(op, uw)
	}
	if tokenRemain > 0 {
		idx := uint32(len(tx.Vout)) - 1
		if changeAmt > 0 {
			idx--
		}
		op := types.NewOutPoint(txHash, idx)
		uw, err := NewTokenUtxoWrap(fromAcc.AddressHash(), tid, 0, tokenRemain)
		if err != nil {
			return nil, nil, nil, err
		}
		tokenChange = MakePbUtxo(op, uw)
	}
	//
	return tx, boxChange, tokenChange, nil
}

// MakeUnsignedTx make a tx without signature
func MakeUnsignedTx(
	from *types.AddressHash, to []*types.AddressHash, amounts []uint64, utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {
	if len(to) != len(amounts) {
		return nil, ErrInvalidArguments
	}
	// calc change amount
	changeAmt, err := calcChangeAmount(amounts, utxos...)
	if err != nil {
		return nil, err
	}
	// vin
	vins := make([]*types.TxIn, 0, len(utxos))
	for _, utxo := range utxos {
		vins = append(vins, MakeVin(ConvPbOutPoint(utxo.OutPoint), 0))
	}
	// vout for toAddrs
	vouts := make([]*types.TxOut, 0, len(to))
	for i, addr := range to {
		vouts = append(vouts, MakeVout(addr, amounts[i]))
	}

	// construct transaction
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, vouts...)
	// change
	if changeAmt > 0 {
		tx.Vout = append(tx.Vout, MakeVout(from, changeAmt))
	}
	return tx, nil
}

func makeUnsignedContractTx(
	addr *types.AddressHash, amount, gasLimit uint64, contractVout *types.TxOut,
	byteCode []byte, utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {
	// calc change amount
	changeAmt, err := calcContractChangeAmount(gasLimit, utxos...)
	if err != nil {
		return nil, err
	}
	// create tx
	vins := make([]*types.TxIn, 0, len(utxos))
	for _, utxo := range utxos {
		vins = append(vins, MakeVin(ConvPbOutPoint(utxo.OutPoint), 0))
	}
	tx := new(types.Transaction).AppendVin(vins...).AppendVout(contractVout).
		WithData(types.ContractDataType, byteCode)
	if changeAmt > 0 {
		tx.Vout = append(tx.Vout, MakeVout(addr, changeAmt))
	}
	return tx, nil
}

//MakeUnsignedContractDeployTx make a contract tx without signature
func MakeUnsignedContractDeployTx(
	from *types.AddressHash, amount, gasLimit, nonce uint64, byteCode []byte,
	utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {
	contractVout, err := MakeContractCreationVout(from, amount, gasLimit, nonce)
	if err != nil {
		return nil, err
	}
	return makeUnsignedContractTx(from, amount, gasLimit, contractVout, byteCode, utxos...)
}

//MakeUnsignedContractCallTx call a contract tx without signature
func MakeUnsignedContractCallTx(
	from, to *types.AddressHash, amount, gasLimit, nonce uint64, byteCode []byte,
	utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {
	contractVout, err := MakeContractCallVout(from, to, amount, gasLimit, nonce)
	if err != nil {
		return nil, err
	}
	return makeUnsignedContractTx(from, amount, gasLimit, contractVout, byteCode, utxos...)
}

// NewContractTxWithUtxos new a contract transaction
func NewContractTxWithUtxos(
	fromAcc *acc.Account, contractAddr *types.AddressHash, amount, gasLimit,
	nonce uint64, byteCode []byte, utxos ...*rpcpb.Utxo,
) (*types.Transaction, *rpcpb.Utxo, error) {
	tx, err := MakeUnsignedContractCallTx(fromAcc.AddressHash(), contractAddr,
		amount, gasLimit, nonce, byteCode, utxos...)
	if err != nil {
		return nil, nil, err
	}
	// sign vin
	if err := SignTxWithUtxos(tx, utxos, fromAcc); err != nil {
		return nil, nil, err
	}
	// change
	var change *rpcpb.Utxo
	changeAmt, _ := calcContractChangeAmount(gasLimit, utxos...)
	if changeAmt > 0 {
		txHash, _ := tx.TxHash()
		idx := uint32(len(tx.Vout)) - 1
		op, uw := types.NewOutPoint(txHash, idx), NewUtxoWrap(fromAcc.AddressHash(),
			0, changeAmt)
		change = MakePbUtxo(op, uw)
	}
	//
	return tx, change, nil
}

// MakeUnsignedSplitAddrTx make unsigned split addr tx
func MakeUnsignedSplitAddrTx(
	from *types.AddressHash, addrs []*types.AddressHash, weights []uint32,
	utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {
	// check
	if len(addrs) != len(weights) {
		return nil, ErrInvalidArguments
	}
	// calc change amount
	changeAmt, err := calcChangeAmount(nil, utxos...)
	if err != nil {
		return nil, err
	}
	// vin
	vins := make([]*types.TxIn, 0)
	for _, utxo := range utxos {
		vins = append(vins, MakeVin(ConvPbOutPoint(utxo.OutPoint), 0))
	}
	// vout for toAddrs
	splitAddrOut := MakeSplitAddrVout(addrs, weights)
	// construct transaction
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, splitAddrOut)
	// change
	if changeAmt > 0 {
		tx.Vout = append(tx.Vout, MakeVout(from, changeAmt))
	}
	//
	return tx, nil
}

// MakeUnsignedTokenIssueTx make unsigned token issue tx
func MakeUnsignedTokenIssueTx(
	issuer, owner *types.AddressHash, tag *rpcpb.TokenTag, utxos ...*rpcpb.Utxo,
) (*types.Transaction, uint32, error) {

	// calc change amount
	changeAmt, err := calcChangeAmount(nil, utxos...)
	if err != nil {
		return nil, 0, err
	}
	// vin
	vins := make([]*types.TxIn, 0)
	for _, utxo := range utxos {
		vins = append(vins, MakeVin(ConvPbOutPoint(utxo.OutPoint), 0))
	}
	// vout for toAddrs
	issueOut := MakeIssueTokenVout(owner, tag)
	// construct transaction
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, issueOut)
	// change
	if changeAmt > 0 {
		tx.Vout = append(tx.Vout, MakeVout(issuer, changeAmt))
	}
	// issue token vout is set to 0 defaultly
	return tx, 0, nil
}

// MakeUnsignedTokenTransferTx make unsigned token transfer tx
func MakeUnsignedTokenTransferTx(
	from *types.AddressHash, to []*types.AddressHash, amounts []uint64,
	tid *types.TokenID, utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {
	// check
	if len(to) != len(amounts) {
		return nil, ErrInvalidArguments
	}
	// calc change amount
	changeAmt, err := calcChangeAmount(nil, utxos...)
	if err != nil {
		return nil, err
	}
	tokenRemain, err := calcTokenChangeAmount(tid, amounts, utxos...)
	if err != nil {
		return nil, err
	}
	// vin
	vins := make([]*types.TxIn, 0)
	for _, utxo := range utxos {
		vins = append(vins, MakeVin(ConvPbOutPoint(utxo.OutPoint), 0))
	}
	// vout
	vouts := make([]*types.TxOut, 0)
	for i, addr := range to {
		o, err := MakeTokenVout(addr, tid, amounts[i])
		if err != nil {
			return nil, err
		}
		vouts = append(vouts, o)
	}
	// vout for token change
	if tokenRemain > 0 {
		o, err := MakeTokenVout(from, tid, tokenRemain)
		if err != nil {
			return nil, err
		}
		vouts = append(vouts, o)
	}
	// vout for box change
	if changeAmt > 0 {
		vouts = append(vouts, MakeVout(from, changeAmt))
	}
	// construct transaction
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, vouts...)

	return tx, nil
}

func calcChangeAmount(toAmounts []uint64, utxos ...*rpcpb.Utxo) (uint64, error) {
	inputAmt := uint64(0)
	for _, u := range utxos {
		inputAmt += u.GetTxOut().GetValue()
	}
	toAmount := uint64(0)
	for _, v := range toAmounts {
		toAmount += v
	}
	fee := uint64((len(utxos)+len(toAmounts)+1)/core.InOutNumPerExtraFee+1) * core.TransferFee
	changeAmt := inputAmt - toAmount - fee
	if changeAmt > inputAmt {
		return 0, ErrInsufficientInput
	}
	return changeAmt, nil
}

func calcContractChangeAmount(gasLimit uint64, utxos ...*rpcpb.Utxo) (uint64, error) {
	inputAmt := uint64(0)
	for _, u := range utxos {
		inputAmt += u.GetTxOut().GetValue()
	}
	// 2 stands for contract vout and change vout
	extraFee := uint64((len(utxos)+2)/core.InOutNumPerExtraFee) * core.TransferFee
	changeAmt := inputAmt - gasLimit*core.FixedGasPrice - extraFee
	if changeAmt > inputAmt {
		return 0, ErrInsufficientInput
	}
	return changeAmt, nil
}

func calcTokenChangeAmount(
	tid *types.TokenID, toAmounts []uint64, utxos ...*rpcpb.Utxo,
) (uint64, error) {
	inputAmt := uint64(0)
	for _, u := range utxos {
		v, id, err := ParseUtxoAmount(u)
		if err != nil {
			return 0, err
		}
		if id == nil || *id != *tid {
			continue
		}
		inputAmt += v
	}
	toAmount := uint64(0)
	for _, v := range toAmounts {
		toAmount += v
	}
	changeAmt := inputAmt - toAmount
	if changeAmt > inputAmt {
		return 0, ErrInsufficientTokenInput
	}
	return changeAmt, nil
}
