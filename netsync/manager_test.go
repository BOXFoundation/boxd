package netsync

import "testing"

func runTestHeightLocator(t *testing.T, got, expect []int32) {
	if len(got) != len(expect) {
		t.Fatalf("want: %v, got: %v", expect, got)
	}
	for i := 0; i < len(got); i++ {
		if got[i] != expect[i] {
			t.Fatalf("want: %v, got: %v", expect, got)
		}
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
	hh = heightLocator(100)
	expect = []int32{100, 99, 98, 97, 96, 95, 93, 90, 85, 76, 59, 26, 0}
	runTestHeightLocator(t, hh, expect)
}
