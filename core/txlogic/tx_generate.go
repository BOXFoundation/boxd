// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package txlogic

import (
	"fmt"

	"github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/rpc/pb"
	acc "github.com/BOXFoundation/boxd/wallet/account"
)

// NewIssueTokenTxWithUtxos new issue token tx with utxos
func NewIssueTokenTxWithUtxos(
	fromAcc *acc.Account, utxos []*rpcpb.Utxo, to string,
	tag *types.TokenTag, supply uint64, changeAmt uint64) (
	*types.Transaction, *types.TokenID, *rpcpb.Utxo, error) {

	// check input and output amount
	utxoValue := uint64(0)
	for _, u := range utxos {
		utxoValue += u.GetTxOut().GetValue()
	}
	if utxoValue < changeAmt {
		return nil, nil, nil, fmt.Errorf("input %d is less than output %d",
			utxoValue, changeAmt)
	}
	// vin
	vins := make([]*types.TxIn, 0, len(utxos))
	for _, u := range utxos {
		vins = append(vins, MakeVin(u, 0))
	}
	// token vout 0
	tokenVout, err := MakeIssueTokenVout(to, tag, supply)
	if err != nil {
		return nil, nil, nil, err
	}
	// construct transaction
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, tokenVout)
	// vout for change of from
	var fromOut *corepb.TxOut
	if changeAmt > 0 {
		fromOut = MakeVout(fromAcc.Addr(), changeAmt)
		tx.Vout = append(tx.Vout, fromOut)
	}
	// sign vin
	if err := SignTxWithUtxos(tx, utxos, fromAcc); err != nil {
		return nil, nil, nil, err
	}
	// create change utxo
	txHash, _ := tx.TxHash()
	var change *rpcpb.Utxo
	if changeAmt > 0 {
		change = &rpcpb.Utxo{
			OutPoint:    NewPbOutPoint(txHash, uint32(len(tx.Vout))-1),
			TxOut:       fromOut,
			BlockHeight: 0,
			IsCoinbase:  false,
			IsSpent:     false,
		}
	}

	return tx, types.NewTokenID(txHash, 0), change, nil
}

// MakeUnsignedTx make a tx without signature
func MakeUnsignedTx(
	from string, to []string, amounts []uint64, changeAmt uint64, utxos ...*rpcpb.Utxo,
) (*types.Transaction, error) {
	utxoValue := uint64(0)
	for _, u := range utxos {
		amount, tid, err := ParseUtxoAmount(u)
		if err != nil || tid != nil {
			return nil, fmt.Errorf("error: %v or tid(%+v) is not nil", err, tid)
		}
		utxoValue += amount
	}
	amount := uint64(0)
	for _, a := range amounts {
		amount += a
	}
	if utxoValue < amount+changeAmt {
		return nil, fmt.Errorf("input %d is less than output %d",
			utxoValue, amount+changeAmt)
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

// NewSplitAddrTxWithUtxos new split address tx
func NewSplitAddrTxWithUtxos(
	acc *acc.Account, addrs []string, weights []uint64, utxos []*rpcpb.Utxo, fee uint64,
) (tx *types.Transaction, change *rpcpb.Utxo, splitAddr string, err error) {

	utxoValue := uint64(0)
	for _, u := range utxos {
		utxoValue += u.GetTxOut().GetValue()
	}
	changeAmt := utxoValue - fee

	// vin
	vins := make([]*types.TxIn, 0)
	for _, utxo := range utxos {
		vins = append(vins, MakeVin(utxo, 0))
	}

	// vout for toAddrs
	splitAddrOut := MakeSplitAddrVout(addrs, weights)

	// construct transaction
	tx = new(types.Transaction)
	tx.Vin = append(tx.Vin, vins...)
	tx.Vout = append(tx.Vout, splitAddrOut)
	// change
	var changeOut *corepb.TxOut
	if changeAmt > 0 {
		changeOut = MakeVout(acc.Addr(), changeAmt)
		tx.Vout = append(tx.Vout, changeOut)
	}

	// sign vin
	if err = SignTxWithUtxos(tx, utxos, acc); err != nil {
		return
	}

	// create change utxo
	if changeOut != nil {
		txHash, _ := tx.TxHash()
		change = &rpcpb.Utxo{
			OutPoint:    NewPbOutPoint(txHash, uint32(len(tx.Vout))-1),
			TxOut:       changeOut,
			BlockHeight: 0,
			IsCoinbase:  false,
			IsSpent:     false,
		}
	}

	splitAddr, err = MakeSplitAddr(addrs, weights)

	return
}
