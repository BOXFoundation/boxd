// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package worldstate

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/trie"
	"github.com/BOXFoundation/boxd/core/types"
	corecrypto "github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/vm/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

var emptyCodeHash = crypto.Keccak256(nil)

// Code ...
type Code []byte

func (s Code) String() string {
	return string(s) //strings.Join(Disassemble(s), " ")
}

// Storage ...
type Storage map[corecrypto.HashType]corecrypto.HashType

func (s Storage) String() (str string) {
	for key, value := range s {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}

	return
}

// Copy new a new storage by last one.
func (s Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range s {
		cpy[key] = value
	}

	return cpy
}

// stateObject represents an account which is being modified.
//
// The usage pattern is as follows:
// First you need to obtain a state object.
// Account values can be accessed and modified through the object.
// Finally, call CommitTrie to write the modified storage trie into a database.
type stateObject struct {
	address  types.AddressHash
	addrHash corecrypto.HashType // hash of address of the account
	data     Account
	db       *StateDB

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// Write caches.
	trie *trie.Trie // storage trie, which becomes non-nil on first access
	code Code       // contract bytecode, which gets set when code is loaded

	originStorage Storage // Storage entry cache to avoid duplicate reads
	dirtyStorage  Storage // Storage entries that need to be flushed to disk

	// Cache flags.
	// When an object is marked suicided it will be delete from the trie
	// during the "update" phase of the state transition.
	dirtyCode bool // true if the code was updated
	suicided  bool
	deleted   bool
}

// empty returns whether the account is considered empty.
func (s *stateObject) empty() bool {
	return s.data.Nonce == 0 && s.data.Balance.Sign() == 0 && bytes.Equal(s.data.CodeHash, emptyCodeHash)
}

// Account represent vm accounts.
// These objects are stored in the main account trie.
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     corecrypto.HashType // merkle root of the storage trie
	CodeHash []byte
}

// newObject creates a state object.
func newObject(db *StateDB, address types.AddressHash, data Account) *stateObject {
	if data.Balance == nil {
		data.Balance = new(big.Int)
	}
	if data.CodeHash == nil {
		data.CodeHash = emptyCodeHash
	}
	return &stateObject{
		db:            db,
		address:       address,
		addrHash:      crypto.Keccak256Hash(address[:]),
		data:          data,
		originStorage: make(Storage),
		dirtyStorage:  make(Storage),
	}
}

// EncodeRLP implements rlp.Encoder.
func (s *stateObject) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, s.data)
}

// setError remembers the first non-nil error it is called with.
func (s *stateObject) setError(err error) {
	if err != nil {
		logger.Errorf("stateObject SET ERROR: %v", err)
	}
	if s.dbErr == nil {
		s.dbErr = err
	}
}

func (s *stateObject) markSuicided() {
	s.suicided = true
}

func (s *stateObject) touch() {
	s.db.journal.append(touchChange{
		account: &s.address,
	})
	if s.address == ripemd {
		// Explicitly put it in the dirty-cache, which is otherwise generated from
		// flattened journals.
		s.db.journal.dirty(s.address)
	}
}

func (s *stateObject) getTrie() *trie.Trie {
	if s.trie == nil {
		var err error
		s.trie, err = trie.New(&s.data.Root, s.db.db)
		if err != nil {
			s.trie, _ = trie.New(&corecrypto.HashType{}, s.db.db)
			s.setError(fmt.Errorf("can't create storage trie: %v", err))
		}
	}
	return s.trie
}

// GetState returns a value in account storage.
func (s *stateObject) GetState(key corecrypto.HashType) corecrypto.HashType {
	// If we have a dirty value for this state entry, return it
	value, dirty := s.dirtyStorage[key]
	if dirty {
		return value
	}
	// Otherwise return the entry's original value
	return s.GetCommittedState(key)
}

// GetCommittedState retrieves a value from the committed account storage trie.
func (s *stateObject) GetCommittedState(key corecrypto.HashType) corecrypto.HashType {
	// If we have the original value cached, return that
	value, cached := s.originStorage[key]
	if cached {
		return value
	}
	// Otherwise load the value from the database
	enc, err := s.getTrie().Get(key[:])
	if err != nil && err != core.ErrNodeNotFound {
		s.setError(err)
		return corecrypto.HashType{}
	}
	if len(enc) > 0 {
		_, content, _, err := rlp.Split(enc)
		if err != nil {
			s.setError(err)
		}
		value.SetBytes(content)
	}
	s.originStorage[key] = value
	return value
}

// SetState updates a value in account storage.
func (s *stateObject) SetState(key, value corecrypto.HashType) {
	// If the new value is the same as old, don't set
	prev := s.GetState(key)
	if prev == value {
		return
	}
	// New value is different, update and journal the change
	s.db.journal.append(storageChange{
		account:  &s.address,
		key:      key,
		prevalue: prev,
	})
	s.setState(key, value)
}

