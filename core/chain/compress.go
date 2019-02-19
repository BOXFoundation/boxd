// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"errors"

	"github.com/BOXFoundation/boxd/script"
)

const (
	// p2pkh identifies a compressed pay-to-pubkey-hash script.
	p2pkh = 0

	// p2sh identifies a compressed pay-to-script-hash script.
	p2sh = 1

	specialScripts = 6
)

func serializeSizeVLQ(n uint64) int {
	size := 1
	for ; n > 0x7f; n = (n >> 7) - 1 {
		size++
	}

	return size
}

func putVLQ(target []byte, n uint64) int {
	offset := 0
	for ; ; offset++ {
		highBitMask := byte(0x80)
		if offset == 0 {
			highBitMask = 0x00
		}

		target[offset] = byte(n&0x7f) | highBitMask
		if n <= 0x7f {
			break
		}
		n = (n >> 7) - 1
	}

	for i, j := 0, offset; i < j; i, j = i+1, j-1 {
		target[i], target[j] = target[j], target[i]
	}

	return offset + 1
}

func deserializeVLQ(serialized []byte) (uint64, int) {
	var n uint64
	var size int
	for _, val := range serialized {
		size++
		n = (n << 7) | uint64(val&0x7f)
		if val&0x80 != 0x80 {
			break
		}
		n++
	}

	return n, size
}

func isPubKeyHash(pkscript []byte) (bool, []byte) {
	if len(pkscript) == 25 &&
		pkscript[0] == byte(script.OPDUP) &&
		pkscript[1] == byte(script.OPHASH160) &&
		pkscript[23] == byte(script.OPEQUALVERIFY) &&
		pkscript[24] == byte(script.OPCHECKSIG) {
		return true, pkscript[3:23]
	}

	return false, nil
}

func isScriptHash(pkscript []byte) (bool, []byte) {
	if len(pkscript) == 23 && pkscript[0] == byte(script.OPHASH160) &&
		pkscript[22] == byte(script.OPEQUAL) {
		return true, pkscript[2:22]
	}

	return false, nil
}

// compressedScriptSize returns the number of bytes the passed script would take
// when encoded with the domain specific compression algorithm described above.
func compressedScriptSize(pkScript []byte) int {
	// Pay-to-pubkey-hash script.
	if valid, _ := isPubKeyHash(pkScript); valid {
		return 21
	}

	// Pay-to-script-hash script.
	if valid, _ := isScriptHash(pkScript); valid {
		return 21
	}
	return serializeSizeVLQ(uint64(len(pkScript)+specialScripts)) +
		len(pkScript)
}

func decodeCompressedScriptSize(serialized []byte) int {
	scriptSize, bytesRead := deserializeVLQ(serialized)
	if bytesRead == 0 {
		return 0
	}

	switch scriptSize {
	case p2pkh:
		return 21

	case p2sh:
		return 21
	}

	scriptSize -= specialScripts
	scriptSize += uint64(bytesRead)
	return int(scriptSize)
}

func compressedScript(target, pkScript []byte) int {
	// Pay-to-pubkey-hash script.
	if valid, hash := isPubKeyHash(pkScript); valid {
		target[0] = p2pkh
		copy(target[1:21], hash)
		return 21
	}

	// Pay-to-script-hash script.
	if valid, hash := isScriptHash(pkScript); valid {
		target[0] = p2sh
		copy(target[1:21], hash)
		return 21
	}

	encodedSize := uint64(len(pkScript) + specialScripts)
	vlqSizeLen := putVLQ(target, encodedSize)
	copy(target[vlqSizeLen:], pkScript)
	return vlqSizeLen + len(pkScript)
}

func decompressScript(compressedPkScript []byte) []byte {

	if len(compressedPkScript) == 0 {
		return nil
	}

	encodedScriptSize, bytesRead := deserializeVLQ(compressedPkScript)
	switch encodedScriptSize {
	// Pay-to-pubkey-hash script.  The resulting script is:
	// <OP_DUP><OP_HASH160><20 byte hash><OP_EQUALVERIFY><OP_CHECKSIG>
	case p2pkh:
		pkScript := make([]byte, 25)
		pkScript[0] = byte(script.OPDUP)
		pkScript[1] = byte(script.OPHASH160)
		pkScript[2] = byte(script.OPDATA20)
		copy(pkScript[3:], compressedPkScript[bytesRead:bytesRead+20])
		pkScript[23] = byte(script.OPEQUALVERIFY)
		pkScript[24] = byte(script.OPCHECKSIG)
		return pkScript

	// Pay-to-script-hash script.  The resulting script is:
	// <OP_HASH160><20 byte script hash><OP_EQUAL>
	case p2sh:
		pkScript := make([]byte, 23)
		pkScript[0] = byte(script.OPHASH160)
		pkScript[1] = byte(script.OPDATA20)
		copy(pkScript[2:], compressedPkScript[bytesRead:bytesRead+20])
		pkScript[22] = byte(script.OPEQUAL)
		return pkScript

	}

	scriptSize := int(encodedScriptSize - specialScripts)
	pkScript := make([]byte, scriptSize)
	copy(pkScript, compressedPkScript[bytesRead:bytesRead+scriptSize])
	return pkScript
}

func compressTxOutValue(value uint64) uint64 {
	if value == 0 {
		return 0
	}
	exponent := uint64(0)
	for value%10 == 0 && exponent < 9 {
		value /= 10
		exponent++
	}

	if exponent < 9 {
		lastDigit := value % 10
		value /= 10
		return 1 + 10*(9*value+lastDigit-1) + exponent
	}

	return 10 + 10*(value-1)
}

func decompressTxOutValue(value uint64) uint64 {
	if value == 0 {
		return 0
	}
	value--
	exponent := value % 10
	value /= 10

	n := uint64(0)
	if exponent < 9 {
		lastDigit := value%9 + 1
		value /= 9
		n = value*10 + lastDigit
	} else {
		n = value + 1
	}

	for ; exponent > 0; exponent-- {
		n *= 10
	}

	return n
}

func compressedTxOutSize(value uint64, pkScript []byte) int {
	return serializeSizeVLQ(compressTxOutValue(value)) +
		compressedScriptSize(pkScript)
}

func compressedTxOut(target []byte, value uint64, pkScript []byte) int {
	offset := putVLQ(target, compressTxOutValue(value))
	offset += compressedScript(target[offset:], pkScript)
	return offset
}

func decodeCompressedTxOut(serialized []byte) (uint64, []byte, int, error) {

	compressedAmount, bytesRead := deserializeVLQ(serialized)
	if bytesRead >= len(serialized) {
		return 0, nil, bytesRead, errors.New("unexpected end of " +
			"data after compressed amount")
	}

	scriptSize := decodeCompressedScriptSize(serialized[bytesRead:])
	if len(serialized[bytesRead:]) < scriptSize {
		return 0, nil, bytesRead, errors.New("unexpected end of " +
			"data after script size")
	}

	// Decompress and return the amount and script.
	amount := decompressTxOutValue(compressedAmount)
	script := decompressScript(serialized[bytesRead : bytesRead+scriptSize])
	return amount, script, bytesRead + scriptSize, nil
}
