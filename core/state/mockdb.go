package state

import (
	"math"
	"math/big"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"golang.org/x/crypto/ripemd160"
)

func NewMockdb() *Mockdb {
	return &Mockdb{
		contracts: make(map[types.AddressHash][]uint8, 100),
		nonces:    make(map[types.AddressHash]uint64, 100),
		states:    make(map[crypto.HashType]crypto.HashType, 100),
		accounts:  make(map[types.AddressHash]uint64, 100),
	}
}

type Mockdb struct {
	contracts map[types.AddressHash][]uint8
	nonces    map[types.AddressHash]uint64
	states    map[crypto.HashType]crypto.HashType
	accounts  map[types.AddressHash]uint64
}

func (self *Mockdb) CreateAccount(address types.AddressHash) {
	self.accounts[address] = 0
}

func (self *Mockdb) SubBalance(address types.AddressHash, amount *big.Int) {
	//FIXME: too simple logic
	self.accounts[address] -= amount.Uint64()
}

func (self *Mockdb) AddBalance(address types.AddressHash, amount *big.Int) {
	//FIXME: too simple logic
	self.accounts[address] += amount.Uint64()
}

func (self *Mockdb) GetBalance(address types.AddressHash) *big.Int {
	return big.NewInt(math.MaxUint32)
}

func (self *Mockdb) GetNonce(address types.AddressHash) uint64 {
	return self.nonces[address]
}

func (self *Mockdb) SetNonce(address types.AddressHash, nonce uint64) {
	self.nonces[address] = nonce
}

func (self *Mockdb) GetCodeHash(address types.AddressHash) crypto.HashType {
	code := self.contracts[address]
	if code == nil {
		return crypto.HashType{}
	}
	return crypto.BytesToHash(code)
}

func (self *Mockdb) GetCode(address types.AddressHash) []byte {
	code := self.contracts[address]
	if code != nil {
		return code
	}
	return []byte{}
}

func (self *Mockdb) SetCode(address types.AddressHash, code []byte) {
	self.contracts[address] = code
}

func (self *Mockdb) GetCodeSize(address types.AddressHash) int {
	code := self.contracts[address]
	if code == nil {
		return 0
	}
	return len(code)
}

func (self *Mockdb) AddRefund(refund uint64) {
}

func (self *Mockdb) SubRefund(refund uint64) {
}

func (self *Mockdb) GetRefund() uint64 {
	return 0
}

func (self *Mockdb) GetCommittedState(address types.AddressHash, key crypto.HashType) crypto.HashType {
	return [crypto.HashSize]byte{}
}

func (self *Mockdb) GetState(address types.AddressHash, key crypto.HashType) crypto.HashType {
	newKey := []byte{}
	newKey = append(newKey, address.Bytes()...)
	newKey = append(newKey, key.Bytes()[:crypto.HashSize-ripemd160.Size]...)
	return self.states[crypto.BytesToHash(newKey)]
}

func (self *Mockdb) SetState(address types.AddressHash, key, value crypto.HashType) {
	newKey := []byte{}
	newKey = append(newKey, address.Bytes()...)
	newKey = append(newKey, key.Bytes()[:crypto.HashSize-ripemd160.Size]...)
	self.states[crypto.BytesToHash(newKey)] = value
}

func (self *Mockdb) Suicide(address types.AddressHash) bool {
	return false
}

func (self *Mockdb) HasSuicided(address types.AddressHash) bool {
	return false
}

func (self *Mockdb) Exist(address types.AddressHash) bool {
	if self.contracts[address] != nil {
		return true
	}
	return false
}

func (self *Mockdb) Empty(address types.AddressHash) bool {
	return false
}

func (self *Mockdb) RevertToSnapshot(i int) {
}

func (self *Mockdb) Snapshot() int {
	return 0
}

// func (self *Mockdb) AddLog(log *types.Log) {
// }

func (self *Mockdb) AddPreimage(hash crypto.HashType, data []byte) {
}

func (self *Mockdb) ForEachStorage(types.AddressHash, func(crypto.HashType, crypto.HashType) bool) {
}
