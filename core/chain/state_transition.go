// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"errors"
	"math"
	"math/big"

	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/vm"
)

// define const
const (
	TxGas                 uint64 = 2100
	TxGasContractCreation uint64 = 5300
	TxDataZeroGas         uint64 = 1
	TxDataNonZeroGas      uint64 = 6
)

var (
	errInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
)

// StateTransition the State Transitioning Model
type StateTransition struct {
	msg          types.Message
	gas          uint64
	gasPrice     *big.Int
	initialGas   uint64
	value        *big.Int
	data         []byte
	state        vm.StateDB
	evm          *vm.EVM
	gasRefoundTx *types.Transaction
	remaining    *big.Int
}

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, contractCreation bool) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if contractCreation {
		gas = TxGasContractCreation
	} else {
		gas = TxGas
	}
	// Bump the required gas by the amount of transactional data
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		// Make sure we don't exceed uint64 for all data combinations
		if (math.MaxUint64-gas)/TxDataNonZeroGas < nz {
			return 0, vm.ErrOutOfGas
		}
		gas += nz * TxDataNonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/TxDataZeroGas < z {
			return 0, vm.ErrOutOfGas
		}
		gas += z * TxDataZeroGas
	}
	return gas, nil
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(evm *vm.EVM, msg types.Message) *StateTransition {
	return &StateTransition{
		evm:       evm,
		msg:       msg,
		gasPrice:  msg.GasPrice(),
		value:     msg.Value(),
		data:      msg.Data(),
		state:     evm.StateDB,
		remaining: big.NewInt(0),
	}
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
func ApplyMessage(evm *vm.EVM, msg types.Message) ([]byte, uint64, uint64, bool, *types.Transaction, error) {
	return NewStateTransition(evm, msg).TransitionDb()
}

// to returns the recipient of the message.
func (st *StateTransition) to() types.AddressHash {
	if st.msg == nil || st.msg.To() == nil /* contract creation */ {
		return types.AddressHash{}
	}
	return *st.msg.To()
}

func (st *StateTransition) useGas(amount uint64) error {
	if st.gas < amount {
		logger.Warnf("state transition gas: %d, use gas: %d", st.gas, amount)
		return vm.ErrOutOfGas
	}
	st.gas -= amount

	return nil
}

func (st *StateTransition) buyGas() error {
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.gasPrice)
	if st.state.GetBalance(*st.msg.From()).Cmp(mgval) < 0 {
		logger.Warnf("bug gas error: balance for %s %d, need %d", st.msg.From(),
			st.state.GetBalance(*st.msg.From()), mgval)
		return errInsufficientBalanceForGas
	}
	st.gas += st.msg.Gas()

	st.initialGas = st.msg.Gas()
	st.state.SubBalance(*st.msg.From(), mgval)
	return nil
}

func (st *StateTransition) preCheck() error {
	// Make sure this transaction's nonce is correct.
	// if st.msg.CheckNonce() {
	// 	nonce := st.state.GetNonce(st.msg.From())
	// 	if nonce < st.msg.Nonce() {
	// 		return ErrNonceTooHigh
	// 	} else if nonce > st.msg.Nonce() {
	// 		return ErrNonceTooLow
	// 	}
	// }
	return st.buyGas()
}

// TransitionDb will transition the state by applying the current message and
// returning the result including the used gas.
// There are three scenarios for executing a tx:
// case 1: the tx is invalid before it is actually executed by evm. The tx is not on chain and returns an error.
// case 2: error returned after tx is executed by evm (the error is not an insufficient balance error). The tx is on chain but it's failed 	and the gas is used.
// case 3: the tx execution successful. The tx is on chain and it`s successful.
func (st *StateTransition) TransitionDb() (ret []byte, usedGas, gasRemaining uint64, failed bool, gasRefundTx *types.Transaction, err error) {
	if err = st.preCheck(); err != nil {
		return
	}
	msg := st.msg
	sender := vm.AccountRef(*msg.From())
	contractCreation := msg.Type() == types.ContractCreationType

	// Pay intrinsic gas
	gas, err := IntrinsicGas(st.data, contractCreation)
	if err != nil {
		logger.Warn(err)
		return nil, 0, 0, false, nil, err
	}
	logger.Infof("IntrinsicGas used gas: %d", gas)
	if err = st.useGas(gas); err != nil {
		logger.Warn(err)
		return nil, 0, 0, false, nil, err
	}

	var (
		evm  = st.evm
		addr types.AddressHash
		// vm errors do not effect consensus and are therefor
		// not assigned to err, except for insufficient balance
		// error.
		vmerr error
	)
	if contractCreation {
		//
		ret, addr, st.gas, vmerr = evm.Create(sender, st.data, st.gas, st.value, false)
	} else {
		// Increment the nonce for the next transaction
		st.state.SetNonce(*msg.From(), st.state.GetNonce(sender.Address())+1)
		ret, st.gas, vmerr = evm.Call(sender, st.to(), st.data, st.gas, st.value, false)
		logger.Infof("call contract return: %s", crypto.BytesToHash(ret))
	}
	if vmerr != nil {
		// log.Debug("VM returned with error", "err", vmerr)
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.
		logger.Warn(vmerr)
		if vmerr == vm.ErrInsufficientBalance {
			return nil, 0, 0, false, nil, vmerr
		}
	}
	if contractCreation {
		contractAddr, err := types.NewContractAddressFromHash(addr[:])
		if err != nil {
			logger.Error(err)
		}
		logger.Infof("contract address %s created", contractAddr)
	}
	st.refundGas()
	st.state.AddBalance(st.evm.Coinbase, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))

	logger.Infof("gasUsed: %d, remaining: %v", st.gasUsed(), st.remaining)
	return ret, st.gasUsed(), st.remaining.Uint64(), vmerr != nil, st.gasRefoundTx, nil
}

func (st *StateTransition) refundGas() {
	refund := st.gasUsed() / 2
	if refund > st.state.GetRefund() {
		refund = st.state.GetRefund()
	}
	st.gas += refund

	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.gasPrice)
	if remaining.Uint64() > 0 {
		nonce := st.state.GetNonce(*st.msg.From())
		st.gasRefoundTx = createGasRefundUtxoTx(st.msg.From(), remaining.Uint64(), nonce)
		st.remaining = remaining
	}
	st.state.AddBalance(*st.msg.From(), remaining)
}

func createGasRefundUtxoTx(addrHash *types.AddressHash, value, nonce uint64) *types.Transaction {

	addrScript := *script.PayToPubKeyHashScript(addrHash[:])
	vout := &corepb.TxOut{
		Value:        value,
		ScriptPubKey: addrScript,
	}
	vin := &types.TxIn{
		PrevOutPoint: types.OutPoint{
			Hash:  zeroHash,
			Index: math.MaxUint32,
		},
		ScriptSig: *script.GasRefundSignatureScript(nonce),
	}
	tx := new(types.Transaction)
	tx.Vin = append(tx.Vin, vin)
	tx.Vout = append(tx.Vout, vout)
	return tx
}

// gasUsed returns the amount of gas used up by the state transition.
func (st *StateTransition) gasUsed() uint64 {
	return st.initialGas - st.gas
}
