// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import "testing"

func TestInArray(t *testing.T) {
	type args struct {
		obj   interface{}
		array interface{}
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"test string",
			args{
				obj:   "aa",
				array: []string{"aa", "bb"},
			},
			true,
		},
		{
			"test string",
			args{
				obj:   "aa",
				array: []string{"a", "bb"},
			},
			false,
		},
		{
			"test []byte",
			args{
				obj:   []byte{0x11},
				array: [][]byte{[]byte{0x11}, []byte{0x22}},
			},
			true,
		},
		{
			"test []byte",
			args{
				obj:   []byte{0x11},
				array: [][]byte{[]byte{0x12}, []byte{0x22}},
			},
			false,
		},
		{
			"test byte",
			args{
				obj:   byte(0x11),
				array: []byte{0x11, 0x22},
			},
			true,
		},
		{
			"test byte",
			args{
				obj:   byte(0x11),
				array: []byte{0x12, 0x22},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InArray(tt.args.obj, tt.args.array); got != tt.want {
				t.Errorf("InArray() = %v, want %v", got, tt.want)
			}
		})
	}
}
