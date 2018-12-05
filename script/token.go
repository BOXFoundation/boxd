// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
)

var (
	// TokenNameKey is the key for writing token name onchain
	TokenNameKey = []byte("Name")
	// TokenSymbolKey is the key for writing token symbol onchain
	TokenSymbolKey = []byte("Symbol")
	// TokenAmountKey is the key for writing token amount onchain
	TokenAmountKey = []byte("Amount")
	// TokenDecimalsKey is the key for writing token decimals onchain
	TokenDecimalsKey = []byte("Decimals")

	// TokenTxHashKey is the key for writing tx hash of token id onchain
	TokenTxHashKey = []byte("TokenTxHash")
	// TokenTxOutIdxKey is the key for writing tx output index of token id onchain
	TokenTxOutIdxKey = []byte("TokenTxOutIdx")
)

// IssueParams defines parameters for issuing ERC20-like tokens
type IssueParams struct {
	Name        string
	Symbol      string
	TotalSupply uint64
	Decimals    uint8
}

// TokenID uniquely identifies a token, consisting of tx hash and output index
type TokenID struct {
	// An outpoint uniquely identifies a token by its issurance tx and output index,
	// which contains all parameters of the token
	types.OutPoint
}

func (t *TokenID) String() string {
	return fmt.Sprintf("%x#%d", t.Hash, t.Index)
}

// TransferParams defines parameters for transferring tokens
type TransferParams struct {
	TokenID
	Amount uint64
}

// NewTokenID returns a new token ID
func NewTokenID(txHash crypto.HashType, txOutIdx uint32) TokenID {
	tokenID := TokenID{}
	tokenID.Hash = txHash
	tokenID.Index = txOutIdx
	return tokenID
}

// IssueTokenScript creates a script to issue tokens to the specified address.
func IssueTokenScript(pubKeyHash []byte, params *IssueParams) *Script {
	// Regular p2pkh
	script := PayToPubKeyHashScript(pubKeyHash)
	// Append parameters to p2pkh:
	// TokenNameKey OP_DROP <token name> OP_DROP
	// TokenSymbolKey OP_DROP <token symbol> OP_DROP
	// TokenAmountKey OP_DROP <token supply> OP_DROP
	// TokenDecimalsKey OP_DROP <token decimals> OP_DROP
	nameOperand := []byte(params.Name)
	symbolOperand := []byte(params.Symbol)
	totalSupplyOperand := make([]byte, 8)
	binary.LittleEndian.PutUint64(totalSupplyOperand, params.TotalSupply)
	decimalsOperand := []byte{params.Decimals}

	script.AddOperand(TokenNameKey).AddOpCode(OPDROP).AddOperand(nameOperand).AddOpCode(OPDROP)
	script.AddOperand(TokenSymbolKey).AddOpCode(OPDROP).AddOperand(symbolOperand).AddOpCode(OPDROP)
	script.AddOperand(TokenAmountKey).AddOpCode(OPDROP).AddOperand(totalSupplyOperand).AddOpCode(OPDROP)
	script.AddOperand(TokenDecimalsKey).AddOpCode(OPDROP).AddOperand(decimalsOperand).AddOpCode(OPDROP)
	return script
}

// GetIssueParams returns token issue parameters embedded in the script
func (s *Script) GetIssueParams() (*IssueParams, error) {
	// OPDUP OPHASH160 pubKeyHash OPEQUALVERIFY OPCHECKSIG
	// TokenNameKey OP_DROP <token name> OP_DROP
	// TokenSymbolKey OP_DROP <token symbol> OP_DROP
	// TokenAmountKey OP_DROP <token supply> OP_DROP
	// TokenDecimalsKey OP_DROP <token decimals> OP_DROP
	params := &IssueParams{}
	// pc points to second OP_DROP
	_, operand, pc, err := s.getNthOp(0, 7)
	if err != nil {
		return nil, err
	}
	params.Name = string(operand)

	if _, operand, pc, err = s.getNthOp(pc, 3); err != nil {
		return nil, err
	}
	params.Symbol = string(operand)

	if _, operand, pc, err = s.getNthOp(pc, 3); err != nil {
		return nil, err
	}
	params.TotalSupply = binary.LittleEndian.Uint64(operand)

	if _, operand, _, err = s.getNthOp(pc, 3); err != nil {
		return nil, err
	}
	params.Decimals = operand[0]

	return params, nil
}

