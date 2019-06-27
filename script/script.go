// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"math/big"
	"reflect"
	"strings"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"golang.org/x/crypto/ripemd160"
)

var logger = log.NewLogger("script") // logger

// constants
const (
	p2PKHScriptLen = 25
	p2SHScriptLen  = 23

	LockTimeThreshold = 5e8 // Tue Nov 5 00:53:20 1985 UTC
)

// variables
var (
	ZeroContractAddress = types.AddressHash{}
)

// PayToPubKeyHashScript creates a script to lock a transaction output to the specified address.
func PayToPubKeyHashScript(pubKeyHash []byte) *Script {
	return NewScript().AddOpCode(OPDUP).AddOpCode(OPHASH160).AddOperand(pubKeyHash).AddOpCode(OPEQUALVERIFY).AddOpCode(OPCHECKSIG)
}

// PayToPubKeyHashCLTVScript creates a script to lock a transaction output to the specified address till a specific time or block height.
func PayToPubKeyHashCLTVScript(pubKeyHash []byte, blockTimeOrHeight int64) *Script {
	return NewScript().AddOperand(big.NewInt(blockTimeOrHeight).Bytes()).AddOpCode(OPCHECKLOCKTIMEVERIFY).AddOpCode(OPDUP).AddOpCode(OPHASH160).
		AddOperand(pubKeyHash).AddOpCode(OPEQUALVERIFY).AddOpCode(OPCHECKSIG)
}

// SignatureScript creates a script to unlock a utxo.
func SignatureScript(sig *crypto.Signature, pubKey []byte) *Script {
	return NewScript().AddOperand(sig.Serialize()).AddOperand(pubKey)
}

// StandardCoinbaseSignatureScript returns a standard signature script for coinbase transaction.
func StandardCoinbaseSignatureScript(height uint32) *Script {
	return NewScript().AddOperand(big.NewInt(int64(height)).Bytes()).AddOperand(big.NewInt(0).Bytes())
}

// GasRefundSignatureScript returns a standard signature script for gas refound transaction.
func GasRefundSignatureScript(nonce uint64) *Script {
	return NewScript().AddOpCode(OPCONTRACT).AddOperand(big.NewInt(int64(nonce)).Bytes())
}

// SplitAddrScript returns a script to store a split address output
func SplitAddrScript(addrs []types.Address, weights []uint64) *Script {
	// OP_RETURN <hash addr> [(addr1, w1), (addr2, w2), (addr3, w3), ...]
	s := NewScript()
	weight := big.NewInt(0)
	// use as many address/weight pairs as possbile
	for i := 0; i < len(addrs) && i < len(weights); i++ {
		weight.SetUint64(weights[i])
		s.AddOperand(addrs[i].Hash()).AddOperand(weight.Bytes())
	}
	// Hash acts as address, like in p2sh
	scriptHash := crypto.Hash160(*s)
	return NewScript().AddOpCode(OPRETURN).AddOperand(scriptHash).AddScript(s)
}

// Script represents scripts
type Script []byte

// NewScript returns an empty script
func NewScript() *Script {
	emptyBytes := make([]byte, 0)
	return (*Script)(&emptyBytes)
}

// NewScriptFromBytes returns a script from byte slice
func NewScriptFromBytes(scriptBytes []byte) *Script {
	script := Script(scriptBytes)
	return &script
}

// AddOpCode adds an opcode to the script
func (s *Script) AddOpCode(opCode OpCode) *Script {
	*s = append(*s, byte(opCode))
	return s
}

// AddOperand adds an operand to the script
func (s *Script) AddOperand(operand []byte) *Script {
	dataLen := len(operand)

	if dataLen < int(OPPUSHDATA1) {
		*s = append(*s, byte(dataLen))
	} else if dataLen <= 0xff {
		*s = append(*s, byte(OPPUSHDATA1), byte(dataLen))
	} else if dataLen <= 0xffff {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(dataLen))
		*s = append(*s, byte(OPPUSHDATA2))
		*s = append(*s, buf...)
	} else {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(dataLen))
		*s = append(*s, byte(OPPUSHDATA4))
		*s = append(*s, buf...)
	}

	// Append the actual operand
	*s = append(*s, operand...)
	return s
}

