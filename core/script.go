// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"encoding/binary"

	"github.com/btcsuite/btcd/txscript"
)

const (
	// defaultScriptAlloc is the default size used for the backing array
	// for a script being built by the ScriptBuilder.  The array will
	// dynamically grow as needed, but this figure is intended to provide
	// enough space for vast majority of scripts without needing to grow the
	// backing array multiple times.
	defaultScriptAlloc = 500
)

// ScriptBuilder provides a facility for building custom scripts.
type ScriptBuilder struct {
	script []byte
	err    error
}

// AddOp pushes the passed opcode to the end of the script.  The script will not
// be modified if pushing the opcode would cause the script to exceed the
// maximum allowed script engine size.
func (b *ScriptBuilder) AddOp(opcode byte) *ScriptBuilder {
	if b.err != nil {
		return b
	}

	b.script = append(b.script, opcode)
	return b
}

// NewScriptBuilder returns a new instance of a script builder.  See
// ScriptBuilder for details.
func NewScriptBuilder() *ScriptBuilder {
	return &ScriptBuilder{
		script: make([]byte, 0, defaultScriptAlloc),
	}
}

// Script returns the currently built script.  When any errors occurred while
// building the script, the script will be returned up the point of the first
// error along with the error.
func (b *ScriptBuilder) Script() ([]byte, error) {
	return b.script, b.err
}

// AddData pushes the passed data to the end of the script.
func (b *ScriptBuilder) AddData(data []byte) *ScriptBuilder {
	if b.err != nil {
		return b
	}

	return b.addData(data)
}

// addData is the internal function that actually pushes the passed data to the
// end of the script.  It automatically chooses canonical opcodes depending on
// the length of the data.  A zero length buffer will lead to a push of empty
// data onto the stack (OP_0).  No data limits are enforced with this function.
func (b *ScriptBuilder) addData(data []byte) *ScriptBuilder {
	dataLen := len(data)

	// When the data consists of a single number that can be represented
	// by one of the "small integer" opcodes, use that opcode instead of
	// a data push opcode followed by the number.
	if dataLen == 0 || dataLen == 1 && data[0] == 0 {
		b.script = append(b.script, OP0)
		return b
	} else if dataLen == 1 && data[0] <= 16 {
		b.script = append(b.script, (OP1-1)+data[0])
		return b
	} else if dataLen == 1 && data[0] == 0x81 {
		b.script = append(b.script, byte(OP1NEGATE))
		return b
	}

	// Use one of the OP_DATA_# opcodes if the length of the data is small
	// enough so the data push instruction is only a single byte.
	// Otherwise, choose the smallest possible OP_PUSHDATA# opcode that
	// can represent the length of the data.
	if dataLen < OPPUSHDATA1 {
		b.script = append(b.script, byte((OPDATA1-1)+dataLen))
	} else if dataLen <= 0xff {
		b.script = append(b.script, OPPUSHDATA1, byte(dataLen))
	} else if dataLen <= 0xffff {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(dataLen))
		b.script = append(b.script, OPPUSHDATA2)
		b.script = append(b.script, buf...)
	} else {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(dataLen))
		b.script = append(b.script, OPPUSHDATA4)
		b.script = append(b.script, buf...)
	}

	// Append the actual data.
	b.script = append(b.script, data...)

	return b
}

// PayToPubKeyHashScript creates a new script to pay a transaction output to a the
// specified address.
func PayToPubKeyHashScript(pubKeyHash []byte) ([]byte, error) {
	return NewScriptBuilder().AddOp(OPDUP).AddOp(OPHASH160).
		AddData(pubKeyHash).AddOp(OPEQUALVERIFY).AddOp(OPCHECKSIG).
		Script()
}

// StandardCoinbaseScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.
func StandardCoinbaseScript(height int32) ([]byte, error) {
	return txscript.NewScriptBuilder().AddInt64(int64(height)).AddInt64(int64(0)).Script()
}
