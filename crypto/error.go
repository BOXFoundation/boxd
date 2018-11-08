// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import "errors"

// error
var (
	//base58.go
	ErrInvalidBase58Encoding     = errors.New("Invalid base58 encoding")
	ErrInvalidBase58Checksum     = errors.New("Invalid base58 checksum")
	ErrInvalidBase58StringLength = errors.New("Invalid base58 string length, not enough bytes for checksum")
)