// AddScript appends a script to the script
func (s *Script) AddScript(script *Script) *Script {
	*s = append(*s, (*script)...)
	return s
}

// Validate verifies the script
func Validate(scriptSig, scriptPubKey *Script, tx *types.Transaction, txInIdx int) error {
	// concatenate unlocking & locking scripts
	catScript := NewScript().AddScript(scriptSig).AddOpCode(OPCODESEPARATOR).AddScript(scriptPubKey)
	if err := catScript.evaluate(tx, txInIdx); err != nil {
		return err
	}

	if !scriptPubKey.IsPayToScriptHash() {
		return nil
	}

	// Handle p2sh
	// scriptSig: signature <serialized redeemScript>
	//

	// First operand is signature
	_, sig, newPc, _ := scriptSig.parseNextOp(0)
	newScriptSig := NewScript().AddOperand(sig)

	// Second operand is serialized redeem script
	_, redeemScriptBytes, _, _ := scriptSig.parseNextOp(newPc)
	redeemScript := NewScriptFromBytes(redeemScriptBytes)

	// signature becomes the new scriptSig, redeemScript becomes the new scriptPubKey
	catScript = NewScript().AddScript(newScriptSig).AddOpCode(OPCODESEPARATOR).AddScript(redeemScript)
	return catScript.evaluate(tx, txInIdx)
}

// Evaluate interprets the script and returns error if it fails
// It succeeds if the script runs to completion and the top stack element exists and is true
func (s *Script) evaluate(tx *types.Transaction, txInIdx int) error {
	script := *s
	scriptLen := len(script)
	// logger.Debugf("script len %d: %s", scriptLen, s.Disasm())

	stack := newStack()
	for pc, scriptPubKeyStart := 0, 0; pc < scriptLen; {
		opCode, operand, newPc, err := s.parseNextOp(pc)
		if err != nil {
			return err
		}
		pc = newPc

		if err := s.execOp(opCode, operand, tx, txInIdx, pc, &scriptPubKeyStart, stack); err != nil {
			return err
		}
	}

	// Succeed if top stack item is true
	return stack.validateTop()
}

// Get the next opcode & operand. Operand only applies to data push opcodes. Also return incremented pc.
func (s *Script) parseNextOp(pc int) (OpCode, Operand, int /* pc */, error) {
	script := *s
	scriptLen := len(script)
	if pc >= scriptLen {
		return 0, nil, pc, ErrScriptBound
	}

	opCode := OpCode(script[pc])
	pc++

	if opCode > OPPUSHDATA4 {
		return opCode, nil, pc, nil
	}

	var operandSize int
	if opCode < OPPUSHDATA1 {
		// opcode itself encodes operand size
		operandSize = int(opCode)
	} else if opCode == OPPUSHDATA1 {
		if scriptLen-pc < 1 {
			return opCode, nil, pc, ErrNoEnoughDataOPPUSHDATA1
		}
		// 1 byte after opcode encodes operand size
		operandSize = int(script[pc])
		pc++
	} else if opCode == OPPUSHDATA2 {
		if scriptLen-pc < 2 {
			return opCode, nil, pc, ErrNoEnoughDataOPPUSHDATA2
		}
		// 2 bytes after opcode encodes operand size
		operandSize = int(binary.LittleEndian.Uint16(script[pc : pc+2]))
		pc += 2
	} else if opCode == OPPUSHDATA4 {
		if scriptLen-pc < 4 {
			return opCode, nil, pc, ErrNoEnoughDataOPPUSHDATA4
		}
		// 4 bytes after opcode encodes operand size
		operandSize = int(binary.LittleEndian.Uint16(script[pc : pc+4]))
		pc += 4
	}

	if scriptLen-pc < operandSize {
		return opCode, nil, pc, ErrScriptBound
	}
	// Read operand
	operand := Operand(script[pc : pc+operandSize])
	pc += operandSize
	return opCode, operand, pc, nil
}