func (s *stateObject) setState(key, value corecrypto.HashType) {
	s.dirtyStorage[key] = value
}

// updateTrie writes cached storage modifications into the object's storage trie.
func (s *stateObject) updateTrie() *trie.Trie {
	tr := s.getTrie()
	for key, value := range s.dirtyStorage {
		delete(s.dirtyStorage, key)

		// Skip noop changes, persist actual changes
		if value == s.originStorage[key] {
			continue
		}
		s.originStorage[key] = value

		if value == corecrypto.ZeroHash {
			s.setError(tr.Delete(key[:]))
			continue
		}
		// Encoding []byte cannot fail, ok to ignore the error.
		v, _ := rlp.EncodeToBytes(bytes.TrimLeft(value[:], "\x00"))
		s.setError(tr.Update(key[:], v))
	}
	return tr
}

// UpdateRoot sets the trie root to the current root hash of
func (s *stateObject) updateRoot() {
	s.updateTrie()
	s.data.Root = s.trie.Hash()
	//logger.Debugf("DEBUG: state object update addr %x root: %s", s.address[:], s.data.Root)
}

// CommitTrie the storage trie of the object to dwb.
// This updates the trie root.
func (s *stateObject) CommitTrie() (*corecrypto.HashType, error) {
	s.updateTrie()
	if s.dbErr != nil {
		return nil, s.dbErr
	}
	//logger.Debugf("DEBUG: state object CommitTrie addr %x root: %s", s.address[:], s.trie.RootHash())
	return s.trie.RootHash(), nil
}

// AddBalance removes amount from c's balance.
// It is used to add funds to the destination account of a transfer.
func (s *stateObject) AddBalance(amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Sign() == 0 {
		if s.empty() {
			s.touch()
		}

		return
	}
	s.SetBalance(new(big.Int).Add(s.Balance(), amount))
}

// SubBalance removes amount from c's balance.
// It is used to remove funds from the origin account of a transfer.
func (s *stateObject) SubBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	s.SetBalance(new(big.Int).Sub(s.Balance(), amount))
}

func (s *stateObject) SetBalance(amount *big.Int) {
	s.db.journal.append(balanceChange{
		account: &s.address,
		prev:    new(big.Int).Set(s.data.Balance),
	})
	s.setBalance(amount)
}

func (s *stateObject) setBalance(amount *big.Int) {
	s.data.Balance = amount
}

// Return the gas back to the origin. Used by the Virtual machine or Closures
func (s *stateObject) ReturnGas(gas *big.Int) {}

func (s *stateObject) deepCopy(db *StateDB) *stateObject {
	stateObject := newObject(db, s.address, s.data)
	if s.trie != nil {
		stateObject.trie = db.trie.Copy()
	}
	stateObject.code = s.code
	stateObject.dirtyStorage = s.dirtyStorage.Copy()
	stateObject.originStorage = s.originStorage.Copy()
	stateObject.suicided = s.suicided
	stateObject.dirtyCode = s.dirtyCode
	stateObject.deleted = s.deleted
	return stateObject
}

//
// Attribute accessors
//

// Returns the address of the contract/account
func (s *stateObject) Address() types.AddressHash {
	return s.address
}

// Code returns the contract code associated with this object, if any.
func (s *stateObject) Code() []byte {
	if s.code != nil {
		return s.code
	}
	if bytes.Equal(s.CodeHash(), emptyCodeHash) {
		return nil
	}
	code, err := s.db.db.Get(s.CodeHash())
	if len(code) == 0 || err != nil {
		s.setError(fmt.Errorf("can't load code hash %x: %v", s.CodeHash(), err))
	}
	s.code = code
	return code
}

func (s *stateObject) SetCode(codeHash corecrypto.HashType, code []byte) {
	prevcode := s.Code()
	s.db.journal.append(codeChange{
		account:  &s.address,
		prevhash: s.CodeHash(),
		prevcode: prevcode,
	})
	s.setCode(codeHash, code)
}

func (s *stateObject) setCode(codeHash corecrypto.HashType, code []byte) {
	s.code = code
	s.data.CodeHash = codeHash[:]
	s.dirtyCode = true
}

func (s *stateObject) SetNonce(nonce uint64) {
	s.db.journal.append(nonceChange{
		account: &s.address,
		prev:    s.data.Nonce,
	})
	s.setNonce(nonce)
}

func (s *stateObject) setNonce(nonce uint64) {
	s.data.Nonce = nonce
}

func (s *stateObject) CodeHash() []byte {
	return s.data.CodeHash
}

func (s *stateObject) Balance() *big.Int {
	return s.data.Balance
}

func (s *stateObject) Nonce() uint64 {
	return s.data.Nonce
}

// Never called, but must be present to allow stateObject to be used
// as a vm.Account interface that also satisfies the vm.ContractRef
// interface. Interfaces are awesome.
func (s *stateObject) Value() *big.Int {
	panic("Value on stateObject should never be called")
}
