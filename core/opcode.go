// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

// An opcode defines the information related to a txscript opcode.  opfunc, if
// present, is the function to call to perform the opcode on the script.  The
// current script is passed in as a slice with the first member being the opcode
// itself.
type opcode struct {
	value  byte
	name   string
	length int
}

// const define const.
const (
	OP0           = 0x00
	OPDATA1       = 0x01 // 1
	OPPUSHDATA1   = 0x4c // 76
	OPPUSHDATA2   = 0x4d // 77
	OPPUSHDATA4   = 0x4e // 78
	OP1NEGATE     = 0x4f // 79
	OP1           = 0x51
	OPDUP         = 0x76 // 118
	OPEQUALVERIFY = 0x88 // 136
	OPHASH160     = 0xa9 // 169

	OPCHECKSIG = 0xac // 172
)

// Conditional execution constants.
const (
	OpCondFalse = 0
	OpCondTrue  = 1
	OpCondSkip  = 2
)
