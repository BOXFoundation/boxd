// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"bytes"

	"github.com/BOXFoundation/boxd/log"
	"github.com/btcsuite/btcd/txscript"

	"encoding/binary"
	"errors"
)

const (
	// defaultScriptAlloc is the default size used for the backing array
	// for a script being built by the Builder.  The array will
	// dynamically grow as needed, but this figure is intended to provide
	// enough space for vast majority of scripts without needing to grow the
	// backing array multiple times.
	defaultScriptAlloc = 500
)

var logger = log.NewLogger("script") // logger

// Builder provides a facility for building custom scripts.
type Builder struct {
	script []byte
	err    error
}

// AddOp pushes the passed opcode to the end of the script.  The script will not
// be modified if pushing the opcode would cause the script to exceed the
// maximum allowed script engine size.
func (b *Builder) AddOp(opcode byte) *Builder {
	if b.err != nil {
		return b
	}

	b.script = append(b.script, opcode)
	return b
}

// NewBuilder returns a new instance of a script builder.  See
// Builder for details.
func NewBuilder() *Builder {
	return &Builder{
		script: make([]byte, 0, defaultScriptAlloc),
	}
}

// Script returns the currently built script.  When any errors occurred while
// building the script, the script will be returned up the point of the first
// error along with the error.
func (b *Builder) Script() ([]byte, error) {
	return b.script, b.err
}

// AddData pushes the passed data to the end of the script.
func (b *Builder) AddData(data []byte) *Builder {
	if b.err != nil {
		return b
	}

	return b.addData(data)
}

// addData is the internal function that actually pushes the passed data to the
// end of the script.  It automatically chooses canonical opcodes depending on
// the length of the data.  A zero length buffer will lead to a push of empty
// data onto the stack (OP_0).  No data limits are enforced with this function.
func (b *Builder) addData(data []byte) *Builder {
	dataLen := len(data)

	// When the data consists of a single number that can be represented
	// by one of the "small integer" opcodes, use that opcode instead of
	// a data push opcode followed by the number.
	if dataLen == 0 || dataLen == 1 && data[0] == 0 {
		b.script = append(b.script, byte(OP0))
		return b
	} else if dataLen == 1 && data[0] <= 16 {
		b.script = append(b.script, (byte(OP1)-1)+data[0])
		return b
	} else if dataLen == 1 && data[0] == 0x81 {
		b.script = append(b.script, byte(OP1NEGATE))
		return b
	}

	// Use one of the OP_DATA_# opcodes if the length of the data is small
	// enough so the data push instruction is only a single byte.
	// Otherwise, choose the smallest possible OP_PUSHDATA# opcode that
	// can represent the length of the data.
	if dataLen < int(OPPUSHDATA1) {
		b.script = append(b.script, byte(dataLen))
	} else if dataLen <= 0xff {
		b.script = append(b.script, byte(OPPUSHDATA1), byte(dataLen))
	} else if dataLen <= 0xffff {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(dataLen))
		b.script = append(b.script, byte(OPPUSHDATA2))
		b.script = append(b.script, buf...)
	} else {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(dataLen))
		b.script = append(b.script, byte(OPPUSHDATA4))
		b.script = append(b.script, buf...)
	}

	// Append the actual data.
	b.script = append(b.script, data...)

	return b
}

// PayToPubKeyHashScript creates a new script to pay a transaction output to a the
// specified address.
func PayToPubKeyHashScript(pubKeyHash []byte) ([]byte, error) {
	return nil, nil
	// return NewBuilder().AddOp(OPDUP).AddOp(OPHASH160).
	// 	AddData(pubKeyHash).AddOp(OPEQUALVERIFY).AddOp(OPCHECKSIG).
	// 	Script()
}

// StandardCoinbaseScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.
func StandardCoinbaseScript(height int32) ([]byte, error) {
	return txscript.NewScriptBuilder().AddInt64(int64(height)).AddInt64(int64(0)).Script()
}

