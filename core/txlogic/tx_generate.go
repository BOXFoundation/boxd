// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"errors"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/pb"
	acc "github.com/BOXFoundation/boxd/wallet/account"
)

//
var (
	ErrInsufficientBalance = errors.New("insufficient account balance")
)

// NewTxWithUtxos new a transaction
func NewTxWithUtxos(
	fromAcc *acc.Account, utxos []*rpcpb.Utxo, toAddrs []string,
	amounts []uint64, changeAmt uint64,
) (*types.Transaction, *rpcpb.Utxo, error) {
	tx, err := MakeUnsignedTx(fromAcc.Addr(), toAddrs, amounts, changeAmt, utxos...)
	if err != nil {
		return nil, nil, err
	}
	// sign vin
	if err := SignTxWithUtxos(tx, utxos, fromAcc); err != nil {
		return nil, nil, err
	}
	// create change utxo
	var change *rpcpb.Utxo
	if changeAmt > 0 {
		txHash, _ := tx.TxHash()
		change = &rpcpb.Utxo{
			OutPoint:    NewPbOutPoint(txHash, uint32(len(tx.Vout))-1),
			TxOut:       MakeVout(fromAcc.Addr(), changeAmt),
			BlockHeight: 0,
			IsCoinbase:  false,
			IsSpent:     false,
		}
	}

	return tx, change, nil
}

// NewIssueTokenTxWithUtxos new issue token tx with utxos
func NewIssueTokenTxWithUtxos(
	fromAcc *acc.Account, utxos []*rpcpb.Utxo, to string, tag *rpcpb.TokenTag,
	changeAmt uint64,
) (*types.Transaction, *TokenID, *rpcpb.Utxo, error) {

	tx, issueOutIndex, err := MakeUnsignedTokenIssueTx(fromAcc.Addr(), to, tag,
		changeAmt, utxos...)
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
		change = &rpcpb.Utxo{
			OutPoint:    NewPbOutPoint(txHash, uint32(len(tx.Vout))-1),
			TxOut:       tx.Vout[len(tx.Vout)-1],
			BlockHeight: 0,
			IsCoinbase:  false,
			IsSpent:     false,
		}
	}
	return tx, NewTokenID(txHash, issueOutIndex), change, nil
}

// NewSplitAddrTxWithUtxos new split address tx
func NewSplitAddrTxWithUtxos(
	acc *acc.Account, addrs []string, weights []uint64, utxos []*rpcpb.Utxo, fee uint64,
) (tx *types.Transaction, change *rpcpb.Utxo, splitAddr string, err error) {

	// calc change amount
	utxoValue := uint64(0)
	for _, u := range utxos {
		utxoValue += u.GetTxOut().GetValue()
	}
	changeAmt := utxoValue - fee
	// make unsigned split addr tx
	tx, splitAddr, err = MakeUnsignedSplitAddrTx(acc.Addr(), addrs, weights,
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
		change = &rpcpb.Utxo{
			OutPoint:    NewPbOutPoint(txHash, uint32(len(tx.Vout))-1),
			TxOut:       tx.Vout[len(tx.Vout)-1],
			BlockHeight: 0,
			IsCoinbase:  false,
			IsSpent:     false,
		}
	}
	return
}

// MakeUnsignedTx make a tx without signature
func MakeUnsignedTx(
	from string, to []string, amounts []uint64, changeAmt uint64, utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {

	if !checkAmount(amounts, changeAmt, utxos...) {
		return nil, ErrInsufficientBalance
	}

	// vin
	vins := make([]*types.TxIn, 0, len(utxos))
	for _, utxo := range utxos {
		vins = append(vins, MakeVin(utxo, 0))
	}

	// vout for toAddrs
	vouts := make([]*corepb.TxOut, 0, len(to))
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

// MakeUnsignedSplitAddrTx make unsigned split addr tx
func MakeUnsignedSplitAddrTx(
	from string, addrs []string, weights []uint64, changeAmt uint64, utxos ...*rpcpb.Utxo,
) (*types.Transaction, string, error) {

	if !checkAmount(nil, changeAmt, utxos...) {
		return nil, "", ErrInsufficientBalance
	}
	// vin
	vins := make([]*types.TxIn, 0)
	for _, utxo := range utxos {
		vins = append(vins, MakeVin(utxo, 0))
	}
	// vout for toAddrs
	splitAddrOut := MakeSplitAddrVout(addrs, weights)
	// construct transaction
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, splitAddrOut)
	// change
	var changeOut *corepb.TxOut
	if changeAmt > 0 {
		changeOut = MakeVout(from, changeAmt)
		tx.Vout = append(tx.Vout, changeOut)
	}
	// calc split addr
	addr, err := MakeSplitAddr(addrs, weights)
	//
	return tx, addr, err
}

// MakeUnsignedTokenIssueTx make unsigned token issue tx
func MakeUnsignedTokenIssueTx(
	issuer string, issuee string, tag *rpcpb.TokenTag, changeAmt uint64,
	utxos ...*rpcpb.Utxo,
) (*types.Transaction, uint32, error) {

	if !checkAmount(nil, changeAmt, utxos...) {
		return nil, 0, ErrInsufficientBalance
	}
	// vin
	vins := make([]*types.TxIn, 0)
	for _, utxo := range utxos {
		vins = append(vins, MakeVin(utxo, 0))
	}
	// vout for toAddrs
	issueOut, err := MakeIssueTokenVout(issuee, tag)
	if err != nil {
		return nil, 0, err
	}
	// construct transaction
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, issueOut)
	// change
	var changeOut *corepb.TxOut
	if changeAmt > 0 {
		changeOut = MakeVout(issuer, changeAmt)
		tx.Vout = append(tx.Vout, changeOut)
	}
	// issue token vout is set to 0 defaultly
	return tx, 0, err
}

func checkAmount(amounts []uint64, changeAmt uint64, utxos ...*rpcpb.Utxo) bool {
	utxoValue := uint64(0)
	for _, u := range utxos {
		amount, tid, err := ParseUtxoAmount(u)
		if err != nil || tid != nil {
			continue
		}
		utxoValue += amount
	}
	amount := uint64(0)
	for _, a := range amounts {
		amount += a
	}
	return utxoValue >= amount+changeAmt
}