// Execute an operation
func (s *Script) execOp(opCode OpCode, pushData Operand, tx *types.Transaction,
	txInIdx int, pc int, scriptPubKeyStart *int, stack *Stack) error {

	// Push value
	if opCode <= OPPUSHDATA4 {
		// if opCode < OPPUSHDATA1 {
		// 	logger.Debugf("push data len: %d, pc: %d", len(pushData), pc)
		// } else {
		// 	logger.Debugf("opcode: %s, push data len: %d, pc: %d", opCodeToName(opCode), len(pushData), pc)
		// }
		stack.push(pushData)
		return nil
	} else if opCode <= OP16 && opCode != OPRESERVED {
		op := big.NewInt(int64(opCode) - int64(OP1) + 1)
		// logger.Debugf("opcode: %s, push data: %v, pc: %d", opCodeToName(opCode), op, pc)
		stack.push(Operand(op.Bytes()))
		return nil
	}

	// logger.Debugf("opcode: %s, pc: %d", opCodeToName(opCode), pc)
	switch opCode {
	case OPRETURN:
		return ErrOpReturn

	case OPDROP:
		if stack.size() < 1 {
			return ErrInvalidStackOperation
		}
		stack.pop()

	case OPDUP:
		if stack.size() < 1 {
			return ErrInvalidStackOperation
		}
		stack.push(stack.topN(1))

	case OPADD:
		fallthrough
	case OPSUB:
		if stack.size() < 2 {
			return ErrInvalidStackOperation
		}
		op1, op2 := big.NewInt(0), big.NewInt(0)
		op1.SetBytes(stack.topN(2))
		op2.SetBytes(stack.topN(1))
		switch opCode {
		case OPADD:
			op1.Add(op1, op2)
		case OPSUB:
			op1.Sub(op1, op2)
		default:
			return ErrBadOpcode
		}
		stack.pop()
		stack.pop()
		stack.push(op1.Bytes())

	case OPEQUAL:
		fallthrough
	case OPEQUALVERIFY:
		if stack.size() < 2 {
			return ErrInvalidStackOperation
		}
		op1 := stack.topN(2)
		op2 := stack.topN(1)
		// use bytes.Equal() instead of reflect.DeepEqual() for efficiency
		isEqual := bytes.Equal(op1, op2)
		stack.pop()
		stack.pop()
		if isEqual {
			stack.push(operandTrue)
		} else {
			stack.push(operandFalse)
		}
		if opCode == OPEQUALVERIFY {
			if isEqual {
				stack.pop()
			} else {
				return ErrScriptEqualVerify
			}
		}

	case OPHASH160:
		if stack.size() < 1 {
			return ErrInvalidStackOperation
		}
		hash160 := Operand(crypto.Hash160(stack.topN(1)))
		stack.pop()
		stack.push(hash160)

	case OPCODESEPARATOR:
		// scriptPubKey starts after the code separator; pc points to the next byte
		*scriptPubKeyStart = pc

	case OPCHECKSIG:
		fallthrough
	case OPCHECKSIGVERIFY:
		if stack.size() < 2 {
			return ErrInvalidStackOperation
		}
		signature := stack.topN(2)
		pubKey := stack.topN(1)

		// script consists of: scriptSig + OPCODESEPARATOR + scriptPubKey
		scriptPubKey := (*s)[*scriptPubKeyStart:]

		isVerified := verifySig(signature, pubKey, scriptPubKey, tx, txInIdx)

		stack.pop()
		stack.pop()
		if isVerified {
			stack.push(operandTrue)
		} else {
			stack.push(operandFalse)
		}
		if opCode == OPCHECKSIGVERIFY {
			if isVerified {
				stack.pop()
			} else {
				return ErrScriptSignatureVerifyFail
			}
		}

	case OPCHECKMULTISIG:
		fallthrough
	case OPCHECKMULTISIGVERIFY:
		// Format: e.g.,
		// <Signature B> <Signature C> | 2 <Public Key A> <Public Key B> <Public Key C> 3 CHECKMULTISIG
		i := 1
		if stack.size() < i {
			return ErrInvalidStackOperation
		}

		// public keys
		pubKeyCount, err := stack.topN(i).int()
		if err != nil {
			return err
		}
		if pubKeyCount < 0 {
			return ErrCountNegative
		}
		i++
		pubKeyIdx := i
		i += pubKeyCount
		if stack.size() < i {
			return ErrInvalidStackOperation
		}

		// signatures
		sigCount, err := stack.topN(i).int()
		if err != nil {
			return err
		}
		if sigCount < 0 {
			return ErrCountNegative
		}
		if sigCount > pubKeyCount {
			return ErrScriptSignatureVerifyFail
		}
		i++
		sigIdx := i
		i += sigCount
		// Note: i points right beyond signature so use (i-1)
		if stack.size() < i-1 {
			logger.Errorf("sssss%d vs %d", stack.size(), i)
			return ErrInvalidStackOperation
		}

		// script consists of: scriptSig + OPCODESEPARATOR + scriptPubKey
		scriptPubKey := (*s)[*scriptPubKeyStart:]

		isVerified := true
		for isVerified && sigCount > 0 {
			signature := stack.topN(sigIdx)
			pubKey := stack.topN(pubKeyIdx)

			if verifySig(signature, pubKey, scriptPubKey, tx, txInIdx) {
				sigIdx++
				sigCount--
			}
			pubKeyIdx++
			pubKeyCount--

			// More signatures left than keys means verification failure
			if sigCount > pubKeyCount {
				isVerified = false
			}
		}

		for ; i > 1; i-- {
			stack.pop()
		}
		if isVerified {
			stack.push(operandTrue)
		} else {
			stack.push(operandFalse)
		}
		if opCode == OPCHECKSIGVERIFY {
			if isVerified {
				stack.pop()
			} else {
				return ErrScriptSignatureVerifyFail
			}
		}

	case OPCHECKLOCKTIMEVERIFY:
		if stack.size() < 1 {
			return ErrInvalidStackOperation
		}
		op := big.NewInt(0)
		op.SetBytes(stack.topN(1))
		lockTime := op.Int64()
		if checkLockTime(lockTime, tx.LockTime) {
			stack.pop()
		} else {
			return ErrScriptLockTimeVerifyFail
		}

	default:
		return ErrBadOpcode
	}
	return nil
}

