// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/util"
	vmcrypto "github.com/BOXFoundation/boxd/vm/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/ripemd160"
)

//
var (
	AddressTypeP2PKHPrefix     = [2]byte{FixBoxPrefix, FixP2PKHPrefix} // p2pkh addresses start with b1
	AddrTypeP2PKHPrefix        = "b1"
	AddressTypeSplitAddrPrefix = [2]byte{FixBoxPrefix, FixSplitPrefix} // b2
	AddrTypeSplitAddrPrefix    = "b2"
	AddressTypeP2SHPrefix      = [2]byte{FixBoxPrefix, FixP2SHPrefix} // b3
	AddrTypeP2SHPrefix         = "b3"
	AddrTypeContractPrefix     = "b5"
	AddressTypeContractPrefix  = [2]byte{FixBoxPrefix, FixContractPrefix} // b5

	ZeroAddressHash = AddressHash{}

	_ Address = (*AddressPubKeyHash)(nil)
	_ Address = (*AddressTypeSplit)(nil)
	_ Address = (*AddressContract)(nil)
)

// const
const (
	BoxPrefix           = 'b'
	AddressPrefixLength = 2
	FixBoxPrefix        = 0x13
	FixP2PKHPrefix      = 0x26
	FixSplitPrefix      = 0x28
	FixP2SHPrefix       = 0x2a
	FixContractPrefix   = 0x30

	AddressLength       = 26
	EncodeAddressLength = 35

	AddressUnkownType AddressType = iota
	AddressP2PKHType
	AddressSplitType
	AddressP2SHType
	AddressContractType
)

// AddressType defines address type
type AddressType int

// AddressHash Alias for address hash
type AddressHash [ripemd160.Size]byte

// NewAddressHash news a AddressHash
func NewAddressHash(b []byte) *AddressHash {
	a := new(AddressHash)
	a.SetBytes(b)
	return a
}

// SetBytes set bytes for a addressHash.
func (a *AddressHash) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-ripemd160.Size:]
	}
	copy(a[ripemd160.Size-len(b):], b)
}

// Bytes returns the bytes of a addressHash.
func (a *AddressHash) Bytes() []byte {
	if a == nil {
		return nil
	}
	return a[:]
}

// MarshalJSON implements Log json marshaler interface
func (a AddressHash) MarshalJSON() ([]byte, error) {
	out := hex.EncodeToString(a[:])
	return json.Marshal(out)
}

// BytesToAddressHash converts bytes to addressHash.
func BytesToAddressHash(b []byte) AddressHash {
	var a AddressHash
	a.SetBytes(b)
	return a
}

// Big converts an address to a big integer.
func (a AddressHash) Big() *big.Int { return new(big.Int).SetBytes(a[:]) }

// String returns a human-readable string for the address hash.
func (a AddressHash) String() string {
	return encodeAddress(a[:], AddressTypeP2PKHPrefix)
}

// BigToAddressHash returns Address with byte values of b.
func BigToAddressHash(b *big.Int) AddressHash { return BytesToAddressHash(b.Bytes()) }

// HexToAddressHash returns AddressHash with byte values of s.
// If s is larger than len(h), s will be cropped from the left.
func HexToAddressHash(s string) AddressHash {
	hexBytes, _ := util.FromHex(s)
	return BytesToAddressHash(hexBytes)
}

// Address is an interface type for any type of destination a transaction output may spend to.
type Address interface {
	Type() AddressType
	String() string
	SetString(string) error
	Hash() []byte
	Hash160() *AddressHash
}

// NormalizeAddressHash converts AddressHash to common Hash
func NormalizeAddressHash(addrHash *AddressHash) *crypto.HashType {
	if addrHash == nil || *addrHash == ZeroAddressHash {
		return new(crypto.HashType)
	}
	hash := new(crypto.HashType)
	copy(hash[12:], addrHash[:])
	return hash
}

// AddressPubKeyHash is an Address for a pay-to-pubkey-hash (P2PKH) transaction.
type AddressPubKeyHash struct {
	hash AddressHash
}

// NewAddressPubKeyHash returns a new AddressPubKeyHash.  pkHash mustbe 20 bytes.
func NewAddressPubKeyHash(pkHash []byte) (*AddressPubKeyHash, error) {
	return newAddressPubKeyHash(pkHash)
}

// NewAddressFromPubKey returns a new AddressPubKeyHash derived from an ecdsa public key
func NewAddressFromPubKey(pubKey *crypto.PublicKey) (*AddressPubKeyHash, error) {
	pkHash := crypto.Hash160(pubKey.Serialize())
	return newAddressPubKeyHash(pkHash)
}

