// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"errors"
	"math/big"
)

// Operand represents stack operand when interpretting script
type Operand []byte

var (
	operandFalse = Operand([]byte{0})
	operandTrue  = Operand([]byte{1})
)

func (o Operand) int() (int, error) {
	bigInt := big.NewInt(0)
	bigInt.SetBytes(o)
	if !bigInt.IsInt64() {
		return 0, errors.New("Cannot be converted to int 64")
	}
	return int(bigInt.Int64()), nil
}

func (o Operand) int64() (int64, error) {
	bigInt := big.NewInt(0)
	bigInt.SetBytes(o)
	if bigInt.IsInt64() {
		return bigInt.Int64(), nil
	}
	return 0, errors.New("Cannot be converted to int64")
}

// Stack is used when interpretting script
type Stack struct {
	stk []Operand
}

func (s *Stack) size() int {
	return len(s.stk)
}

func (s *Stack) empty() bool {
	return len(s.stk) == 0
}

func (s *Stack) push(o Operand) {
	s.stk = append(s.stk, o)
}

func (s *Stack) pop() Operand {
	stackLen := len(s.stk)
	if stackLen == 0 {
		return nil
	}

	o := s.stk[stackLen-1]
	s.stk = s.stk[:stackLen-1]
	return o
}

// topN returns the top n-th element, n starts from 1.
func (s *Stack) topN(n int) Operand {
	stackLen := len(s.stk)
	if n <= 0 || n > stackLen {
		return nil
	}
	return s.stk[stackLen-n]
}

// validateTop succeeds if top stack item is true
func (s *Stack) validateTop() error {
	if s.empty() {
		return ErrFinalStackEmpty
	}
	topOp := big.NewInt(0)
	topOp.SetBytes(s.topN(1))
	if topOp.Cmp(big.NewInt(0)) == 0 {
		return ErrFinalTopStackEleFalse
	}
	return nil
}

// NewStack creates a clean stack
func newStack() *Stack {
	stk := make([]Operand, 0)
	return &Stack{stk}
}
