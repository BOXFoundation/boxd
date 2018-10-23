// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package key

import (
	"testing"

	"github.com/facebookgo/ensure"
)

func TestKeyAncestry(t *testing.T) {
	k1 := NewKey("/A/B/C")
	k2 := NewKey("/A/B/C/D")

	ensure.DeepEqual(t, k1.String(), "/A/B/C")
	ensure.DeepEqual(t, k2.String(), "/A/B/C/D")
	ensure.True(t, k1.IsAncestorOf(k2))
	ensure.True(t, k2.IsDescendantOf(k1))
	ensure.True(t, NewKey("/A").IsAncestorOf(k2))
	ensure.True(t, NewKey("/A").IsAncestorOf(k1))
	ensure.True(t, !NewKey("/A").IsDescendantOf(k2))
	ensure.True(t, !NewKey("/A").IsDescendantOf(k1))
	ensure.True(t, k2.IsDescendantOf(NewKey("/A")))
	ensure.True(t, k1.IsDescendantOf(NewKey("/A")))
	ensure.True(t, !k2.IsAncestorOf(NewKey("/A")))
	ensure.True(t, !k1.IsAncestorOf(NewKey("/A")))
	ensure.True(t, !k2.IsAncestorOf(k2))
	ensure.True(t, !k1.IsAncestorOf(k1))
	ensure.DeepEqual(t, k1.Child(NewKey("D")).String(), k2.String())
	ensure.DeepEqual(t, k1.ChildString("D").String(), k2.String())
	ensure.DeepEqual(t, k1.String(), k2.Parent().String())
	ensure.DeepEqual(t, k1.Parent().String(), k2.Parent().Parent().String())
}

func TestLess(t *testing.T) {
	checkLess := func(a, b string) {
		ak := NewKey(a)
		bk := NewKey(b)
		ensure.True(t, ak.Less(bk))
		ensure.False(t, bk.Less(ak))
	}

	checkLess("/a/b/c", "/a/b/c/d")
	checkLess("/a/b", "/a/b/c/d")
	checkLess("/a", "/a/b/c/d")
	checkLess("/a/a/c", "/a/b/c")
	checkLess("/a/a/d", "/a/b/c")
	checkLess("/a/b/c/d/e/f/g/h", "/b")
	checkLess("/", "/a")
}
