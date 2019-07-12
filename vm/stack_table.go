// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package vm

import (
	"fmt"

	"github.com/BOXFoundation/boxd/vm/common/types"
)

func makeStackFunc(pop, push int) stackValidationFunc {
	return func(stack *Stack) error {
		if err := stack.require(pop); err != nil {
			return err
		}

		if stack.len()+push-pop > int(types.StackLimit) {
			return fmt.Errorf("stack limit reached %d (%d)", stack.len(), types.StackLimit)
		}
		return nil
	}
}

func makeDupStackFunc(n int) stackValidationFunc {
	return makeStackFunc(n, n+1)
}

func makeSwapStackFunc(n int) stackValidationFunc {
	return makeStackFunc(n, n)
}
