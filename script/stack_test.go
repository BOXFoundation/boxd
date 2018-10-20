// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

import (
	"testing"

	"github.com/facebookgo/ensure"
)

func TestStack(t *testing.T) {
	const n = 100

	s := newStack()
	ensure.True(t, s.empty())
	ensure.NotNil(t, s.validateTop())

	for i := 0; i < n; i++ {
		s.push(Operand{(byte)(i)})
	}
	ensure.DeepEqual(t, s.size(), n)

	ensure.True(t, s.topN(0) == nil)
	for i := 0; i < n; i++ {
		ensure.DeepEqual(t, s.topN(i+1), Operand{(byte)(n - i - 1)})
	}
	ensure.True(t, s.topN(n+1) == nil)

	for i := 0; i < n; i++ {
		if i < n-1 {
			ensure.Nil(t, s.validateTop())
		} else {
			ensure.NotNil(t, s.validateTop())
		}
		ensure.DeepEqual(t, s.pop(), Operand{(byte)(n - i - 1)})
	}
}
