// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"encoding/binary"
)

var (
	// TokenNameKey is the key for writing token name onchain
	TokenNameKey = []byte("Name")
	// TokenAmountKey is the key for writing token amount onchain
	TokenAmountKey = []byte("Amount")
)

// IssueParams defines parameters for issuing tokens
type IssueParams struct {
	// token name
	Name string
	// token total supply
	TotalSupply uint64
}

// TransferParams defines parameters for transferring tokens
type TransferParams struct {
	Amount uint64
}

// IssueTokenScript creates a script to issue tokens to the specified address.
func IssueTokenScript(pubKeyHash []byte, params *IssueParams) *Script {
	// Regular p2pkh
	script := PayToPubKeyHashScript(pubKeyHash)
	// Append parameters to p2pkh: TokenNameKey OP_DROP <token name> OP_DROP TokenAmountKey OP_DROP <token supply> OP_DROP
	nameOperand := []byte(params.Name)
	totalSupplyOperand := make([]byte, 8)
	binary.LittleEndian.PutUint64(totalSupplyOperand, params.TotalSupply)
	script.AddOperand(TokenNameKey).AddOpCode(OPDROP).AddOperand(nameOperand).AddOpCode(OPDROP)
	return script.AddOperand(TokenAmountKey).AddOpCode(OPDROP).AddOperand(totalSupplyOperand).AddOpCode(OPDROP)
}

// GetIssueParams returns token issue parameters embedded in the script
func (s *Script) GetIssueParams() (*IssueParams, error) {
	// TODO: do more checks to verify the script is valid, e.g., first opcode is OP_DUP
	// OPDUP OPHASH160 pubKeyHash OPEQUALVERIFY OPCHECKSIG
	// TokenNameKey OP_DROP <token name> OP_DROP TokenAmountKey OP_DROP <token supply> OP_DROP
	params := &IssueParams{}
	// pc points to second OP_DROP
	_, operand, pc, err := s.getNthOp(0, 7)
	if err != nil {
		return nil, err
	}
	params.Name = string(operand)

	_, operand, pc, err = s.getNthOp(pc, 3)
	if err != nil {
		return nil, err
	}

	params.TotalSupply = binary.LittleEndian.Uint64(operand)

	return params, nil
}

// TransferTokenScript creates a script to transfer tokens to the specified address.
func TransferTokenScript(pubKeyHash []byte, params *TransferParams) *Script {
	// Regular p2pkh
	script := PayToPubKeyHashScript(pubKeyHash)
	// Append parameters to p2pkh: TokenAmountKey OP_DROP <token amount> OP_DROP
	amountOperand := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountOperand, params.Amount)
	return script.AddOperand(TokenAmountKey).AddOpCode(OPDROP).AddOperand(amountOperand).AddOpCode(OPDROP)
}

// GetTransferParams returns token transfer parameters embedded in the script
func (s *Script) GetTransferParams() (*TransferParams, error) {
	// TODO: do more checks to verify the script is valid, e.g., first opcode is OP_DUP
	// OPDUP OPHASH160 pubKeyHash OPEQUALVERIFY OPCHECKSIG
	// TokenAmountKey OP_DROP <token amount> OP_DROP
	params := &TransferParams{}
	_, operand, _, err := s.getNthOp(0, 7)
	if err != nil {
		return nil, err
	}
	params.Amount = binary.LittleEndian.Uint64(operand)
	return params, nil
}

// GetTokenAmount returns token issue/transfer amount.
// Must be called on token tx scriptPubKey
// TODO: validate the script is strictly formed, similar to IsPayToPubKeyHash
func (s *Script) GetTokenAmount() uint64 {
	issueParams, err := s.GetIssueParams()
	if err == nil {
		// must be token issue script since getNthOp(0, 7+3) will be out of bound in token transfer
		return issueParams.TotalSupply
	}
	// token transfer
	transferParams, err := s.GetTransferParams()
	if err != nil {
		logger.Errorf("token tx scriptPubKey is malformed, neither issurance nor transfer: %s", s.disasm())
		return 0
	}
	return transferParams.Amount
}
