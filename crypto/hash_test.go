// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"bytes"
	"reflect"
	"testing"
)

// test RIPEMD160 hash
func TestRipemd160(t *testing.T) {
	// empty string
	expectDigest := []byte{156, 17, 133, 165, 197, 233, 252, 84, 97, 40, 8, 151, 126, 232, 245, 72, 178, 37, 141, 49}
	emptyStringDigest := Ripemd160([]byte(""))
	if !reflect.DeepEqual(emptyStringDigest, expectDigest) {
		t.Errorf("Ripemd160 digest = %v, expects %v", emptyStringDigest, expectDigest)
	}
}

// test SHA256 hash
func TestSha256(t *testing.T) {
	// empty string
	expectDigest := []byte{227, 176, 196, 66, 152, 252, 28, 20, 154, 251, 244, 200, 153, 111, 185, 36, 39, 174, 65, 228, 100, 155, 147, 76, 164, 149, 153, 27, 120, 82, 184, 85}
	emptyStringDigest := Sha256([]byte(""))
	if !reflect.DeepEqual(emptyStringDigest, expectDigest) {
		t.Errorf("Sha256 digest = %v, expects %v", emptyStringDigest, expectDigest)
	}
}

func TestSetString(t *testing.T) {
	hexString := "7c3040dcb540cc57f8c4ed08dbcfba807434dc861c94a1c161b099f58d9ebe6d"
	hash := &HashType{}
	hash.SetString(hexString)
	if hash.String() != hexString {
		t.Errorf("Error setting string to hash\nexpected: %s\nactual: %s", hexString, hash.String())
	}
}