func checkLockTime(lockTime, txLockTime int64) bool {
	// same type: either both block height or both UTC seconds
	if !(lockTime < LockTimeThreshold && txLockTime < LockTimeThreshold ||
		lockTime >= LockTimeThreshold && txLockTime >= LockTimeThreshold) {
		return false
	}
	return lockTime <= txLockTime
}

// verify if signature is right
// scriptPubKey is the locking script of the utxo tx input tx.Vin[txInIdx] references
func verifySig(sigStr []byte, publicKeyStr []byte, scriptPubKey []byte, tx *types.Transaction, txInIdx int) bool {
	sig, err := crypto.SigFromBytes(sigStr)
	if err != nil {
		logger.Debugf("Deserialize signature failed")
		return false
	}
	publicKey, err := crypto.PublicKeyFromBytes(publicKeyStr)
	if err != nil {
		logger.Debugf("Deserialize public key failed")
		return false
	}

	sigHash, err := CalcTxHashForSig(scriptPubKey, tx, txInIdx)
	if err != nil {
		logger.Debugf("Calculate signature hash failed")
		return false
	}

	return sig.VerifySignature(publicKey, sigHash)
}

// CalcTxHashForSig calculates the hash of a tx input, used for signature
func CalcTxHashForSig(
	scriptPubKey []byte, originalTx *types.Transaction, txInIdx int,
) (*crypto.HashType, error) {

	if txInIdx >= len(originalTx.Vin) {
		return nil, ErrInputIndexOutOfBound
	}
	tx := types.NewTx(originalTx.Version, originalTx.Magic, originalTx.LockTime).
		AppendVout(originalTx.Vout...)
	for i, txIn := range originalTx.Vin {
		if i != txInIdx {
			tx.AppendVin(types.NewTxIn(&txIn.PrevOutPoint, nil, txIn.Sequence))
		} else {
			tx.AppendVin(types.NewTxIn(&txIn.PrevOutPoint, scriptPubKey, txIn.Sequence))
		}
	}

	return tx.CalcTxHash()
}

// parses the entire script and returns operator/operand sequences.
// The returned result will contain the parsed script up to the failure point, with the last element being the error
func (s *Script) parse() []interface{} {
	var elements []interface{}

	for pc := 0; pc < len(*s); {
		opCode, operand, newPc, err := s.parseNextOp(pc)
		if err != nil {
			elements = append(elements, err)
			return elements
		}
		if operand != nil {
			elements = append(elements, operand)
		} else {
			elements = append(elements, opCode)
		}
		pc = newPc
	}

	return elements
}