// Script represents scripts
type Script []byte

// Evaluate interprets the script and returns error if it fails
func (s *Script) Evaluate() error {
	script := *s
	scriptLen := len(script)

	stack := newStack()
	for pc := 0; pc < scriptLen; {
		logger.Infof("pc: %d, script length: %d", pc, scriptLen)
		opCode, operand, newPc, err := s.parseNextOp(pc)
		if err != nil {
			return err
		}
		pc = newPc

		if err := execOp(opCode, operand, stack); err != nil {
			return err
		}
	}

	// Succeed if top stack item is true
	return stack.validateTop()
}

// Get the next opcode & operand. Also return incremented pc.
func (s *Script) parseNextOp(pc int) (OpCode, Operand, int, error) {
	script := *s
	scriptLen := len(script)
	if pc >= scriptLen {
		return 0, nil, pc, errors.New("Program counter out of script bound")
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
			return opCode, nil, pc, errors.New("OP_PUSHDATA1 has not enough data")
		}
		// 1 byte after opcode encodes operand size
		operandSize = int(script[pc])
		pc++
	} else if opCode == OPPUSHDATA2 {
		if scriptLen-pc < 2 {
			return opCode, nil, pc, errors.New("OP_PUSHDATA2 has not enough data")
		}
		// 2 bytes after opcode encodes operand size
		operandSize = int(binary.LittleEndian.Uint16(script[pc : pc+2]))
		pc += 2
	} else if opCode == OPPUSHDATA4 {
		if scriptLen-pc < 4 {
			return opCode, nil, pc, errors.New("OP_PUSHDATA4 has not enough data")
		}
		// 4 bytes after opcode encodes operand size
		operandSize = int(binary.LittleEndian.Uint16(script[pc : pc+4]))
		pc += 4
	}

	if scriptLen-pc < operandSize {
		return opCode, nil, pc, errors.New("Program counter out of script bound")
	}
	// Read operand
	operand := Operand(script[pc : pc+operandSize])
	pc += operandSize
	return opCode, operand, pc, nil
}

// Execute an operation
func execOp(opCode OpCode, op Operand, stack *Stack) error {
	if opCode <= OPPUSHDATA4 {
		stack.push(op)
		return nil
	}
	switch opCode {
	// Push value
	case OP1NEGATE:
		fallthrough
	case OP1:
		fallthrough
	case OP2:
		fallthrough
	case OP3:
		fallthrough
	case OP4:
		fallthrough
	case OP5:
		fallthrough
	case OP6:
		fallthrough
	case OP7:
		fallthrough
	case OP8:
		fallthrough
	case OP9:
		fallthrough
	case OP10:
		fallthrough
	case OP11:
		fallthrough
	case OP12:
		fallthrough
	case OP13:
		fallthrough
	case OP14:
		fallthrough
	case OP15:
		fallthrough
	case OP16:
		sn := scriptNum(opCode) - scriptNum(OP1-1)
		stack.push(Operand(sn.Bytes()))

	case OPADD:
		fallthrough
	case OPSUB:
		if stack.size() < 2 {
			return errors.New("ScriptErrInvalidStackOperation")
		}
		op1 := stack.topN(2)
		sn1, err := newScriptNum(op1)
		if err != nil {
			return err
		}
		op2 := stack.topN(1)
		sn2, err := newScriptNum(op2)
		if err != nil {
			return err
		}
		var sn scriptNum
		switch opCode {
		case OPADD:
			sn = sn1 + sn2
		case OPSUB:
			sn = sn1 - sn2
		default:
			return errors.New("Bad opcode")
		}
		stack.pop()
		stack.pop()
		stack.push(sn.Bytes())

	case OPEQUAL:
		fallthrough
	case OPEQUALVERIFY:
		if stack.size() < 2 {
			return errors.New("ScriptErrInvalidStackOperation")
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
				return errors.New("ScriptErrEqualVerify")
			}
		}

	default:
		return errors.New("Bad opcode")
	}
	return nil
}
