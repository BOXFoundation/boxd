// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package vm

import (
	coretypes "github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/vm/common/math"
	"github.com/BOXFoundation/boxd/vm/common/types"
)

// memoryGasCosts calculates the quadratic gas for memory expansion. It does so
// only for the memory region that is expanded, not the total memory.
func memoryGasCost(mem *Memory, newMemSize uint64) (uint64, error) {

	if newMemSize == 0 {
		return 0, nil
	}
	// The maximum that will fit in a uint64 is max_word_count - 1
	// anything above that will result in an overflow.
	// Additionally, a newMemSize which results in a
	// newMemSizeWords larger than 0x7ffffffff will cause the square operation
	// to overflow.
	// The constant 0xffffffffe0 is the highest number that can be used without
	// overflowing the gas calculation
	if newMemSize > 0xffffffffe0 {
		return 0, errGasUintOverflow
	}

	newMemSizeWords := toWordSize(newMemSize)
	newMemSize = newMemSizeWords * 32

	// fee = words * MemGas(3) + words^2 / 512 - mem.lastGasCost
	// why sub mem.lastGasCost?
	// because newMemSize is total memSize, we need cal incremental memory gas.
	if newMemSize > uint64(mem.Len()) {
		square := newMemSizeWords * newMemSizeWords
		linCoef := newMemSizeWords * types.MemoryGas
		quadCoef := square / types.QuadCoeffDiv
		newTotalFee := linCoef + quadCoef

		fee := newTotalFee - mem.lastGasCost
		mem.lastGasCost = newTotalFee

		return fee, nil
	}
	return 0, nil
}

func constGasFunc(gas uint64) gasFunc {
	return func(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
		return gas, nil
	}
}

func gasCallDataCopy(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}

	var overflow bool
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}

	words, overflow := bigUint64(stack.Back(2))
	if overflow {
		return 0, errGasUintOverflow
	}

	if words, overflow = math.SafeMul(toWordSize(words), types.CopyGas); overflow {
		return 0, errGasUintOverflow
	}

	if gas, overflow = math.SafeAdd(gas, words); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasReturnDataCopy(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}

	var overflow bool
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}

	words, overflow := bigUint64(stack.Back(2))
	if overflow {
		return 0, errGasUintOverflow
	}

	if words, overflow = math.SafeMul(toWordSize(words), types.CopyGas); overflow {
		return 0, errGasUintOverflow
	}

	if gas, overflow = math.SafeAdd(gas, words); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasSStore(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var (
		y, x    = stack.Back(1), stack.Back(0)
		current = evm.StateDB.GetState(contract.Address(), crypto.BigToHash(x))
	)
	// This checks for 3 scenario's and calculates gas accordingly:
	//
	// 1. From a zero-value address to a non-zero value         (NEW VALUE)
	// 2. From a non-zero value address to a zero-value address (DELETE)
	// 3. From a non-zero to a non-zero                         (CHANGE)
	switch {
	case current == (crypto.HashType{}) && y.Sign() != 0: // 0 => non 0
		return types.SstoreSetGas, nil
	case current != (crypto.HashType{}) && y.Sign() == 0: // non 0 => 0
		evm.StateDB.AddRefund(types.SstoreRefundGas)
		return types.SstoreClearGas, nil
	default: // non 0 => non 0 (or 0 => 0)
		return types.SstoreResetGas, nil
	}
}

func makeGasLog(n uint64) gasFunc {
	return func(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
		requestedSize, overflow := bigUint64(stack.Back(1))
		if overflow {
			return 0, errGasUintOverflow
		}

		gas, err := memoryGasCost(mem, memorySize)
		if err != nil {
			return 0, err
		}

		if gas, overflow = math.SafeAdd(gas, types.LogGas); overflow {
			return 0, errGasUintOverflow
		}
		if gas, overflow = math.SafeAdd(gas, n*types.LogTopicGas); overflow {
			return 0, errGasUintOverflow
		}

		var memorySizeGas uint64
		if memorySizeGas, overflow = math.SafeMul(requestedSize, types.LogDataGas); overflow {
			return 0, errGasUintOverflow
		}
		if gas, overflow = math.SafeAdd(gas, memorySizeGas); overflow {
			return 0, errGasUintOverflow
		}
		return gas, nil
	}
}

func gasSha3(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var overflow bool
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}

	if gas, overflow = math.SafeAdd(gas, types.Sha3Gas); overflow {
		return 0, errGasUintOverflow
	}

	wordGas, overflow := bigUint64(stack.Back(1))
	if overflow {
		return 0, errGasUintOverflow
	}
	if wordGas, overflow = math.SafeMul(toWordSize(wordGas), types.Sha3WordGas); overflow {
		return 0, errGasUintOverflow
	}
	if gas, overflow = math.SafeAdd(gas, wordGas); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasCodeCopy(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}

	var overflow bool
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}

	wordGas, overflow := bigUint64(stack.Back(2))
	if overflow {
		return 0, errGasUintOverflow
	}
	if wordGas, overflow = math.SafeMul(toWordSize(wordGas), types.CopyGas); overflow {
		return 0, errGasUintOverflow
	}
	if gas, overflow = math.SafeAdd(gas, wordGas); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasExtCodeCopy(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}

	var overflow bool
	if gas, overflow = math.SafeAdd(gas, gt.ExtcodeCopy); overflow {
		return 0, errGasUintOverflow
	}

	wordGas, overflow := bigUint64(stack.Back(3))
	if overflow {
		return 0, errGasUintOverflow
	}

	if wordGas, overflow = math.SafeMul(toWordSize(wordGas), types.CopyGas); overflow {
		return 0, errGasUintOverflow
	}

	if gas, overflow = math.SafeAdd(gas, wordGas); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasExtCodeHash(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return gt.ExtcodeHash, nil
}

