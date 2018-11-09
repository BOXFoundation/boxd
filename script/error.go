// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"errors"
)

// error
var (

	// script.go
	ErrScriptBound               = errors.New("Program counter out of script bound")
	ErrNoEnoughDataOPPUSHDATA1   = errors.New("OP_PUSHDATA1 has not enough data")
	ErrNoEnoughDataOPPUSHDATA2   = errors.New("OP_PUSHDATA2 has not enough data")
	ErrNoEnoughDataOPPUSHDATA4   = errors.New("OP_PUSHDATA4 has not enough data")
	ErrInvalidStackOperation     = errors.New("ScriptErrInvalidStackOperation")
	ErrBadOpcode                 = errors.New("Bad opcode")
	ErrScriptEqualVerify         = errors.New("ScriptErrEqualVerify")
	ErrScriptSignatureVerifyFail = errors.New("ScriptErrSignatureVerifyFail")
	ErrInputIndexOutOfBound      = errors.New("input index out of bound")
	ErrAddressNotApplicable      = errors.New("Address only applies to p2pkh and token txs")

	// stack.go
	ErrFinalStackEmpty       = errors.New("Final stack empty")
	ErrFinalTopStackEleFalse = errors.New("Final top stack element false")

	// scriptnum.go
	ErrNumberTooBig = errors.New("Number too big error")
	ErrMinimalData  = errors.New("Minimal data error")
)
