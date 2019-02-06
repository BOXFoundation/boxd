// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletdata

import (
	"bytes"
	"fmt"

	"github.com/BOXFoundation/boxd/core/txlogic"

	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
)

// AddrProcessor is an interface for operating address utxo processor
type AddrProcessor interface {
	Addr() types.Address
	ApplyTransaction(tx *types.Transaction, height uint32)
	GetBalance() uint64
	RevertHeight(uint32)
	CreateSendTransaction([]byte, map[types.Address]uint64, crypto.Signer) (*types.Transaction, error)
}

// MemAddrProcessor holds utxo info of a certain address in memory
type MemAddrProcessor struct {
	addr       types.Address
	utxos      *chain.UtxoSet
	filter     []byte
	txs        map[crypto.HashType]*types.Transaction
	heightToTx map[uint32][]crypto.HashType
	caching    map[types.OutPoint]*types.UtxoWrap
	maxHeight  uint32
}

// NewMemAddrProcessor creates an instance of MemAddrProcessor
func NewMemAddrProcessor(addr types.Address) AddrProcessor {
	return &MemAddrProcessor{
		addr:       addr,
		utxos:      chain.NewUtxoSet(),
		filter:     *script.PayToPubKeyHashScript(addr.Hash()),
		txs:        make(map[crypto.HashType]*types.Transaction),
		heightToTx: make(map[uint32][]crypto.HashType),
		caching:    make(map[types.OutPoint]*types.UtxoWrap),
		maxHeight:  0,
	}
}

var _ AddrProcessor = (*MemAddrProcessor)(nil)

// Addr returns Address interface for target address
func (m *MemAddrProcessor) Addr() types.Address {
	return m.addr
}

// ApplyTransaction applies transaction of current address
func (m *MemAddrProcessor) ApplyTransaction(tx *types.Transaction, height uint32) {
	m.utxos.ApplyTxWithScriptFilter(tx, height, m.filter)
	hash, _ := tx.CalcTxHash()
	m.txs[*hash] = tx
	m.heightToTx[height] = append(m.heightToTx[height], *hash)
	m.maxHeight = height
	for _, vin := range tx.Vin {
		delete(m.caching, vin.PrevOutPoint)
	}
	for idx, vout := range tx.Vout {
		if bytes.HasPrefix(vout.ScriptPubKey, m.filter) {
			outpoint := types.OutPoint{
				Hash:  *hash,
				Index: uint32(idx),
			}
			delete(m.caching, outpoint)
		}
	}
}

// RevertHeight for current address
func (m *MemAddrProcessor) RevertHeight(targetHeight uint32) {
	for i := m.maxHeight; i >= targetHeight; i-- {
		if txHashes, ok := m.heightToTx[uint32(i)]; ok {
			for j := len(txHashes); j >= 0; j-- {
				tx, ok := m.txs[txHashes[j]]
				if !ok {
					continue
				}
				m.revertTx(tx, i)
			}
		}
	}
}

func (m *MemAddrProcessor) revertTx(tx *types.Transaction, height uint32) {
	hash, _ := tx.CalcTxHash()
	for idx := range tx.Vout {
		m.utxos.SpendUtxo(types.OutPoint{
			Hash:  *hash,
			Index: uint32(idx),
		})
	}
	for _, vin := range tx.Vin {
		previousTx, ok := m.txs[vin.PrevOutPoint.Hash]
		if !ok {
			// ignore used utxo if it's ot related to this address
			continue
		}
		prevTxOut := previousTx.Vout[vin.PrevOutPoint.Index]
		if bytes.HasPrefix(prevTxOut.ScriptPubKey, m.filter) {
			m.utxos.AddUtxo(tx, vin.PrevOutPoint.Index, height)
		}
	}
}

// GetFee returns recommended fee
func (m *MemAddrProcessor) GetFee() (uint64, error) {
	return 1, nil
}

// Fund returns utxo that can cover the required amount
func (m *MemAddrProcessor) Fund(fromAddr types.Address, amountRequired uint64) (map[types.OutPoint]*types.UtxoWrap, error) {
	if fromAddr.String() != m.addr.String() {
		return nil, fmt.Errorf("MemAddrProcessor address mismatch")
	}
	var current uint64
	out := make(map[types.OutPoint]*types.UtxoWrap)
	for o, w := range m.utxos.GetUtxos() {
		current += w.Value()
		out[o] = w
		if current >= amountRequired {
			return out, nil
		}
	}
	for o, w := range m.caching {
		current += w.Value()
		out[o] = w
		if current >= amountRequired {
			return out, nil
		}
	}
	return nil, fmt.Errorf("not enough balance")
}

// GetBalance returns total box amount of current address
func (m *MemAddrProcessor) GetBalance() (balance uint64) {
	for _, val := range m.utxos.GetUtxos() {
		balance += val.Value()
	}
	return
}

// CreateSendTransaction creates transaction using utxo in blockchain or in memory cache
func (m *MemAddrProcessor) CreateSendTransaction(pubKeyBytes []byte, targets map[types.Address]uint64, signer crypto.Signer) (*types.Transaction, error) {
	tx, err := txlogic.CreateTransaction(m, m.addr, targets, pubKeyBytes, signer)
	if err != nil {
		return nil, err
	}
	for _, vin := range tx.Vin {
		m.utxos.SpendUtxo(vin.PrevOutPoint)
		delete(m.caching, vin.PrevOutPoint)
	}
	hash, err := tx.TxHash()
	if err != nil {
		return nil, err
	}
	for idx, vout := range tx.Vout {
		if bytes.HasPrefix(vout.ScriptPubKey, m.filter) {
			wrap := types.NewUtxoWrap(vout.Value, vout.ScriptPubKey, 0)
			m.caching[types.OutPoint{Hash: *hash, Index: uint32(idx)}] = wrap
		}
	}
	return tx, nil
}