// NewAddress creates an address from string
func NewAddress(address string) (*AddressPubKeyHash, error) {
	addr := &AddressPubKeyHash{}
	err := addr.SetString(address)
	return addr, err
}

func newAddressPubKeyHash(pkHash []byte) (*AddressPubKeyHash, error) {
	// Check for a valid pubkey hash length.
	if len(pkHash) != ripemd160.Size {
		return nil, core.ErrInvalidPKHash
	}

	addr := &AddressPubKeyHash{}
	copy(addr.hash[:], pkHash)
	return addr, nil
}

// Hash returns the bytes to be included in a txout script to pay to a pubkey hash.
func (a *AddressPubKeyHash) Hash() []byte {
	if a == nil {
		return nil
	}
	return a.hash[:]
}

// Type returns AddressP2PKHType
func (a *AddressPubKeyHash) Type() AddressType {
	return AddressP2PKHType
}

// String returns a human-readable string for the pay-to-pubkey-hash address.
func (a *AddressPubKeyHash) String() string {
	if a == nil {
		return ""
	}
	return encodeAddress(a.hash[:], AddressTypeP2PKHPrefix)
}

// SetString sets the Address's internal byte array using byte array decoded from input
// base58 format string, returns error if input string is invalid
func (a *AddressPubKeyHash) SetString(in string) error {
	if len(in) != EncodeAddressLength || in[0] != BoxPrefix {
		return core.ErrInvalidAddressString
	}
	rawBytes, err := crypto.Base58CheckDecode(in)
	if err != nil {
		return err
	}
	if len(rawBytes) != 22 {
		return core.ErrInvalidAddressString
	}
	var prefix [2]byte
	copy(prefix[:], rawBytes[:2])
	if prefix != AddressTypeP2PKHPrefix && prefix != AddressTypeP2SHPrefix {
		return core.ErrInvalidAddressString
	}
	copy(a.hash[:], rawBytes[2:])
	return nil
}

// Hash160 returns the underlying array of the pubkey hash.
func (a *AddressPubKeyHash) Hash160() *AddressHash {
	if a == nil {
		return nil
	}
	return &a.hash
}

// NewSplitAddress creates an address with a string prefixed by "b2"
func NewSplitAddress(address string) (*AddressTypeSplit, error) {
	addr := new(AddressTypeSplit)
	err := addr.SetString(address)
	return addr, err
}

// NewSplitAddressFromHash creates an address with byte array of size 20
func NewSplitAddressFromHash(hash []byte) (*AddressTypeSplit, error) {
	if len(hash) != ripemd160.Size {
		return nil, core.ErrInvalidPKHash
	}

	addr := &AddressTypeSplit{}
	copy(addr.hash[:], hash)
	return addr, nil
}

// AddressTypeSplit stands for split address
type AddressTypeSplit struct {
	hash AddressHash
}

// Type returns AddressSplitType
func (a *AddressTypeSplit) Type() AddressType {
	return AddressSplitType
}

// String returns a human-readable string for the split address.
func (a *AddressTypeSplit) String() string {
	if a == nil {
		return ""
	}
	return encodeAddress(a.hash[:], AddressTypeSplitAddrPrefix)
}

// SetString sets the Address's internal byte array using byte array decoded from input
// base58 format string, returns error if input string is invalid
func (a *AddressTypeSplit) SetString(in string) error {
	if len(in) != EncodeAddressLength || in[0] != BoxPrefix {
		return core.ErrInvalidAddressString
	}
	rawBytes, err := crypto.Base58CheckDecode(in)
	if err != nil {
		return err
	}
	if len(rawBytes) != 22 {
		return core.ErrInvalidAddressString
	}
	var prefix [2]byte
	copy(prefix[:], rawBytes[:2])
	if prefix != AddressTypeSplitAddrPrefix {
		return core.ErrInvalidAddressString
	}
	copy(a.hash[:], rawBytes[2:])
	return nil
}

// Hash returns the bytes to be included in a txout script to pay to a split addr.
func (a *AddressTypeSplit) Hash() []byte {
	if a == nil {
		return nil
	}
	return a.hash[:]
}

// Hash160 returns the underlying array of the pubkey hash.
func (a *AddressTypeSplit) Hash160() *AddressHash {
	if a == nil {
		return nil
	}
	return &a.hash
}

// AddressContract is an Address for a contract
type AddressContract struct {
	hash AddressHash
}

// Hash returns the bytes to be included in a txout script to pay to contract
func (a *AddressContract) Hash() []byte {
	if a == nil {
		return nil
	}
	return a.hash[:]
}

// Type returns AddressContractType
func (a *AddressContract) Type() AddressType {
	return AddressContractType
}

