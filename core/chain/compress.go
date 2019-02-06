// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"errors"

	"github.com/BOXFoundation/boxd/script"
)

// serializeSizeVLQ returns the number of bytes it would take to serialize the
// passed number as a variable-length quantity according to the format described
// above.
func serializeSizeVLQ(n uint64) int {
	size := 1
	for ; n > 0x7f; n = (n >> 7) - 1 {
		size++
	}

	return size
}

// putVLQ serializes the provided number to a variable-length quantity according
// to the format described above and returns the number of bytes of the encoded
// value.  The result is placed directly into the passed byte slice which must
// be at least large enough to handle the number of bytes returned by the
// serializeSizeVLQ function or it will panic.
func putVLQ(target []byte, n uint64) int {
	offset := 0
	for ; ; offset++ {
		// The high bit is set when another byte follows.
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

	// Reverse the bytes so it is MSB-encoded.
	for i, j := 0, offset; i < j; i, j = i+1, j-1 {
		target[i], target[j] = target[j], target[i]
	}

	return offset + 1
}

// deserializeVLQ deserializes the provided variable-length quantity according
// to the format described above.  It also returns the number of bytes
// deserialized.
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

const (
	// cstPayToPubKeyHash identifies a compressed pay-to-pubkey-hash script.
	cstPayToPubKeyHash = 0

	// cstPayToScriptHash identifies a compressed pay-to-script-hash script.
	cstPayToScriptHash = 1

	// cstPayToPubKeyComp2 identifies a compressed pay-to-pubkey script to
	// a compressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	cstPayToPubKeyComp2 = 2

	// cstPayToPubKeyComp3 identifies a compressed pay-to-pubkey script to
	// a compressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	cstPayToPubKeyComp3 = 3

	// cstPayToPubKeyUncomp4 identifies a compressed pay-to-pubkey script to
	// an uncompressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	cstPayToPubKeyUncomp4 = 4

	// cstPayToPubKeyUncomp5 identifies a compressed pay-to-pubkey script to
	// an uncompressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	cstPayToPubKeyUncomp5 = 5

	// numSpecialScripts is the number of special scripts recognized by the
	// domain-specific script compression algorithm.
	numSpecialScripts = 6
)

// isPubKeyHash returns whether or not the passed public key script is a
// standard pay-to-pubkey-hash script along with the pubkey hash it is paying to
// if it is.
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

// isScriptHash returns whether or not the passed public key script is a
// standard pay-to-script-hash script along with the script hash it is paying to
// if it is.
func isScriptHash(pkscript []byte) (bool, []byte) {
	if len(pkscript) == 23 && pkscript[0] == byte(script.OPHASH160) &&
		pkscript[22] == byte(script.OPEQUAL) {
		return true, pkscript[2:22]
	}

	return false, nil
}

// isPubKey returns whether or not the passed public key script is a standard
// pay-to-pubkey script that pays to a valid compressed or uncompressed public
// key along with the serialized pubkey it is paying to if it is.
//
// NOTE: This function ensures the public key is actually valid since the
// compression algorithm requires valid pubkeys.  It does not support hybrid
// pubkeys.  This means that even if the script has the correct form for a
// pay-to-pubkey script, this function will only return true when it is paying
// to a valid compressed or uncompressed pubkey.
// func isPubKey(script []byte) (bool, []byte) {
// 	// Pay-to-compressed-pubkey script.
// 	if len(script) == 35 && script[0] == script.OP_DATA_33 &&
// 		script[34] == script.OP_CHECKSIG && (script[1] == 0x02 ||
// 		script[1] == 0x03) {

// 		// Ensure the public key is valid.
// 		serializedPubKey := script[1:34]
// 		_, err := btcec.ParsePubKey(serializedPubKey, btcec.S256())
// 		if err == nil {
// 			return true, serializedPubKey
// 		}
// 	}

// 	// Pay-to-uncompressed-pubkey script.
// 	if len(script) == 67 && script[0] == script.OP_DATA_65 &&
// 		script[66] == script.OP_CHECKSIG && script[1] == 0x04 {

// 		// Ensure the public key is valid.
// 		serializedPubKey := script[1:66]
// 		_, err := btcec.ParsePubKey(serializedPubKey, btcec.S256())
// 		if err == nil {
// 			return true, serializedPubKey
// 		}
// 	}

// 	return false, nil
// }

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

	// Pay-to-pubkey (compressed or uncompressed) script.
	// if valid, _ := isPubKey(pkScript); valid {
	// 	return 33
	// }

	// When none of the above special cases apply, encode the script as is
	// preceded by the sum of its size and the number of special cases
	// encoded as a variable length quantity.
	return serializeSizeVLQ(uint64(len(pkScript)+numSpecialScripts)) +
		len(pkScript)
}

