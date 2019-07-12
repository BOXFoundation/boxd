// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package math

import (
	"fmt"
	"strconv"
)

// Integer limit values.
const (
	MaxUint64 = 1<<64 - 1
)

// HexOrDecimal64 marshals uint64 as hex or decimal.
type HexOrDecimal64 uint64

// UnmarshalText implements encoding.TextUnmarshaler.
func (i *HexOrDecimal64) UnmarshalText(input []byte) error {
	int, ok := ParseUint64(string(input))
	if !ok {
		return fmt.Errorf("invalid hex or decimal integer %q", input)
	}
	*i = HexOrDecimal64(int)
	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (i HexOrDecimal64) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%#x", uint64(i))), nil
}

// ParseUint64 parses s as an integer in decimal or hexadecimal syntax.
// Leading zeros are accepted. The empty string parses as zero.
func ParseUint64(s string) (uint64, bool) {
	if s == "" {
		return 0, true
	}
	if len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X") {
		v, err := strconv.ParseUint(s[2:], 16, 64)
		return v, err == nil
	}
	v, err := strconv.ParseUint(s, 10, 64)
	return v, err == nil
}

// NOTE: The following methods need to be optimised using either bit checking or asm

// SafeSub returns subtraction result and whether overflow occurred.
func SafeSub(x, y uint64) (uint64, bool) {
	return x - y, x < y
}

// SafeAdd returns the result and whether overflow occurred.
func SafeAdd(x, y uint64) (uint64, bool) {
	return x + y, y > MaxUint64-x
}

// SafeMul returns multiplication result and whether overflow occurred.
func SafeMul(x, y uint64) (uint64, bool) {
	if x == 0 || y == 0 {
		return 0, false
	}
	return x * y, y > MaxUint64/x
}
