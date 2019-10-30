// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"math/big"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/vm/common/math"
)

// NetParams Represents current network parameters
type NetParams struct {
	DynastySwitchThreshold *big.Int
	BlockReward            *big.Int
	CalcScoreThreshold     *big.Int
}

// FetchNetParamsByHeight fetch net params from genesis contract.
func (chain *BlockChain) FetchNetParamsByHeight(height uint32) (*NetParams, error) {

	output, err := chain.CallGenesisContract(height, "getNetParams")
	if err != nil {
		return nil, err
	}
	var res [12]*big.Int
	if err := ContractAbi.Unpack(&res, "getNetParams", output); err != nil {
		logger.Errorf("Failed to unpack the result of call getNetParams. Err: %v", err)
		return nil, err
	}
	return &NetParams{
		DynastySwitchThreshold: res[1],
		BlockReward:            res[11],
		CalcScoreThreshold:     res[10],
	}, nil
}

// CallGenesisContract genesis smart contract
func (chain *BlockChain) CallGenesisContract(height uint32, method string) ([]byte, error) {

	data, err := ContractAbi.Pack(method)
	if err != nil {
		return nil, err
	}
	adminAddr, err := types.NewAddress(Admin)
	msg := types.NewVMTransaction(new(big.Int), math.MaxUint64/2, 0, 0, nil,
		types.ContractCallType, data).
		WithFrom(adminAddr.Hash160()).WithTo(&ContractAddr)
	evm, vmErr, err := chain.NewEvmContextForLocalCallByHeight(msg, height)
	if err != nil {
		return nil, err
	}
	output, _, _, _, _, err := ApplyMessage(evm, msg)
	if err := vmErr(); err != nil {
		return nil, err
	}
	return output, nil
}
