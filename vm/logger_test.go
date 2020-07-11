// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package vm

import (
	"math/big"
	"testing"

	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
)

type dummyContractRef struct {
	calledForEach bool
}

func (dummyContractRef) ReturnGas(*big.Int)              {}
func (dummyContractRef) Address() types.AddressHash      { return types.AddressHash{} }
func (dummyContractRef) Value() *big.Int                 { return new(big.Int) }
func (dummyContractRef) SetCode(crypto.HashType, []byte) {}
func (d *dummyContractRef) ForEachStorage(callback func(key, value crypto.HashType) bool) {
	d.calledForEach = true
}
func (d *dummyContractRef) SubBalance(amount *big.Int) {}
func (d *dummyContractRef) AddBalance(amount *big.Int) {}
func (d *dummyContractRef) SetBalance(*big.Int)        {}
func (d *dummyContractRef) SetNonce(uint64)            {}
func (d *dummyContractRef) Balance() *big.Int          { return new(big.Int) }

type dummyStatedb struct {
	state.Mockdb
}

func (*dummyStatedb) GetRefund() uint64                                         { return 1337 }
func (*dummyStatedb) AddLog(log *types.Log)                                     {}
func (*dummyStatedb) UpdateUtxo(addr types.AddressHash, utxoBytes []byte) error { return nil }
func (*dummyStatedb) THash() crypto.HashType                                    { return crypto.HashType{} }
func (*dummyStatedb) GetLogs(hash crypto.HashType) []*types.Log                 { return nil }
func (*dummyStatedb) Error() error                                              { return nil }
func (*dummyStatedb) SetError(error)                                            {}

func TestStoreCapture(t *testing.T) {
	var (
		env      = NewEVM(Context{}, &dummyStatedb{}, Config{})
		logger   = NewStructLogger(nil)
		mem      = NewMemory()
		stack    = newstack()
		contract = NewContract(&dummyContractRef{}, &dummyContractRef{}, new(big.Int), 0)
	)
	stack.push(big.NewInt(1))
	stack.push(big.NewInt(0))
	var index crypto.HashType
	logger.CaptureState(env, 0, SSTORE, 0, 0, mem, stack, newIntPool(), contract, 0, nil)
	if len(logger.changedValues[contract.Address()]) == 0 {
		t.Fatalf("expected exactly 1 changed value on address %x, got %d", contract.Address(), len(logger.changedValues[contract.Address()]))
	}
	exp := crypto.BigToHash(big.NewInt(1))
	if logger.changedValues[contract.Address()][index] != exp {
		t.Errorf("expected %x, got %x", exp, logger.changedValues[contract.Address()][index])
	}
}