func gasMLoad(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var overflow bool
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, errGasUintOverflow
	}
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasMStore8(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var overflow bool
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, errGasUintOverflow
	}
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasMStore(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var overflow bool
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, errGasUintOverflow
	}
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasCreate(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var overflow bool
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	if gas, overflow = math.SafeAdd(gas, types.CreateGas); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasCreate2(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var overflow bool
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	if gas, overflow = math.SafeAdd(gas, types.Create2Gas); overflow {
		return 0, errGasUintOverflow
	}
	wordGas, overflow := bigUint64(stack.Back(2))
	if overflow {
		return 0, errGasUintOverflow
	}
	if wordGas, overflow = math.SafeMul(toWordSize(wordGas), types.Sha3WordGas); overflow {
		return 0, errGasUintOverflow
	}
	if gas, overflow = math.SafeAdd(gas, wordGas); overflow {
		return 0, errGasUintOverflow
	}

	return gas, nil
}

func gasBalance(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return gt.Balance, nil
}

func gasExtCodeSize(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return gt.ExtcodeSize, nil
}

func gasSLoad(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return gt.SLoad, nil
}

func gasExp(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	expByteLen := uint64((stack.data[stack.len()-2].BitLen() + 7) / 8)

	var (
		gas      = expByteLen * gt.ExpByte // no overflow check required. Max is 256 * ExpByte gas
		overflow bool
	)
	if gas, overflow = math.SafeAdd(gas, GasSlowStep); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasCall(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var (
		gas            = gt.Calls
		transfersValue = stack.Back(2).Sign() != 0
		address        = coretypes.BigToAddressHash(stack.Back(1))
	)
	if transfersValue && evm.StateDB.Empty(address) {
		gas += types.CallNewAccountGas
	}
	if transfersValue {
		gas += types.CallValueTransferGas
	}
	memoryGas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	var overflow bool
	if gas, overflow = math.SafeAdd(gas, memoryGas); overflow {
		return 0, errGasUintOverflow
	}

	evm.callGasTemp, err = callGas(gt, contract.Gas, gas, stack.Back(0))
	if err != nil {
		return 0, err
	}
	if gas, overflow = math.SafeAdd(gas, evm.callGasTemp); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasCallCode(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas := gt.Calls
	if stack.Back(2).Sign() != 0 {
		gas += types.CallValueTransferGas
	}
	memoryGas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	var overflow bool
	if gas, overflow = math.SafeAdd(gas, memoryGas); overflow {
		return 0, errGasUintOverflow
	}

	evm.callGasTemp, err = callGas(gt, contract.Gas, gas, stack.Back(0))
	if err != nil {
		return 0, err
	}
	if gas, overflow = math.SafeAdd(gas, evm.callGasTemp); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasReturn(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return memoryGasCost(mem, memorySize)
}

func gasRevert(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return memoryGasCost(mem, memorySize)
}

func gasSuicide(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var gas uint64

	gas = gt.Suicide
	var (
		address = coretypes.BigToAddressHash(stack.Back(0))
	)
	// if empty and transfers value
	if evm.StateDB.Empty(address) && evm.StateDB.GetBalance(contract.Address()).Sign() != 0 {
		gas += gt.CreateBySuicide
	}

	if !evm.StateDB.HasSuicided(contract.Address()) {
		evm.StateDB.AddRefund(types.SuicideRefundGas)
	}
	return gas, nil
}

func gasDelegateCall(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	var overflow bool
	if gas, overflow = math.SafeAdd(gas, gt.Calls); overflow {
		return 0, errGasUintOverflow
	}

	evm.callGasTemp, err = callGas(gt, contract.Gas, gas, stack.Back(0))
	if err != nil {
		return 0, err
	}
	if gas, overflow = math.SafeAdd(gas, evm.callGasTemp); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasStaticCall(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	var overflow bool
	if gas, overflow = math.SafeAdd(gas, gt.Calls); overflow {
		return 0, errGasUintOverflow
	}

	evm.callGasTemp, err = callGas(gt, contract.Gas, gas, stack.Back(0))
	if err != nil {
		return 0, err
	}
	if gas, overflow = math.SafeAdd(gas, evm.callGasTemp); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasPush(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return GasFastestStep, nil
}

func gasSwap(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return GasFastestStep, nil
}

func gasDup(gt types.GasTable, evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return GasFastestStep, nil
}