// TransferTokenScript creates a script to transfer tokens to the specified address.
func TransferTokenScript(pubKeyHash []byte, params *TransferParams) *Script {
	// Regular p2pkh
	script := PayToPubKeyHashScript(pubKeyHash)
	// Append parameters to p2pkh:
	// TokenTxHashKey OP_DROP <tx hash> OP_DROP
	// TokenTxOutIdxKey OP_DROP <tx output index> OP_DROP
	// TokenAmountKey OP_DROP <token amount> OP_DROP
	tokenTxHash := []byte(params.Hash[:])
	tokenTxOutIdx := make([]byte, 4)
	binary.LittleEndian.PutUint32(tokenTxOutIdx, params.Index)
	amount := make([]byte, 8)
	binary.LittleEndian.PutUint64(amount, params.Amount)
	return script.AddOperand(TokenTxHashKey).AddOpCode(OPDROP).AddOperand(tokenTxHash).AddOpCode(OPDROP).
		AddOperand(TokenTxOutIdxKey).AddOpCode(OPDROP).AddOperand(tokenTxOutIdx).AddOpCode(OPDROP).
		AddOperand(TokenAmountKey).AddOpCode(OPDROP).AddOperand(amount).AddOpCode(OPDROP)
}

// GetTransferParams returns token transfer parameters embedded in the script
func (s *Script) GetTransferParams() (*TransferParams, error) {
	// OPDUP OPHASH160 pubKeyHash OPEQUALVERIFY OPCHECKSIG
	// TokenTxHashKey OP_DROP <tx hash> OP_DROP
	// TokenTxOutIdxKey OP_DROP <tx output index> OP_DROP
	// TokenAmountKey OP_DROP <token amount> OP_DROP
	params := &TransferParams{}
	_, operand, pc, err := s.getNthOp(0, 7)
	if err != nil {
		return nil, err
	}
	if numOfBytesRead := copy(params.Hash[:], operand); numOfBytesRead != crypto.HashSize {
		return nil, fmt.Errorf("tx hash size not %d: %d", crypto.HashSize, numOfBytesRead)
	}

	if _, operand, pc, err = s.getNthOp(pc, 3); err != nil {
		return nil, err
	}
	params.Index = binary.LittleEndian.Uint32(operand)

	if _, operand, _, err = s.getNthOp(pc, 3); err != nil {
		return nil, err
	}
	params.Amount = binary.LittleEndian.Uint64(operand)

	return params, nil
}

// IsTokenIssue returns if the script is token issurance
func (s *Script) IsTokenIssue() bool {
	// two parts: p2pkh + issue parameters

	p2PKHSubScript := NewScriptFromBytes((*s)[:p2PKHScriptLen])
	if !p2PKHSubScript.IsPayToPubKeyHash() {
		return false
	}

	paramsSubScript := NewScriptFromBytes((*s)[p2PKHScriptLen:])
	r := paramsSubScript.parse()
	return len(r) == 16 &&
		reflect.DeepEqual([]byte(r[0].(Operand)), TokenNameKey) && reflect.DeepEqual(r[1], OPDROP) && reflect.DeepEqual(r[3], OPDROP) &&
		reflect.DeepEqual([]byte(r[4].(Operand)), TokenSymbolKey) && reflect.DeepEqual(r[5], OPDROP) && reflect.DeepEqual(r[7], OPDROP) &&
		reflect.DeepEqual([]byte(r[8].(Operand)), TokenAmountKey) && reflect.DeepEqual(r[9], OPDROP) && reflect.DeepEqual(r[11], OPDROP) &&
		reflect.DeepEqual([]byte(r[12].(Operand)), TokenDecimalsKey) && reflect.DeepEqual(r[13], OPDROP) && reflect.DeepEqual(r[15], OPDROP)
}

// IsTokenTransfer returns if the script is token issurance
func (s *Script) IsTokenTransfer() bool {
	// two parts: p2pkh + issue parameters

	p2PKHSubScript := NewScriptFromBytes((*s)[:p2PKHScriptLen])
	if !p2PKHSubScript.IsPayToPubKeyHash() {
		return false
	}

	paramsSubScript := NewScriptFromBytes((*s)[p2PKHScriptLen:])
	r := paramsSubScript.parse()
	return len(r) == 12 && reflect.DeepEqual([]byte(r[0].(Operand)), TokenTxHashKey) && reflect.DeepEqual(r[1], OPDROP) &&
		reflect.DeepEqual(r[3], OPDROP) && reflect.DeepEqual([]byte(r[4].(Operand)), TokenTxOutIdxKey) &&
		reflect.DeepEqual(r[5], OPDROP) && reflect.DeepEqual(r[7], OPDROP) && reflect.DeepEqual([]byte(r[8].(Operand)), TokenAmountKey) &&
		reflect.DeepEqual(r[9], OPDROP) && reflect.DeepEqual(r[11], OPDROP)
}

// P2PKHScriptPrefix returns p2pkh prefix of token script
func (s *Script) P2PKHScriptPrefix() *Script {
	return NewScriptFromBytes((*s)[:p2PKHScriptLen])
}
