// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

// Operand represents stack operand when interpretting script
type Operand []byte

var (
	operandFalse = Operand([]byte{0})
	operandTrue  = Operand([]byte{1})
)

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
	snTop, err := newScriptNum(s.topN(1))
	if err != nil {
		return err
	}
	if snTop == scriptNumZero {
		return ErrFinalTopStackEleFalse
	}
	return nil
}

// NewStack creates a clean stack
func newStack() *Stack {
	stk := make([]Operand, 0)
	return &Stack{stk}
}