// Disasm disassembles script in human readable format. If the script fails to parse, the returned string will
// contain the disassembled script up to the failure point, appended by the string '[Error: error info]'
func (s *Script) Disasm() string {
	var str []string

	elements := s.parse()
	for _, e := range elements {
		switch v := e.(type) {
		case Operand:
			str = append(str, hex.EncodeToString(v))
		case OpCode:
			str = append(str, opCodeToName(v))
		case error:
			str = append(str, "[Error: "+v.Error()+"]")
		default:
			return "Disasmbler encounters unexpected type"
		}
	}

	return strings.Join(str, " ")
}

// IsPayToPubKeyHash returns if the script is p2pkh
func (s *Script) IsPayToPubKeyHash() bool {
	if len(*s) != p2PKHScriptLen {
		return false
	}

	r := s.parse()
	return len(r) == 5 && reflect.DeepEqual(r[0], OPDUP) && reflect.DeepEqual(r[1], OPHASH160) &&
		isOperandOfLen(r[2], 20) && reflect.DeepEqual(r[3], OPEQUALVERIFY) && reflect.DeepEqual(r[4], OPCHECKSIG)
}

// IsPayToPubKeyHashCLTVScript returns if the script is p2pkhCLTV
func (s *Script) IsPayToPubKeyHashCLTVScript() bool {
	r := s.parse()
	return len(r) == 7 && reflect.DeepEqual(r[1], OPCHECKLOCKTIMEVERIFY) && reflect.DeepEqual(r[2], OPDUP) && reflect.DeepEqual(r[3], OPHASH160) && isOperandOfLen(r[4], 20) && reflect.DeepEqual(r[5], OPEQUALVERIFY) && reflect.DeepEqual(r[6], OPCHECKSIG)
}

// IsPayToScriptHash returns if the script is p2sh
func (s *Script) IsPayToScriptHash() bool {
	if len(*s) != p2SHScriptLen {
		return false
	}

	// OP_HASH160 <160-bit redeemp script hash> OP_EQUAL
	r := s.parse()
	return len(r) == 3 && reflect.DeepEqual(r[0], OPHASH160) && isOperandOfLen(r[1], 20) && reflect.DeepEqual(r[2], OPEQUAL)
}

// IsSplitAddrScript returns if the script is split address
// Note: assume OP_RETURN is only used for split address here. Add a magic number if OP_RETURN is used for something else
func (s *Script) IsSplitAddrScript() bool {
	// OP_RETURN <hash addr> [(addr1, w1), (addr2, w2), (addr3, w3), ...]
	r := s.parse()
	return len(r) >= 4 && len(r)%2 == 0 && reflect.DeepEqual(r[0], OPRETURN) && isOperandOfLen(r[1], 20)
}

// IsRegisterCandidateScript returns if the script is register candidate script
func (s *Script) IsRegisterCandidateScript() bool {
	r := s.parse()

	if len(r) == 0 {
		return false
	}
	v, ok := r[0].(Operand)
	if !ok {
		return false
	}
	if _, err := v.int64(); err != nil {
		return false
	}

	return true
}

// IsRegisterCandidateScriptOfBlock returns if the script is register candidate script of certain block time/height
func (s *Script) IsRegisterCandidateScriptOfBlock(blockTimeOrHeight int64) bool {
	r := s.parse()

	if len(r) < 7 {
		return false
	}
	v, ok := r[0].(Operand)
	if !ok {
		return false
	}
	w, err := v.int64()
	if err != nil {
		return false
	}

	return w == blockTimeOrHeight
}

// IsStandard returns if a script is standard
// Only certain types of transactions are allowed, i.e., regarded as standard
func (s *Script) IsStandard() bool {
	if !s.IsPayToPubKeyHash() &&
		!s.IsPayToScriptHash() &&
		!s.IsPayToPubKeyHashCLTVScript() &&
		!s.IsTokenIssue() &&
		!s.IsTokenTransfer() &&
		!s.IsSplitAddrScript() &&
		!s.IsContractPubkey() &&
		!s.IsRegisterCandidateScript() {
		return false
	}

	_, err := s.ExtractAddress()
	if err != nil && err != ErrAddressNotApplicable {
		logger.Errorf("Failed to extract address. script: %s, Err: %v", s.Disasm(), err)
		return false
	}

	return true
}

