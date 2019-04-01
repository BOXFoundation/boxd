package state

import (
	"math"
	"math/big"

	"github.com/BOXFoundation/boxd/vm/common"
	"github.com/BOXFoundation/boxd/vm/common/types"
)

func NewMockdb() *Mockdb {
	return &Mockdb{
		contracts: make(map[common.Address][]uint8, 100),
		nonces:    make(map[common.Address]uint64, 100),
		states:    make(map[common.Hash]common.Hash, 100),
		accounts:  make(map[common.Address]uint64, 100),
	}
}

type Mockdb struct {
	contracts map[common.Address][]uint8
	nonces    map[common.Address]uint64
	states    map[common.Hash]common.Hash
	accounts  map[common.Address]uint64
}

func (self *Mockdb) CreateAccount(address common.Address) {
	self.accounts[address] = 0
}

func (self *Mockdb) SubBalance(address common.Address, amount *big.Int) {
	//FIXME: too simple logic
	self.accounts[address] -= amount.Uint64()
}

func (self *Mockdb) AddBalance(address common.Address, amount *big.Int) {
	//FIXME: too simple logic
	self.accounts[address] += amount.Uint64()
}

func (self *Mockdb) GetBalance(address common.Address) *big.Int {
	return big.NewInt(math.MaxUint32)
}

func (self *Mockdb) GetNonce(address common.Address) uint64 {
	return self.nonces[address]
}

func (self *Mockdb) SetNonce(address common.Address, nonce uint64) {
	self.nonces[address] = nonce
}

func (self *Mockdb) GetCodeHash(address common.Address) common.Hash {
	code := self.contracts[address]
	if code == nil {
		return common.Hash{}
	}
	return common.BytesToHash(code)
}

func (self *Mockdb) GetCode(address common.Address) []byte {
	code := self.contracts[address]
	if code != nil {
		return code
	}
	return []byte{}
}

func (self *Mockdb) SetCode(address common.Address, code []byte) {
	self.contracts[address] = code
}

func (self *Mockdb) GetCodeSize(address common.Address) int {
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

func (self *Mockdb) GetCommittedState(address common.Address, key common.Hash) common.Hash {
	return [common.HashLength]byte{}
}

func (self *Mockdb) GetState(address common.Address, key common.Hash) common.Hash {
	newKey := []byte{}
	newKey = append(newKey, address.Bytes()...)
	newKey = append(newKey, key.Bytes()...)
	return self.states[common.BytesToHash(newKey)]
}

func (self *Mockdb) SetState(address common.Address, key, value common.Hash) {
	newKey := []byte{}
	newKey = append(newKey, address.Bytes()...)
	newKey = append(newKey, key.Bytes()...)
	self.states[common.BytesToHash(newKey)] = value
}

func (self *Mockdb) Suicide(address common.Address) bool {
	return false
}

func (self *Mockdb) HasSuicided(address common.Address) bool {
	return false
}

func (self *Mockdb) Exist(address common.Address) bool {
	if self.contracts[address] != nil {
		return true
	}
	return false
}

func (self *Mockdb) Empty(address common.Address) bool {
	return false
}

func (self *Mockdb) RevertToSnapshot(i int) {
}

func (self *Mockdb) Snapshot() int {
	return 0
}

func (self *Mockdb) AddLog(log *types.Log) {
}

func (self *Mockdb) AddPreimage(hash common.Hash, data []byte) {
}

func (self *Mockdb) ForEachStorage(common.Address, func(common.Hash, common.Hash) bool) {
}
