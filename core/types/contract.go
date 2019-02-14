// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/BOXFoundation/boxd/crypto"
)

var tokenPrefix = [2]byte{35, 47}

// Contract is the address representation of a contract like token
type Contract interface {
	String() string
	SetString(string) error
	OutPoint() OutPoint
}

// Token stands for token address
type Token struct {
	op OutPoint
}

// NewTokenFromOutpoint creates an token adddress from outpoint
func NewTokenFromOutpoint(op OutPoint) *Token {
	return &Token{
		op: op,
	}
}

// String returns an address string starting with "b4"
func (t *Token) String() string {
	return encodeContract(t.op, tokenPrefix)
}

// SetString sets the token address with a string starting with "b4", returns an error if the input
// string is invalid
func (t *Token) SetString(in string) error {
	outpoint, err := decodeContract(in, tokenPrefix)
	if err != nil {
		return nil
	}
	t.op = *outpoint
	return nil
}

// OutPoint returns an outpoint object of the token
func (t *Token) OutPoint() OutPoint {
	return t.op
}

func encodeContract(op OutPoint, prefix [2]byte) string {
	b := make([]byte, 0, crypto.HashSize+6)
	b = append(b, op.Hash.GetBytes()...)
	buf2 := make([]byte, 4)
	binary.BigEndian.PutUint32(buf2, op.Index)
	b = append(b, buf2...)
	b = append(prefix[:], b...)
	return crypto.Base58CheckEncode(b)
}

func decodeContract(in string, prefix [2]byte) (*OutPoint, error) {
	buf, err := crypto.Base58CheckDecode(in)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(buf[:2], prefix[:]) {
		return nil, fmt.Errorf("invalid prefix")
	}
	if len(buf) != crypto.HashSize+6 {
		return nil, fmt.Errorf("invalid byte length")
	}
	hash := &crypto.HashType{}
	if err := hash.SetBytes(buf[2 : 2+crypto.HashSize]); err != nil {
		return nil, err
	}
	index := binary.BigEndian.Uint32(buf[2+crypto.HashSize:])
	return &OutPoint{
		Hash:  *hash,
		Index: index,
	}, nil
}