// decodeCompressedScriptSize treats the passed serialized bytes as a compressed
// script, possibly followed by other data, and returns the number of bytes it
// occupies taking into account the special encoding of the script size by the
// domain specific compression algorithm described above.
func decodeCompressedScriptSize(serialized []byte) int {
	scriptSize, bytesRead := deserializeVLQ(serialized)
	if bytesRead == 0 {
		return 0
	}

	switch scriptSize {
	case cstPayToPubKeyHash:
		return 21

	case cstPayToScriptHash:
		return 21

	case cstPayToPubKeyComp2, cstPayToPubKeyComp3, cstPayToPubKeyUncomp4,
		cstPayToPubKeyUncomp5:
		return 33
	}

	scriptSize -= numSpecialScripts
	scriptSize += uint64(bytesRead)
	return int(scriptSize)
}

// putCompressedScript compresses the passed script according to the domain
// specific compression algorithm described above directly into the passed
// target byte slice.  The target byte slice must be at least large enough to
// handle the number of bytes returned by the compressedScriptSize function or
// it will panic.
func putCompressedScript(target, pkScript []byte) int {
	// Pay-to-pubkey-hash script.
	if valid, hash := isPubKeyHash(pkScript); valid {
		target[0] = cstPayToPubKeyHash
		copy(target[1:21], hash)
		return 21
	}

	// Pay-to-script-hash script.
	if valid, hash := isScriptHash(pkScript); valid {
		target[0] = cstPayToScriptHash
		copy(target[1:21], hash)
		return 21
	}

	// Pay-to-pubkey (compressed or uncompressed) script.
	// if valid, serializedPubKey := isPubKey(pkScript); valid {
	// 	pubKeyFormat := serializedPubKey[0]
	// 	switch pubKeyFormat {
	// 	case 0x02, 0x03:
	// 		target[0] = pubKeyFormat
	// 		copy(target[1:33], serializedPubKey[1:33])
	// 		return 33
	// 	case 0x04:
	// 		// Encode the oddness of the serialized pubkey into the
	// 		// compressed script type.
	// 		target[0] = pubKeyFormat | (serializedPubKey[64] & 0x01)
	// 		copy(target[1:33], serializedPubKey[1:33])
	// 		return 33
	// 	}
	// }

	// When none of the above special cases apply, encode the unmodified
	// script preceded by the sum of its size and the number of special
	// cases encoded as a variable length quantity.
	encodedSize := uint64(len(pkScript) + numSpecialScripts)
	vlqSizeLen := putVLQ(target, encodedSize)
	copy(target[vlqSizeLen:], pkScript)
	return vlqSizeLen + len(pkScript)
}

