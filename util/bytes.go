// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"hash/fnv"
	"io"

	base58 "github.com/jbenet/go-base58"
)

func init() {
	ReadByte = ReadUint8
	WriteByte = WriteUint8
}

// Hash by Sha3-256
type Hash []byte

// HexHash is the hex string of a hash
type HexHash string

// Hex return hex encoded hash.
func (h Hash) Hex() HexHash {
	return HexHash(Hex(h))
}

// Base58 return base58 encodes string
func (h Hash) Base58() string {
	return base58.Encode(h)
}

// Equals compare two Hash. True is equal, otherwise false.
func (h Hash) Equals(b Hash) bool {
	return bytes.Compare(h, b) == 0
}

func (h Hash) String() string {
	return string(h.Hex())
}

// Hash return hex decoded hash.
func (hh HexHash) Hash() (Hash, error) {
	v, err := FromHex(string(hh))
	if err != nil {
		return nil, err
	}
	return Hash(v), nil
}

// Hex encodes []byte to Hex.
func Hex(data []byte) string {
	return hex.EncodeToString(data)
}

// FromHex decodes string from Hex.
func FromHex(data string) ([]byte, error) {
	return hex.DecodeString(data)
}

// Uint64 encodes []byte.
func Uint64(data []byte) uint64 {
	return defaultEndian.Uint64(data)
}

// FromUint64 decodes unit64 value.
func FromUint64(v uint64) []byte {
	b := make([]byte, 8)
	defaultEndian.PutUint64(b, v)
	return b
}

// Uint32 encodes []byte.
func Uint32(data []byte) uint32 {
	return defaultEndian.Uint32(data)
}

// FromUint32 decodes uint32.
func FromUint32(v uint32) []byte {
	b := make([]byte, 4)
	defaultEndian.PutUint32(b, v)
	return b
}

// Uint16 encodes []byte.
func Uint16(data []byte) uint16 {
	return defaultEndian.Uint16(data)
}

// FromUint16 decodes uint16.
func FromUint16(v uint16) []byte {
	b := make([]byte, 2)
	defaultEndian.PutUint16(b, v)
	return b
}

// Int64 encodes []byte.
func Int64(data []byte) int64 {
	return int64(defaultEndian.Uint64(data))
}

// FromInt64 decodes int64 v.
func FromInt64(v int64) []byte {
	b := make([]byte, 8)
	defaultEndian.PutUint64(b, uint64(v))
	return b
}

// Int32 encodes []byte.
func Int32(data []byte) int32 {
	return int32(defaultEndian.Uint32(data))
}

// FromInt32 decodes int32 v.
func FromInt32(v int32) []byte {
	b := make([]byte, 4)
	defaultEndian.PutUint32(b, uint32(v))
	return b
}

// Int16 encode []byte.
func Int16(data []byte) int16 {
	return int16(defaultEndian.Uint16(data))
}

// FromInt16 decodes int16 v.
func FromInt16(v int16) []byte {
	b := make([]byte, 2)
	defaultEndian.PutUint16(b, uint16(v))
	return b
}

// Equal checks whether byte slice a and b are equal.
func Equal(a []byte, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// HashBytes return bytes hash
func HashBytes(a []byte) uint32 {
	hasherA := fnv.New32a()
	hasherA.Write(a)
	return hasherA.Sum32()
}

// Less return if a < b
func Less(a []byte, b []byte) bool {
	return HashBytes(a) < HashBytes(b)
}

// ANDBytes ands one by one. It works on all architectures, independent if
// it supports unaligned read/writes or not.
func ANDBytes(a, b []byte) []byte {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	dst := make([]byte, n)
	for i := 0; i < n; i++ {
		dst[i] = a[i] & b[i]
	}
	return dst
}

// ORBytes ors one by one. It works on all architectures, independent if
// it supports unaligned read/writes or not.
func ORBytes(a, b []byte) []byte {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	dst := make([]byte, n)
	for i := 0; i < n; i++ {
		dst[i] = a[i] | b[i]
	}
	return dst
}

// BitIndexes return marked bit indexes.
func BitIndexes(data []byte) []uint32 {
	indexes := []uint32{}
	for i := 0; i < len(data)*8; i++ {
		bitMask := byte(1) << byte(7-i%8)

		if (data[i/8] & bitMask) != 0 {
			indexes = append(indexes, uint32(i))
		}
	}
	return indexes
}

////////////////////////////////////////////////////////////////////////////////
// read/write via reader/writer

type byteReader struct {
	io.Reader
}

func (r *byteReader) ReadByte() (byte, error) {
	return ReadUint8(r)
}

// ReadHex reads Hex string.
func ReadHex(r io.Reader) (string, error) {
	data, err := ReadVarBytes(r)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

// WriteHex writes Hex string.
func WriteHex(w io.Writer, s string) error {
	data, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	return WriteVarBytes(w, data)
}

// ReadUvarint read uint64.
func ReadUvarint(r io.Reader) (uint64, error) {
	return binary.ReadUvarint(&byteReader{r})
}

// WriteUvarint writes unit64 value.
func WriteUvarint(w io.Writer, v uint64) error {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], v)
	_, err := w.Write(buf[:n])
	return err
}