// String returns a human-readable string for the split address.
func (a *AddressContract) String() string {
	if a == nil {
		return ""
	}
	return encodeAddress(a.hash[:], AddressTypeContractPrefix)
}

// NewContractAddress returns a new contract address.  pkHash mustbe 20 bytes.
func NewContractAddress(address string) (*AddressContract, error) {
	addr := new(AddressContract)
	err := addr.SetString(address)
	return addr, err
}

// NewContractAddressFromHash creates an contract address from hash
func NewContractAddressFromHash(hash []byte) (*AddressContract, error) {
	// Check for a valid pubkey hash length.
	if len(hash) != ripemd160.Size {
		return nil, core.ErrInvalidPKHash
	}
	addr := new(AddressContract)
	copy(addr.hash[:], hash)
	return addr, nil
}

// SetString sets the Address's internal byte array using byte array decoded from input
// base58 format string, returns error if input string is invalid
func (a *AddressContract) SetString(in string) error {
	if len(in) != EncodeAddressLength || in[0] != BoxPrefix {
		return core.ErrInvalidAddressString
	}
	rawBytes, err := crypto.Base58CheckDecode(in)
	if err != nil {
		return err
	}
	if len(rawBytes) != 22 {
		return core.ErrInvalidAddressString
	}
	var prefix [2]byte
	copy(prefix[:], rawBytes[:2])
	if prefix != AddressTypeContractPrefix {
		return core.ErrInvalidAddressString
	}
	copy(a.hash[:], rawBytes[2:])
	return nil
}

// Hash160 returns the underlying array of the contract hash.
func (a *AddressContract) Hash160() *AddressHash {
	if a == nil {
		return nil
	}
	return &a.hash
}

func encodeAddress(hash []byte, prefix [2]byte) string {
	b := make([]byte, 0, len(hash)+2)
	b = append(b, prefix[:]...)
	b = append(b, hash[:]...)
	return crypto.Base58CheckEncode(b)
}

// ValidateAddr validetes addr
func ValidateAddr(addrs ...string) error {
	for _, addr := range addrs {
		if _, err := ParseAddress(addr); err != nil {
			return fmt.Errorf("addr %s is invalid, error: %s", addr, err)
		}
	}
	return nil
}

// ParseAddress parses address
func ParseAddress(in string) (Address, error) {
	if len(in) != EncodeAddressLength || in[0] != BoxPrefix {
		return nil, core.ErrInvalidAddressString
	}
	var (
		a   Address
		err error
	)
	switch in[:AddressPrefixLength] {
	default:
		return nil, core.ErrInvalidAddressString
	case AddrTypeP2PKHPrefix:
		a, err = NewAddress(in)
	case AddrTypeSplitAddrPrefix:
		a, err = NewSplitAddress(in)
	case AddrTypeContractPrefix:
		a, err = NewContractAddress(in)
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// MakeContractAddress make a contract address from sender and tx hash
func MakeContractAddress(
	sender *AddressPubKeyHash, nonce uint64,
) (*AddressContract, error) {
	// check
	if sender == nil || sender.hash == ZeroAddressHash {
		return nil, errors.New("invalid parameters for contract address")
	}
	ah := CreateAddress(sender.hash, nonce)
	return NewContractAddressFromHash(ah[:])
}

// CreateAddress creates an address given the bytes and the nonce
//
//  e.g.
//	b: [0,0,0,0,0,0,0,0,0,0,120,117,106,105,110,103,115,104,105]
//	nonce: 0
//
//	data: [214,148,0,0,0,0,0,0,0,0,0,0,120,117,106,105,110,103,115,104,105,128]
//		  0xd6(0xc0 + 22 data len) 0x94(0x80 + 20 b len) + b + 0x80(means 0)
//
func CreateAddress(b AddressHash, nonce uint64) *AddressHash {
	data, _ := rlp.EncodeToBytes([]interface{}{b, nonce})         // rlp encode
	addrHash := BytesToAddressHash(vmcrypto.Keccak256(data)[12:]) // hash by Keccak256
	return &addrHash
}

// CreateAddress2 creates an address given the address bytes, initial
// contract code hash and a salt.
func CreateAddress2(b AddressHash, salt [32]byte, inithash []byte) *AddressHash {
	addrHash := BytesToAddressHash(vmcrypto.Keccak256([]byte{0xff}, b[:], salt[:], inithash)[12:])
	return &addrHash
}

//
// InAddresses return if there is an element in the address array
func InAddresses(addr AddressHash, addrs []AddressHash) bool {
	for _, a := range addrs {
		if addr == a {
			return true
		}
	}
	return false
}
