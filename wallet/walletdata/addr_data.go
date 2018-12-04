// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package walletdata

import (
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
}

// MemAddrProcessor holds utxo info of a certain address in memory
type MemAddrProcessor struct {
	addr   types.Address
	utxos  *chain.UtxoSet
	filter []byte
	txs    map[crypto.HashType]*types.Transaction
}

// NewMemAddrProcessor creates an instance of MemAddrProcessor
func NewMemAddrProcessor(addr types.Address) AddrProcessor {
	return &MemAddrProcessor{
		addr:   addr,
		utxos:  chain.NewUtxoSet(),
		filter: *script.PayToPubKeyHashScript(addr.Hash()),
	}
}

var _ AddrProcessor = (*MemAddrProcessor)(nil)

// RevertHeight for current address
func (m *MemAddrProcessor) RevertHeight(uint32) {
	panic("implement me")
}

// Addr returns Address interface for target address
func (m *MemAddrProcessor) Addr() types.Address {
	return m.addr
}

// ApplyTransaction applies transaction of current address
func (m *MemAddrProcessor) ApplyTransaction(tx *types.Transaction, height uint32) {
	m.utxos.ApplyTxWithScriptFilter(tx, height, m.filter)

}

// GetBalance returns total box amount of current address
func (m *MemAddrProcessor) GetBalance() (balance uint64) {
	for _, val := range m.utxos.GetUtxos() {
		balance += val.Output.Value
	}
	return
}