// decompressScript returns the original script obtained by decompressing the
// passed compressed script according to the domain specific compression
// algorithm described above.
//
// NOTE: The script parameter must already have been proven to be long enough
// to contain the number of bytes returned by decodeCompressedScriptSize or it
// will panic.  This is acceptable since it is only an internal function.
func decompressScript(compressedPkScript []byte) []byte {
	// In practice this function will not be called with a zero-length or
	// nil script since the nil script encoding includes the length, however
	// the code below assumes the length exists, so just return nil now if
	// the function ever ends up being called with a nil script in the
	// future.
	if len(compressedPkScript) == 0 {
		return nil
	}

	// Decode the script size and examine it for the special cases.
	encodedScriptSize, bytesRead := deserializeVLQ(compressedPkScript)
	switch encodedScriptSize {
	// Pay-to-pubkey-hash script.  The resulting script is:
	// <OP_DUP><OP_HASH160><20 byte hash><OP_EQUALVERIFY><OP_CHECKSIG>
	case cstPayToPubKeyHash:
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
	case cstPayToScriptHash:
		pkScript := make([]byte, 23)
		pkScript[0] = byte(script.OPHASH160)
		pkScript[1] = byte(script.OPDATA20)
		copy(pkScript[2:], compressedPkScript[bytesRead:bytesRead+20])
		pkScript[22] = byte(script.OPEQUAL)
		return pkScript

	// Pay-to-compressed-pubkey script.  The resulting script is:
	// <OP_DATA_33><33 byte compressed pubkey><OP_CHECKSIG>
	case cstPayToPubKeyComp2, cstPayToPubKeyComp3:
		pkScript := make([]byte, 35)
		// pkScript[0] = script.OP_DATA_33
		pkScript[1] = byte(encodedScriptSize)
		copy(pkScript[2:], compressedPkScript[bytesRead:bytesRead+32])
		pkScript[34] = byte(script.OPCHECKSIG)
		return pkScript

		// Pay-to-uncompressed-pubkey script.  The resulting script is:
		// <OP_DATA_65><65 byte uncompressed pubkey><OP_CHECKSIG>
		// case cstPayToPubKeyUncomp4, cstPayToPubKeyUncomp5:
		// 	// Change the leading byte to the appropriate compressed pubkey
		// 	// identifier (0x02 or 0x03) so it can be decoded as a
		// 	// compressed pubkey.  This really should never fail since the
		// 	// encoding ensures it is valid before compressing to this type.
		// 	compressedKey := make([]byte, 33)
		// 	compressedKey[0] = byte(encodedScriptSize - 2)
		// 	copy(compressedKey[1:], compressedPkScript[1:])
		// 	key, err := btcec.ParsePubKey(compressedKey, btcec.S256())
		// 	if err != nil {
		// 		return nil
		// 	}

		// 	pkScript := make([]byte, 67)
		// 	pkScript[0] = script.OP_DATA_65
		// 	copy(pkScript[1:], key.SerializeUncompressed())
		// 	pkScript[66] = script.OP_CHECKSIG
		// 	return pkScript
	}

	// When none of the special cases apply, the script was encoded using
	// the general format, so reduce the script size by the number of
	// special cases and return the unmodified script.
	scriptSize := int(encodedScriptSize - numSpecialScripts)
	pkScript := make([]byte, scriptSize)
	copy(pkScript, compressedPkScript[bytesRead:bytesRead+scriptSize])
	return pkScript
}

// -----------------------------------------------------------------------------
// In order to reduce the size of stored amounts, a domain specific compression
// algorithm is used which relies on there typically being a lot of zeroes at
// end of the amounts.  The compression algorithm used here was obtained from
// Bitcoin Core, so all credits for the algorithm go to it.
//
// While this is simply exchanging one uint64 for another, the resulting value
// for typical amounts has a much smaller magnitude which results in fewer bytes
// when encoded as variable length quantity.  For example, consider the amount
// of 0.1 BTC which is 10000000 satoshi.  Encoding 10000000 as a VLQ would take
// 4 bytes while encoding the compressed value of 8 as a VLQ only takes 1 byte.
//
// Essentially the compression is achieved by splitting the value into an
// exponent in the range [0-9] and a digit in the range [1-9], when possible,
// and encoding them in a way that can be decoded.  More specifically, the
// encoding is as follows:
// - 0 is 0
// - Find the exponent, e, as the largest power of 10 that evenly divides the
//   value up to a maximum of 9
// - When e < 9, the final digit can't be 0 so store it as d and remove it by
//   dividing the value by 10 (call the result n).  The encoded value is thus:
//   1 + 10*(9*n + d-1) + e
// - When e==9, the only thing known is the amount is not 0.  The encoded value
//   is thus:
//   1 + 10*(n-1) + e   ==   10 + 10*(n-1)
//
// Example encodings:
// (The numbers in parenthesis are the number of bytes when serialized as a VLQ)
//            0 (1) -> 0        (1)           *  0.00000000 BTC
//         1000 (2) -> 4        (1)           *  0.00001000 BTC
//        10000 (2) -> 5        (1)           *  0.00010000 BTC
//     12345678 (4) -> 111111101(4)           *  0.12345678 BTC
//     50000000 (4) -> 47       (1)           *  0.50000000 BTC
//    100000000 (4) -> 9        (1)           *  1.00000000 BTC
//    500000000 (5) -> 49       (1)           *  5.00000000 BTC
//   1000000000 (5) -> 10       (1)           * 10.00000000 BTC
// -----------------------------------------------------------------------------