func TestHashType_SetString(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name    string
		hash    *HashType
		args    args
		wantErr bool
	}{
		{
			name:    "error encoding",
			hash:    &HashType{},
			args:    args{"123x"},
			wantErr: true,
		},
		{
			name:    "incorrect length",
			hash:    &HashType{},
			args:    args{"1234"},
			wantErr: true,
		},
		{
			name:    "normal hash",
			hash:    &HashType{},
			args:    args{"7c3040dcb540cc57f8c4ed08dbcfba807434dc861c94a1c161b099f58d9ebe6d"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.hash.SetString(tt.args.str); (err != nil) != tt.wantErr {
				t.Errorf("HashType.SetString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHashType_String(t *testing.T) {
	testStr1 := "7c3040dcb540cc57f8c4ed08db00ba807434dc861c94a1c161b099f58d9ebe6d"
	hash1 := &HashType{}
	hash1.SetString(testStr1)

	testStr2 := "f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0"
	hash2 := &HashType{}
	hash2.SetString(testStr2)
	tests := []struct {
		name string
		hash HashType
		want string
	}{
		{
			name: "string 1",
			hash: *hash1,
			want: testStr1,
		},
		{
			name: "string 2",
			hash: *hash2,
			want: testStr2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hash.String(); got != tt.want {
				t.Errorf("HashType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_reverseBytes(t *testing.T) {
	type args struct {
		buf       []byte
		bufExpect []byte
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "empty",
			args: args{
				buf:       []byte{},
				bufExpect: []byte{},
			},
		},
		{
			name: "size 1",
			args: args{
				buf:       []byte{0x01},
				bufExpect: []byte{0x01},
			},
		},
		{
			name: "size 3",
			args: args{
				buf:       []byte{0x01, 0x02, 0x03},
				bufExpect: []byte{0x03, 0x02, 0x01},
			},
		},
		{
			name: "size 4",
			args: args{
				buf:       []byte{0x01, 0x02, 0x03, 0x04},
				bufExpect: []byte{0x04, 0x03, 0x02, 0x01},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reverseBytes(tt.args.buf)
			if !bytes.Equal(tt.args.buf, tt.args.bufExpect) {
				t.Errorf("reverseBytes actual = %v, want %v", tt.args.buf, tt.args.bufExpect)
			}
		})
	}
}

func TestSha256Multi(t *testing.T) {
	type args struct {
		data [][]byte
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "multi 1",
			args: args{
				data: [][]byte{[]byte("contentbox"), []byte("contentbox")},
			},
			want: []byte{0x6a, 0xcf, 0x6e, 0xae, 0x9a, 0x5b, 0xad, 0x6d, 0x35, 0x95, 0x49, 0x8a, 0x2c, 0x1, 0xf5, 0xd2, 0x6a, 0x95, 0xd, 0x94, 0x6c, 0xca, 0x82, 0x70, 0xb0, 0x4b, 0x7c, 0xe5, 0xf3, 0x76, 0x8a, 0xd0},
		},
		{
			name: "multi 2",
			args: args{
				data: [][]byte{[]byte("blockchain"), []byte("blockchain"), []byte("blockchain")},
			},
			want: []byte{0x3f, 0x5c, 0xfa, 0xfa, 0x8d, 0xe4, 0xa7, 0xec, 0xd8, 0x4f, 0x74, 0x6a, 0xce, 0xe0, 0x94, 0x1b, 0x5b, 0x3e, 0x4a, 0x64, 0xea, 0xf8, 0xec, 0x70, 0x45, 0xc6, 0xae, 0x17, 0xa6, 0x8a, 0xfc, 0xb0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Sha256Multi(tt.args.data...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sha256Multi() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDoubleHashH(t *testing.T) {
	type args struct {
		b []byte
	}
	tests := []struct {
		name string
		args args
		want HashType
	}{
		{
			name: "empty",
			args: args{[]byte("")},
			want: HashType{0x5d, 0xf6, 0xe0, 0xe2, 0x76, 0x13, 0x59, 0xd3, 0xa, 0x82, 0x75, 0x5, 0x8e, 0x29, 0x9f, 0xcc, 0x3, 0x81, 0x53, 0x45, 0x45, 0xf5, 0x5c, 0xf4, 0x3e, 0x41, 0x98, 0x3f, 0x5d, 0x4c, 0x94, 0x56},
		},
		{
			name: "doublehash 1",
			args: args{[]byte("contentbox")},
			want: HashType{0x92, 0xe, 0x4a, 0xf5, 0xf8, 0x1e, 0xaf, 0x69, 0xd8, 0xeb, 0xac, 0xef, 0xaa, 0x2d, 0x63, 0x58, 0x91, 0x69, 0xa5, 0xbb, 0x99, 0x97, 0x88, 0xb4, 0x34, 0x89, 0x9c, 0x7e, 0x57, 0x57, 0x3b, 0x1a},
		},
		{
			name: "doublehash 2",
			args: args{[]byte("blockchain")},
			want: HashType{0x97, 0x67, 0x58, 0x15, 0xe2, 0x5e, 0x3f, 0x37, 0xf2, 0x6f, 0x47, 0x83, 0xba, 0x74, 0xe, 0xca, 0xa9, 0xb4, 0xfa, 0x28, 0x72, 0x6, 0x9c, 0xf, 0xdf, 0x1f, 0x2c, 0xc, 0x1e, 0x7f, 0x59, 0xd},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DoubleHashH(tt.args.b); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DoubleHashH() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHash160(t *testing.T) {
	type args struct {
		b []byte
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "empty",
			args: args{[]byte("")},
			want: []byte{0xb4, 0x72, 0xa2, 0x66, 0xd0, 0xbd, 0x89, 0xc1, 0x37, 0x6, 0xa4, 0x13, 0x2c, 0xcf, 0xb1, 0x6f, 0x7c, 0x3b, 0x9f, 0xcb},
		},
		{
			name: "hash160 1",
			args: args{[]byte("contentbox")},
			want: []byte{0x94, 0x7b, 0x3f, 0x67, 0xb, 0xba, 0x5f, 0xdd, 0xa4, 0x1a, 0x38, 0xb8, 0x19, 0xf5, 0xf8, 0x89, 0x23, 0x8c, 0xa9, 0xdf},
		},
		{
			name: "hash160 2",
			args: args{[]byte("blockchain")},
			want: []byte{0x75, 0x5f, 0x6f, 0x4a, 0xf6, 0xe1, 0x1c, 0x5c, 0xf6, 0x42, 0xf0, 0xed, 0x6e, 0xcd, 0xa8, 0x9d, 0x86, 0x19, 0xce, 0xe7},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Hash160(tt.args.b); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Hash160() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashType_IsEqual(t *testing.T) {
	hash1 := DoubleHashH([]byte("contentbox"))
	hash1Equal := DoubleHashH([]byte("contentbox"))
	hash2 := DoubleHashH([]byte("blockchain"))
	type args struct {
		target *HashType
	}
	tests := []struct {
		name string
		hash *HashType
		args args
		want bool
	}{
		{
			name: "both nil",
			hash: nil,
			args: args{nil},
			want: true,
		},
		{
			name: "one nil",
			hash: &HashType{},
			args: args{nil},
			want: false,
		},
		{
			name: "equal",
			hash: &hash1,
			args: args{&hash1Equal},
			want: true,
		},
		{
			name: "not equal",
			hash: &hash1,
			args: args{&hash2},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hash.IsEqual(tt.args.target); got != tt.want {
				t.Errorf("HashType.IsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashType_SetBytes(t *testing.T) {
	type args struct {
		hashBytes []byte
	}
	tests := []struct {
		name    string
		hash    *HashType
		args    args
		wantErr bool
	}{
		{
			name:    "wrong size",
			hash:    &HashType{},
			args:    args{[]byte("")},
			wantErr: true,
		},
		{
			name:    "correct size",
			hash:    &HashType{},
			args:    args{make([]byte, HashSize)},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.hash.SetBytes(tt.args.hashBytes); (err != nil) != tt.wantErr {
				t.Errorf("HashType.SetBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHashType_GetBytes(t *testing.T) {
	tests := []struct {
		name string
		hash *HashType
		want []byte
	}{
		{
			name: "init value",
			hash: &HashType{},
			want: make([]byte, HashSize),
		},
		{
			name: "real value",
			hash: &HashType{0x92, 0xe, 0x4a, 0xf5, 0xf8, 0x1e, 0xaf, 0x69, 0xd8, 0xeb, 0xac, 0xef, 0xaa, 0x2d, 0x63, 0x58, 0x91, 0x69, 0xa5, 0xbb, 0x99, 0x97, 0x88, 0xb4, 0x34, 0x89, 0x9c, 0x7e, 0x57, 0x57, 0x3b, 0x1a},
			want: []byte{0x92, 0xe, 0x4a, 0xf5, 0xf8, 0x1e, 0xaf, 0x69, 0xd8, 0xeb, 0xac, 0xef, 0xaa, 0x2d, 0x63, 0x58, 0x91, 0x69, 0xa5, 0xbb, 0x99, 0x97, 0x88, 0xb4, 0x34, 0x89, 0x9c, 0x7e, 0x57, 0x57, 0x3b, 0x1a},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hash.GetBytes(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HashType.GetBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkHash160(b *testing.B) {
	msg := []byte("1234567890-=qwertyuiop[]\asdfghjkl;'zxcvbnm,./")
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Hash160(msg)
		}
	})
}

func BenchmarkSha256(b *testing.B) {
	msg := []byte("1234567890-=qwertyuiop[]\asdfghjkl;'zxcvbnm,./")
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Sha256(msg)
		}
	})
}
