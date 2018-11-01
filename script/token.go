// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import "encoding/binary"

var (
	// TokenNameKey is the key for writing token name onchain
	TokenNameKey = []byte("Name")
	// TokenAmountKey is the key for writing token amount onchain
	TokenAmountKey = []byte("Amount")
)

// IssueParams defines parameters for issuing tokens
type IssueParams struct {
	// token name
	name string
	// token total supply
	totalSupply uint64
}

// TransferParams defines parameters for transferring tokens
type TransferParams struct {
	amount uint64
}

// IssueTokenScript creates a script to issue tokens to the specified address.
func IssueTokenScript(pubKeyHash []byte, params *IssueParams) *Script {
	// Regular p2pkh
	script := PayToPubKeyHashScript(pubKeyHash)
	// Append parameters to p2pkh: TokenNameKey OP_DROP <token name> OP_DROP TokenAmountKey OP_DROP <token supply> OP_DROP
	nameOperand := []byte(params.name)
	totalSupplyOperand := make([]byte, 8)
	binary.LittleEndian.PutUint64(totalSupplyOperand, params.totalSupply)
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
	params.name = string(operand)

	_, operand, pc, err = s.getNthOp(pc, 3)
	if err != nil {
		return nil, err
	}

	params.totalSupply = binary.LittleEndian.Uint64(operand)

	return params, nil
}

// TransferTokenScript creates a script to transfer tokens to the specified address.
func TransferTokenScript(pubKeyHash []byte, params *TransferParams) *Script {
	// Regular p2pkh
	script := PayToPubKeyHashScript(pubKeyHash)
	// Append parameters to p2pkh: TokenAmountKey OP_DROP <token amount> OP_DROP
	amountOperand := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountOperand, params.amount)
	return script.AddOperand(TokenAmountKey).AddOpCode(OPDROP).AddOperand(amountOperand).AddOpCode(OPDROP)
}

// GetTransferParams returns token issue parameters embedded in the script
func (s *Script) GetTransferParams() (*TransferParams, error) {
	// TODO: do more checks to verify the script is valid, e.g., first opcode is OP_DUP
	// OPDUP OPHASH160 pubKeyHash OPEQUALVERIFY OPCHECKSIG
	// TokenAmountKey OP_DROP <token amount> OP_DROP
	params := &TransferParams{}
	_, operand, _, err := s.getNthOp(0, 7)
	if err != nil {
		return nil, err
	}
	params.amount = binary.LittleEndian.Uint64(operand)
	return params, nil
}
