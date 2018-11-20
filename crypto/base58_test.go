// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"reflect"
	"testing"
)

func arrWithByte(len int, b byte) []byte {
	arr := make([]byte, len)
	for i := 0; i < len; i++ {
		arr[i] = b
	}
	return arr
}

func TestBase58CheckEncode(t *testing.T) {
	type args struct {
		in []byte
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "all 0",
			args: args{arrWithByte(20, 0)},
			want: "111111111111111111117K4nzc",
		},
		{
			name: "all 1",
			args: args{arrWithByte(20, 1)},
			want: "6Jswqk47s9PUcyCc88MMVwzgvHR4tFVH",
		},
		{
			name: "all 255",
			args: args{arrWithByte(20, 255)},
			want: "QLbz7JHiBTspS962RLKV8GndWFwfcDTBW",
		},
		{
			name: "extra long",
			args: args{arrWithByte(200, 10)},
			want: "SQHUvC4rAWoUDNA4rpaD4YY5zukV8HrTA8Vi2t9t4apeDiRCX5sUfoGHC3dAFt8wNeaKQdWJC6BupV7BMqhmB9vJgmzofr1RbgcKBcZW5kL3HmuJoh9VNHcT3L67aRTyQnW5wbJwzEB2qwhC4sKki8Qp4eE3eLuoXiERaoa7wS1RYFcsui6CYQuw4qHnzpKBHwbvnpGcpPw6nyhbgsPqMPidcycs7X9e6SACjdLrYY6dJXeRS8xpq5GkLCDYmrsWGpcAWJNPB4ctw54b9qLEcw",
		},
		{
			name: "empty array",
			args: args{arrWithByte(0, 0)},
			want: "3QJmnh",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Base58CheckEncode(tt.args.in); got != tt.want {
				t.Errorf("Base58CheckEncode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBase58CheckDecode(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name:    "empty input",
			args:    args{""},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty array",
			args:    args{"3QJmnh"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "normal decode",
			args:    args{"QLbz7JHiBTspS962RLKV8GndWFwfcDTBW"},
			want:    arrWithByte(20, 255),
			wantErr: false,
		},
		{
			name:    "error checksum",
			args:    args{"QLbz7JHiBTspS962RLKV8GndWFwfcDTBV"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid character",
			args:    args{"QLbz7JH_BTspS962RLKV8GndWFwfcDTBW"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid character 2",
			args:    args{"QLbz7JH0BTspS962RLKV8GndWFwfcDTBW"},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Base58CheckDecode(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("Base58CheckDecode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Base58CheckDecode() = %v, want %v", got, tt.want)
			}
		})
	}
}
