// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blocksync

import (
	"reflect"
	"testing"
)

func runTestHeightLocator(t *testing.T, got, expect []int32) {
	if eq := reflect.DeepEqual(got, expect); !eq {
		t.Fatalf("want: %v, got: %v", expect, got)
	}
}

func TestHeightLocator(t *testing.T) {
	hh := heightLocator(20)
	expect := []int32{20, 19, 18, 17, 16, 15, 13, 10, 5, 0}
	runTestHeightLocator(t, hh, expect)

	hh = heightLocator(3)
	expect = []int32{3, 2, 1, 0}
	runTestHeightLocator(t, hh, expect)

	hh = heightLocator(5)
	expect = []int32{5, 4, 3, 2, 1, 0}
	runTestHeightLocator(t, hh, expect)

	hh = heightLocator(6)
	expect = []int32{6, 5, 4, 3, 2, 1, 0}
	runTestHeightLocator(t, hh, expect)

	hh = heightLocator(100)
	expect = []int32{100, 99, 98, 97, 96, 95, 93, 90, 85, 76, 59, 26, 0}
	runTestHeightLocator(t, hh, expect)

	hh = heightLocator(9)
	expect = []int32{9, 8, 7, 6, 5, 4, 2, 0}
	runTestHeightLocator(t, hh, expect)
}