// GetSplitAddrScriptPrefix returns prefix of split addr script without and list of addresses and weights
// only called on split address script, so no need to check error
func (s *Script) GetSplitAddrScriptPrefix() *Script {
	opCode, _, pc, _ := s.getNthOp(0, 0)
	_, operandHash, _, _ := s.getNthOp(pc, 0)

	return NewScript().AddOpCode(opCode).AddOperand(operandHash)
}

// CreateSplitAddrScriptPrefix creates a script prefix for split address with a hashed address
func CreateSplitAddrScriptPrefix(addr types.Address) *Script {
	return NewScript().AddOpCode(OPRETURN).AddOperand(addr.Hash())
}

// is i of type Operand and of specified length
func isOperandOfLen(i interface{}, length int) bool {
	operand, ok := i.(Operand)
	return ok && len(operand) == length
}

// getNthOp returns the n-th (start from 0) operand and operator, counting from pcStart of the script.
func (s *Script) getNthOp(pcStart, n int) (OpCode, Operand, int /* pc */, error) {
	opCode, operand, newPc, err := OpCode(0), Operand(nil), 0, error(nil)

	for pc, i := pcStart, 0; i <= n; i++ {
		opCode, operand, newPc, err = s.parseNextOp(pc)
		if err != nil {
			return 0, nil, 0, err
		}
		pc = newPc
	}
	return opCode, operand, newPc, err
}

// ExtractAddress returns address within the script
func (s *Script) ExtractAddress() (types.Address, error) {

	if s.IsPayToPubKeyHash() || s.IsTokenIssue() || s.IsTokenTransfer() {
		// p2pkh scriptPubKey: OPDUP OPHASH160 <pubKeyHash> OPEQUALVERIFY OPCHECKSIG [token parameters]
		_, pubKeyHash, _, err := s.getNthOp(0, 2)
		if err != nil {
			return nil, err

		}
		return types.NewAddressPubKeyHash(pubKeyHash)
	}

	if s.IsSplitAddrScript() || s.IsContractPubkey() {
		_, pubKeyHash, _, err := s.getNthOp(1, 0)
		if err != nil {
			return nil, err
		}
		var addr types.Address
		if s.IsSplitAddrScript() {
			addr, err = types.NewSplitAddressFromHash(pubKeyHash)
		} else {
			if reflect.DeepEqual([]byte(pubKeyHash), []byte(ZeroContractAddress[:])) {
				return (*types.AddressContract)(nil), nil
			}
			addr, err = types.NewContractAddressFromHash(pubKeyHash)
		}
		if err != nil {
			return nil, err
		}
		return addr, nil
	}

	if s.IsPayToPubKeyHashCLTVScript() {
		_, pubKeyHash, _, err := s.getNthOp(0, 4)
		if err != nil {
			return nil, err
		}
		return types.NewAddressPubKeyHash(pubKeyHash)
	}

	return nil, ErrAddressNotApplicable
}

// ExtractP2PKHAddress returns address within the script
func (s *Script) ExtractP2PKHAddress() ([]byte, error) {
	if s.IsPayToPubKeyHash() {
		_, pubKeyHash, _, err := s.getNthOp(0, 2)
		if err != nil {
			return nil, err
		}
		return pubKeyHash, nil
	}
	return nil, nil
}

