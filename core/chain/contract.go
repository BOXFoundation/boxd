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
	BookKeeperReward       *big.Int
	CalcScoreThreshold     *big.Int
}

// FetchNetParamsByHeight fetch net params from genesis contract.
func (chain *BlockChain) FetchNetParamsByHeight(height uint32) (*NetParams, error) {

	output, err := chain.Call(height, "getNetParams")
	if err != nil {
		return nil, err
	}
	var res [3]*big.Int
	if err := ContractAbi.Unpack(&res, "getNetParams", output); err != nil {
		logger.Errorf("Failed to unpack the result of call getNetParams. Err: %v", err)
		return nil, err
	}
	return &NetParams{
		DynastySwitchThreshold: res[0],
		BookKeeperReward:       res[1],
		CalcScoreThreshold:     res[2],
	}, nil
}

// Call genesis smart contract
func (chain *BlockChain) Call(height uint32, method string) ([]byte, error) {

	data, err := ContractAbi.Pack(method)
	if err != nil {
		return nil, err
	}
	adminAddr, err := types.NewAddress(Admin)
	msg := types.NewVMTransaction(new(big.Int), big.NewInt(1), math.MaxUint64/2,
		0, nil, types.ContractCallType, data).WithFrom(adminAddr.Hash160()).WithTo(&ContractAddr)
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