// ReadVarint read uint64.
func ReadVarint(r io.Reader) (int64, error) {
	return binary.ReadVarint(&byteReader{r})
}

// WriteVarint writes unit64 value.
func WriteVarint(w io.Writer, v int64) error {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutVarint(buf[:], v)
	_, err := w.Write(buf[:n])
	return err
}

// ReadUint64 read uint64.
func ReadUint64(r io.Reader) (uint64, error) {
	return binarySerializer.Uint64(r, defaultEndian)
}

// WriteUint64 writes unit64 value.
func WriteUint64(w io.Writer, v uint64) error {
	return binarySerializer.PutUint64(w, defaultEndian, v)
}

// ReadUint32 reads uint32.
func ReadUint32(r io.Reader) (uint32, error) {
	return binarySerializer.Uint32(r, defaultEndian)
}

// WriteUint32 writes uint32.
func WriteUint32(w io.Writer, v uint32) error {
	return binarySerializer.PutUint32(w, defaultEndian, v)
}

// ReadUint16 reads uint16.
func ReadUint16(r io.Reader) (uint16, error) {
	return binarySerializer.Uint16(r, defaultEndian)
}

// WriteUint16 wtires uint16 v.
func WriteUint16(w io.Writer, v uint16) error {
	return binarySerializer.PutUint16(w, defaultEndian, v)
}

// ReadUint8 reads uint8.
func ReadUint8(r io.Reader) (uint8, error) {
	return binarySerializer.Uint8(r)
}

// WriteUint8 wtires uint8 v.
func WriteUint8(w io.Writer, v uint8) error {
	return binarySerializer.PutUint8(w, v)
}

// ReadByte reads single byte.
var ReadByte func(io.Reader) (byte, error)

// WriteByte wtires single byte.
var WriteByte func(io.Writer, byte) error

// ReadInt64 reads int64.
func ReadInt64(r io.Reader) (int64, error) {
	v, err := binarySerializer.Uint64(r, defaultEndian)
	return int64(v), err
}

// WriteInt64 writes int64 v.
func WriteInt64(w io.Writer, v int64) error {
	return binarySerializer.PutUint64(w, defaultEndian, uint64(v))
}

// ReadInt32 reads int32.
func ReadInt32(r io.Reader) (int32, error) {
	v, err := binarySerializer.Uint32(r, defaultEndian)
	return int32(v), err
}

// WriteInt32 writes int32 v.
func WriteInt32(w io.Writer, v int32) error {
	return binarySerializer.PutUint32(w, defaultEndian, uint32(v))
}

// ReadInt16 reads int16 value.
func ReadInt16(r io.Reader) (int16, error) {
	v, err := binarySerializer.Uint16(r, defaultEndian)
	return int16(v), err
}

// WriteInt16 writes int16 v.
func WriteInt16(w io.Writer, v int16) error {
	return binarySerializer.PutUint16(w, defaultEndian, uint16(v))
}

// ReadInt8 reads int8 value.
func ReadInt8(r io.Reader) (int8, error) {
	v, err := binarySerializer.Uint8(r)
	return int8(v), err
}

// WriteInt8 writes int8 v.
func WriteInt8(w io.Writer, v int8) error {
	return binarySerializer.PutUint8(w, uint8(v))
}

// ReadBytes reads fix length of []byte via reader.
func ReadBytes(r io.Reader, data []byte) error {
	_, err := io.ReadFull(r, data)
	return err
}

// ReadBytesOfLength reads specified length of []byte via reader.
func ReadBytesOfLength(r io.Reader, l uint32) ([]byte, error) {
	buf := make([]byte, l)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// WriteBytes writes fix length of bytes.
func WriteBytes(w io.Writer, v []byte) error {
	_, err := w.Write(v)
	return err
}

// ReadVarBytes reads variable length of []byte via reader.
func ReadVarBytes(r io.Reader) ([]byte, error) {
	var l, err = ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	var buf = make([]byte, l)
	if err = ReadBytes(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// WriteVarBytes writes variable length of bytes.
func WriteVarBytes(w io.Writer, v []byte) error {
	if err := WriteUvarint(w, uint64(len(v))); err != nil {
		return err
	}
	return WriteBytes(w, v)
}
