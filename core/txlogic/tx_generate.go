// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"errors"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	rpcpb "github.com/BOXFoundation/boxd/rpc/pb"
	acc "github.com/BOXFoundation/boxd/wallet/account"
)

//
var (
	ErrInsufficientBalance      = errors.New("insufficient balance")
	ErrInsufficientTokenBalance = errors.New("insufficient token balance")
	ErrInvalidArguments         = errors.New("invalid arguments")
)

// NewTxWithUtxos new a transaction
func NewTxWithUtxos(
	fromAcc *acc.Account, toAddrs []*types.AddressHash, utxos []*rpcpb.Utxo, amounts []uint64,
	changeAmt uint64,
) (*types.Transaction, *rpcpb.Utxo, error) {
	tx, err := MakeUnsignedTx(fromAcc.AddressHash(), toAddrs, amounts, changeAmt, utxos...)
	if err != nil {
		return nil, nil, err
	}
	// sign vin
	if err := SignTxWithUtxos(tx, utxos, fromAcc); err != nil {
		return nil, nil, err
	}
	// change
	var change *rpcpb.Utxo
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
	acc *acc.Account, addrs []*types.AddressHash, weights []uint32,
	utxos []*rpcpb.Utxo, fee uint64,
) (tx *types.Transaction, change *rpcpb.Utxo, err error) {

	if len(addrs) != len(weights) {
		err = ErrInvalidArguments
		return
	}
	// calc change amount
	utxoValue := uint64(0)
	for _, u := range utxos {
		utxoValue += u.GetTxOut().GetValue()
	}
	changeAmt := utxoValue - fee
	// make unsigned split addr tx
	tx, err = MakeUnsignedSplitAddrTx(acc.AddressHash(), addrs, weights,
		changeAmt, utxos...)
	if err != nil {
		return
	}
	// sign vin
	if err = SignTxWithUtxos(tx, utxos, acc); err != nil {
		return
	}
	// create change utxo
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
	fromAcc *acc.Account, to *types.AddressHash, tag *rpcpb.TokenTag, changeAmt uint64,
	utxos ...*rpcpb.Utxo,
) (*types.Transaction, *types.TokenID, *rpcpb.Utxo, error) {

	tx, issueOutIndex, err := MakeUnsignedTokenIssueTx(fromAcc.AddressHash(), to,
		tag, changeAmt, utxos...)
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
	changeAmt uint64, utxos ...*rpcpb.Utxo,
) (*types.Transaction, *rpcpb.Utxo, *rpcpb.Utxo, error) {

	if len(to) != len(amounts) {
		return nil, nil, nil, ErrInvalidArguments
	}
	// unsigned tx
	tx, tokenRemain, err := MakeUnsignedTokenTransferTx(fromAcc.AddressHash(), to,
		amounts, tid, changeAmt, utxos...)
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
	from *types.AddressHash, to []*types.AddressHash, amounts []uint64,
	changeAmt uint64, utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {

	if len(to) != len(amounts) {
		return nil, ErrInvalidArguments
	}

	if !checkAmount(amounts, changeAmt, utxos...) {
		return nil, ErrInsufficientBalance
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
	addr *types.AddressHash, amount, changeAmt uint64, contractVout *types.TxOut,
	byteCode []byte, utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {

	amounts := []uint64{amount}
	if !checkAmount(amounts, changeAmt, utxos...) {
		return nil, ErrInsufficientBalance
	}

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
	from *types.AddressHash, amount, changeAmt, gasLimit, gasPrice, nonce uint64,
	byteCode []byte, utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {

	contractVout, err := MakeContractCreationVout(from, amount,
		gasLimit, gasPrice, nonce)
	if err != nil {
		return nil, err
	}
	return makeUnsignedContractTx(from, amount, changeAmt, contractVout, byteCode, utxos...)
}

//MakeUnsignedContractCallTx call a contract tx without signature
func MakeUnsignedContractCallTx(
	from, to *types.AddressHash, amount, changeAmt, gasLimit, gasPrice, nonce uint64,
	byteCode []byte, utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {
	contractVout, err := MakeContractCallVout(from, to, amount, gasLimit, gasPrice, nonce)
	if err != nil {
		return nil, err
	}
	return makeUnsignedContractTx(from, amount, changeAmt, contractVout, byteCode, utxos...)
}

// NewContractTxWithUtxos new a contract transaction
func NewContractTxWithUtxos(
	fromAcc *acc.Account, contractAddr *types.AddressHash, amount, changeAmt,
	gasPrice, gasLimit, nonce uint64, byteCode []byte, utxos ...*rpcpb.Utxo,
) (*types.Transaction, *rpcpb.Utxo, error) {
	tx, err := MakeUnsignedContractCallTx(fromAcc.AddressHash(), contractAddr,
		amount, changeAmt, gasLimit, gasPrice, nonce, byteCode, utxos...)
	if err != nil {
		return nil, nil, err
	}
	// sign vin
	if err := SignTxWithUtxos(tx, utxos, fromAcc); err != nil {
		return nil, nil, err
	}
	// change
	var change *rpcpb.Utxo
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
	changeAmt uint64, utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {

	if len(addrs) != len(weights) {
		return nil, ErrInvalidArguments
	}
	if !checkAmount(nil, changeAmt, utxos...) {
		return nil, ErrInsufficientBalance
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
	issuer, owner *types.AddressHash, tag *rpcpb.TokenTag, changeAmt uint64,
	utxos ...*rpcpb.Utxo,
) (*types.Transaction, uint32, error) {

	if !checkAmount(nil, changeAmt, utxos...) {
		return nil, 0, ErrInsufficientBalance
	}
	// vin
	vins := make([]*types.TxIn, 0)
	for _, utxo := range utxos {
		vins = append(vins, MakeVin(ConvPbOutPoint(utxo.OutPoint), 0))
	}
	// vout for toAddrs
	issueOut, err := MakeIssueTokenVout(owner, tag)
	if err != nil {
		return nil, 0, err
	}
	// construct transaction
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, issueOut)
	// change
	if changeAmt > 0 {
		tx.Vout = append(tx.Vout, MakeVout(issuer, changeAmt))
	}
	// issue token vout is set to 0 defaultly
	return tx, 0, err
}

// MakeUnsignedTokenTransferTx make unsigned token transfer tx
func MakeUnsignedTokenTransferTx(
	from *types.AddressHash, to []*types.AddressHash, amounts []uint64,
	tid *types.TokenID, changeAmt uint64,
	utxos ...*rpcpb.Utxo,
) (*types.Transaction, uint64, error) {

	if len(to) != len(amounts) {
		return nil, 0, ErrInvalidArguments
	}
	ok, tokenRemain := checkTokenAmount(tid, amounts, changeAmt, utxos...)
	if !ok {
		return nil, 0, ErrInsufficientTokenBalance
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
			return nil, 0, err
		}
		vouts = append(vouts, o)
	}
	// vout for token change
	if tokenRemain > 0 {
		o, err := MakeTokenVout(from, tid, tokenRemain)
		if err != nil {
			return nil, 0, err
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

	return tx, tokenRemain, nil
}

func checkAmount(amounts []uint64, changeAmt uint64, utxos ...*rpcpb.Utxo) bool {
	utxoValue := uint64(0)
	for _, u := range utxos {
		amount, tid, err := ParseUtxoAmount(u)
		if err != nil {
			logger.Warn(err)
			continue
		}
		if tid != nil {
			logger.Warnf("have fetched un-relevant utxo: %+v, wanted non-token utxo", u)
		}
		utxoValue += amount
	}
	amount := uint64(0)
	for _, a := range amounts {
		amount += a
	}
	return utxoValue >= amount+changeAmt
}

func checkTokenAmount(
	tid *types.TokenID, amounts []uint64, changeAmt uint64, utxos ...*rpcpb.Utxo,
) (ok bool, tokenRemain uint64) {
	amt, tAmt := uint64(0), uint64(0)
	for _, u := range utxos {
		v, id, err := ParseUtxoAmount(u)
		if err != nil {
			logger.Warn(err)
			continue
		}
		if tid != nil && id != nil && *id == *tid {
			tAmt += v
		} else if id == nil {
			amt += v
		} else {
			logger.Warnf("have fetched un-relevant utxo: %+v, wanted token id: %+v", u, tid)
		}
	}
	amount := uint64(0)
	for _, a := range amounts {
		amount += a
	}
	return (amt >= changeAmt && tAmt >= amount), tAmt - amount
}