// ParseSplitAddrScript returns [addr1, addr2, addr3, ...], [w1, w2, w3, ...]
// OP_RETURN <hash addr> [(addr1, w1), (addr2, w2), (addr3, w3), ...]
func (s *Script) ParseSplitAddrScript() ([]types.Address, []uint64, error) {
	opCode, _, pc, err := s.getNthOp(0, 0)
	if err != nil || opCode != OPRETURN {
		return nil, nil, ErrInvalidSplitAddrScript
	}

	_, operandHash, pc, err := s.getNthOp(pc, 0)
	if err != nil || opCode != OPRETURN {
		return nil, nil, ErrInvalidSplitAddrScript
	}

	addrs := make([]types.Address, 0)
	weights := make([]uint64, 0)

	for i := 0; ; i++ {
		// public key
		_, operand, _, err := s.getNthOp(pc, i)
		if err != nil {
			if err == ErrScriptBound {
				// reached end
				break
			}
			return nil, nil, ErrInvalidSplitAddrScript
		}
		if i%2 == 0 {
			// address
			addr, err := types.NewAddressPubKeyHash(operand)
			if err != nil {
				return nil, nil, ErrInvalidSplitAddrScript
			}
			addrs = append(addrs, addr)
		} else {
			// weight
			weight, err := operand.int()
			if err != nil {
				return nil, nil, ErrInvalidSplitAddrScript
			}
			weights = append(weights, uint64(weight))
		}
	}

	script := NewScript()
	weight := big.NewInt(0)
	for i := 0; i < len(addrs); i++ {
		weight.SetUint64(weights[i])
		script.AddOperand(addrs[i].Hash()).AddOperand(weight.Bytes())
	}
	scriptHash := crypto.Hash160(*script)
	// Check hash is expected
	if !bytes.Equal(scriptHash, operandHash) {
		return nil, nil, ErrInvalidSplitAddrScript
	}

	return addrs, weights, nil
}

// GetSigOpCount returns number of signature operations in a script
func (s *Script) GetSigOpCount() int {
	numSigs := 0

	elements := s.parse()
	for _, e := range elements {
		switch v := e.(type) {
		case OpCode:
			if v == OPCHECKSIG || v == OPCHECKSIGVERIFY ||
				v == OPCHECKMULTISIG || v == OPCHECKMULTISIGVERIFY {
				numSigs++
			}
		default:
			// Not a opcode
		}
	}

	return numSigs
}

// IsContractSig returns true if the script sig contains OPCONTRACT code
func (s *Script) IsContractSig() bool {
	return len(*s) == 1 && (*s)[0] == byte(OPCONTRACT)
}

// IsContractPubkey returns true if the script pubkey contains OPCONTRACT code
func (s *Script) IsContractPubkey() bool {
	return len(*s) > 1 && (*s)[0] == byte(OPCONTRACT)
}

// MakeContractUtxoScriptPubkey makes a script pubkey for contract addr utxo
func MakeContractUtxoScriptPubkey(addr *types.AddressHash, nonce uint64, version int32) *Script {
	// OP_CONTRACT contract_addr nonce gasPrice gasLimit version code checksum
	s := NewScript()
	s.AddOperand(addr[:]).
		AddOperand(big.NewInt(int64(nonce)).Bytes()).
		AddOperand(big.NewInt(0).Bytes()).
		AddOperand(big.NewInt(0).Bytes()).
		AddOperand(big.NewInt(int64(version)).Bytes()).
		AddOperand(nil)
	// add checksum
	scriptHash := crypto.Hash160(*s)
	checksum := scriptHash[:4]

	return NewScript().AddOpCode(OPCONTRACT).AddScript(s).AddOperand(checksum)
}

// MakeContractScriptPubkey makes a script pubkey for contract vout
func MakeContractScriptPubkey(
	addr types.Address, code []byte, gasPrice, gasLimit, nonce uint64, version int32,
) (*Script, error) {
	// OP_CONTRACT addr nonce gasPrice gasLimit version code checksum
	// check params
	if len(code) == 0 {
		return nil, ErrInvalidContractParams
	}
	overflowVal := uint64(math.MaxInt64)
	if gasPrice > overflowVal || gasLimit > overflowVal {
		return nil, ErrInvalidContractParams
	}
	if gasPrice > overflowVal/gasLimit {
		return nil, ErrInvalidContractParams
	}
	// set params
	s := NewScript()
	toHash := ZeroContractAddress
	if addr != nil && !reflect.ValueOf(addr).IsNil() {
		toHash = *addr.Hash160()
	}
	s.AddOperand(toHash[:]).
		AddOperand(big.NewInt(int64(nonce)).Bytes()).
		AddOperand(big.NewInt(int64(gasPrice)).Bytes()).
		AddOperand(big.NewInt(int64(gasLimit)).Bytes()).
		AddOperand(big.NewInt(int64(version)).Bytes()).
		AddOperand(code)
	// add checksum
	scriptHash := crypto.Hash160(*s)
	checksum := scriptHash[:4]

	return NewScript().AddOpCode(OPCONTRACT).AddScript(s).AddOperand(checksum), nil
}