// compressTxOutAmount compresses the passed amount according to the domain
// specific compression algorithm described above.
func compressTxOutAmount(amount uint64) uint64 {
	// No need to do any work if it's zero.
	if amount == 0 {
		return 0
	}

	// Find the largest power of 10 (max of 9) that evenly divides the
	// value.
	exponent := uint64(0)
	for amount%10 == 0 && exponent < 9 {
		amount /= 10
		exponent++
	}

	// The compressed result for exponents less than 9 is:
	// 1 + 10*(9*n + d-1) + e
	if exponent < 9 {
		lastDigit := amount % 10
		amount /= 10
		return 1 + 10*(9*amount+lastDigit-1) + exponent
	}

	// The compressed result for an exponent of 9 is:
	// 1 + 10*(n-1) + e   ==   10 + 10*(n-1)
	return 10 + 10*(amount-1)
}

// decompressTxOutAmount returns the original amount the passed compressed
// amount represents according to the domain specific compression algorithm
// described above.
func decompressTxOutAmount(amount uint64) uint64 {
	// No need to do any work if it's zero.
	if amount == 0 {
		return 0
	}

	// The decompressed amount is either of the following two equations:
	// x = 1 + 10*(9*n + d - 1) + e
	// x = 1 + 10*(n - 1)       + 9
	amount--

	// The decompressed amount is now one of the following two equations:
	// x = 10*(9*n + d - 1) + e
	// x = 10*(n - 1)       + 9
	exponent := amount % 10
	amount /= 10

	// The decompressed amount is now one of the following two equations:
	// x = 9*n + d - 1  | where e < 9
	// x = n - 1        | where e = 9
	n := uint64(0)
	if exponent < 9 {
		lastDigit := amount%9 + 1
		amount /= 9
		n = amount*10 + lastDigit
	} else {
		n = amount + 1
	}

	// Apply the exponent.
	for ; exponent > 0; exponent-- {
		n *= 10
	}

	return n
}

// -----------------------------------------------------------------------------
// Compressed transaction outputs consist of an amount and a public key script
// both compressed using the domain specific compression algorithms previously
// described.
//
// The serialized format is:
//
//   <compressed amount><compressed script>
//
//   Field                 Type     Size
//     compressed amount   VLQ      variable
//     compressed script   []byte   variable
// -----------------------------------------------------------------------------

// compressedTxOutSize returns the number of bytes the passed transaction output
// fields would take when encoded with the format described above.
func compressedTxOutSize(amount uint64, pkScript []byte) int {
	return serializeSizeVLQ(compressTxOutAmount(amount)) +
		compressedScriptSize(pkScript)
}

// putCompressedTxOut compresses the passed amount and script according to their
// domain specific compression algorithms and encodes them directly into the
// passed target byte slice with the format described above.  The target byte
// slice must be at least large enough to handle the number of bytes returned by
// the compressedTxOutSize function or it will panic.
func putCompressedTxOut(target []byte, amount uint64, pkScript []byte) int {
	offset := putVLQ(target, compressTxOutAmount(amount))
	offset += putCompressedScript(target[offset:], pkScript)
	return offset
}

// decodeCompressedTxOut decodes the passed compressed txout, possibly followed
// by other data, into its uncompressed amount and script and returns them along
// with the number of bytes they occupied prior to decompression.
func decodeCompressedTxOut(serialized []byte) (uint64, []byte, int, error) {
	// Deserialize the compressed amount and ensure there are bytes
	// remaining for the compressed script.
	compressedAmount, bytesRead := deserializeVLQ(serialized)
	if bytesRead >= len(serialized) {
		return 0, nil, bytesRead, errors.New("unexpected end of " +
			"data after compressed amount")
	}

	// Decode the compressed script size and ensure there are enough bytes
	// left in the slice for it.
	scriptSize := decodeCompressedScriptSize(serialized[bytesRead:])
	if len(serialized[bytesRead:]) < scriptSize {
		return 0, nil, bytesRead, errors.New("unexpected end of " +
			"data after script size")
	}

	// Decompress and return the amount and script.
	amount := decompressTxOutAmount(compressedAmount)
	script := decompressScript(serialized[bytesRead : bytesRead+scriptSize])
	return amount, script, bytesRead + scriptSize, nil
}