// MakeContractScriptSig makes a script sig for contract vin
func MakeContractScriptSig() *Script {
	return NewScript().AddOpCode(OPCONTRACT)
}

// ParseContractParams parse script pubkey with OPCONTRACT to stack
func (s *Script) ParseContractParams() (params *types.VMTxParams, typ types.ContractType, err error) {
	typ = types.ContractUnkownType
	// OPCONTRACT
	opCode, _, pc, err := s.getNthOp(0, 0)
	if err != nil || opCode != OPCONTRACT {
		err = ErrInvalidContractScript
		return
	}

	params = new(types.VMTxParams)
	// addr
	_, operand, pc, err := s.getNthOp(pc, 0)
	if err != nil {
		return
	}
	if len(operand) != ripemd160.Size {
		err = ErrInvalidContractScript
		return
	}
	addrHash := new(types.AddressHash)
	copy(addrHash[:], operand[:])
	if *addrHash == ZeroContractAddress {
		typ = types.ContractCreationType
	} else {
		typ = types.ContractCallType
	}
	params.To = addrHash

	// nonce
	params.Nonce, pc, err = s.readUint64(pc)
	if err != nil {
		return
	}
	// gasPrice
	params.GasPrice, pc, err = s.readUint64(pc)
	if err != nil {
		return
	}
	// gasLimit
	params.GasLimit, pc, err = s.readUint64(pc)
	if err != nil {
		return
	}
	// version
	n, pc, e := s.readUint64(pc)
	if err != nil {
		err = e
		return
	}
	params.Version = int32(n)
	// code
	_, operand, pc, err = s.getNthOp(pc, 0)
	if err != nil {
		return
	}
	params.Code = operand
	// checksum
	cs := crypto.Hash160((*s)[1:pc])
	_, operand, pc, err = s.getNthOp(pc, 0)
	if err != nil {
		return
	}
	if len(operand) != 4 {
		err = ErrInvalidContractScript
		return
	}
	if !bytes.Equal(operand, cs[:4]) {
		logger.Warnf("contract script checksum mismatched")
		err = ErrInvalidContractScript
		return
	}
	if _, _, _, e := s.getNthOp(pc, 0); e != ErrScriptBound {
		err = ErrInvalidContractScript
		return
	}
	return
}

// ParseContractNonce returns address within the script
func (s *Script) ParseContractNonce() (uint64, error) {
	if !s.IsContractPubkey() {
		return 0, errors.New("not a contract script")
	}
	_, operand, _, err := s.getNthOp(22, 0) // 1, 22
	if err != nil {
		return 0, err
	}
	n, err := operand.int64()
	if err != nil {
		return 0, err
	}
	return uint64(n), nil
}

// ParseContractGasPrice returns address within the script
func (s *Script) ParseContractGasPrice() (uint64, error) {
	if !s.IsContractPubkey() {
		return 0, errors.New("not a contract script")
	}
	_, operand, _, err := s.getNthOp(22, 1) // 1, 22
	if err != nil {
		return 0, err
	}
	n, err := operand.int64()
	if err != nil {
		return 0, err
	}
	return uint64(n), nil
}

// ParseContractGas returns address within the script
func (s *Script) ParseContractGas() (uint64, error) {
	if !s.IsContractPubkey() {
		return 0, errors.New("not a contract script")
	}
	_, operand, _, err := s.getNthOp(22, 2) // 1, 22
	if err != nil {
		return 0, err
	}
	n, err := operand.int64()
	if err != nil {
		return 0, err
	}
	return uint64(n), nil
}

func (s *Script) readInt64(pc int) (int64, int /* pc */, error) {
	_, operand, pc, err := s.getNthOp(pc, 0)
	if err != nil {
		return 0, pc, err
	}
	n, err := operand.int64()
	if err != nil {
		return 0, pc, err
	}
	return n, pc, nil
}

func (s *Script) readUint64(pc int) (uint64, int /* pc */, error) {
	n, pc, err := s.readInt64(pc)
	if err != nil {
		return 0, pc, err
	}
	return uint64(n), pc, nil
}
