// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package worldstate

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strconv"

	"github.com/BOXFoundation/boxd/core/trie"
	"github.com/BOXFoundation/boxd/core/types"
	corecrypto "github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/vm/common"
	vmtypes "github.com/BOXFoundation/boxd/vm/common/types"
	"github.com/BOXFoundation/boxd/vm/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

type revision struct {
	id           int
	journalIndex int
}

var logger = log.NewLogger("statedb")

var (
	// emptyState is the known hash of an empty state trie entry.
	emptyState = crypto.Keccak256Hash(nil)

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256Hash(nil)
)

// StateDB within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type StateDB struct {
	db       storage.Table
	trie     *trie.Trie
	utxoTrie *trie.Trie

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects      map[types.AddressHash]*stateObject
	stateObjectsDirty map[types.AddressHash]struct{}

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// The refund counter, also used by state transitioning.
	refund uint64

	thash, bhash corecrypto.HashType
	txIndex      int
	logs         map[corecrypto.HashType][]*vmtypes.Log
	logSize      uint

	preimages map[corecrypto.HashType][]byte

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        *journal
	validRevisions []revision
	nextRevisionID int
}

func (s *StateDB) String() string {
	var str string
	for k, v := range s.stateObjects {
		str += k.String() + "|" + strconv.Itoa(int(v.Balance().Int64())) + "|" + strconv.Itoa(int(v.Nonce())) + "|" + v.data.Root.String() + "|" + hex.EncodeToString(v.data.CodeHash) + "\n"
	}
	return str
}

// New a new state from a given trie.
func New(rootHash, utxoRootHash *corecrypto.HashType, db storage.Table) (*StateDB, error) {

	tr, err := trie.New(rootHash, db)
	if err != nil {
		logger.Warn(err)
		return nil, err
	}
	utr, err := trie.New(utxoRootHash, db)
	if err != nil {
		logger.Warn(err)
		return nil, err
	}
	return &StateDB{
		db:                db,
		trie:              tr,
		utxoTrie:          utr,
		stateObjects:      make(map[types.AddressHash]*stateObject),
		stateObjectsDirty: make(map[types.AddressHash]struct{}),
		logs:              make(map[corecrypto.HashType][]*vmtypes.Log),
		preimages:         make(map[corecrypto.HashType][]byte),
		journal:           newJournal(),
	}, nil
}

// setError remembers the first non-nil error it is called with.
func (s *StateDB) setError(err error) {
	if s.dbErr == nil {
		s.dbErr = err
	}
}

func (s *StateDB) Error() error {
	return s.dbErr
}

// Reset clears out all ephemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (s *StateDB) Reset() error {
	// trie
	roothash := s.trie.Hash()
	tr, err := trie.New(&roothash, s.db)
	if err != nil {
		return err
	}
	s.trie = tr
	// utxo tire
	utxoRootHash := s.utxoTrie.Hash()
	utr, err := trie.New(&utxoRootHash, s.db)
	if err != nil {
		return err
	}
	s.utxoTrie = utr
	//
	s.stateObjects = make(map[types.AddressHash]*stateObject)
	s.stateObjectsDirty = make(map[types.AddressHash]struct{})
	s.thash = corecrypto.HashType{}
	s.bhash = corecrypto.HashType{}
	s.txIndex = 0
	s.logs = make(map[corecrypto.HashType][]*vmtypes.Log)
	s.logSize = 0
	s.preimages = make(map[corecrypto.HashType][]byte)
	s.clearJournalAndRefund()
	return nil
}

// AddLog add log
func (s *StateDB) AddLog(log *vmtypes.Log) {
	s.journal.append(addLogChange{txhash: s.thash})

	log.TxHash = s.thash
	log.BlockHash = s.bhash
	log.TxIndex = uint(s.txIndex)
	log.Index = s.logSize
	s.logs[s.thash] = append(s.logs[s.thash], log)
	s.logSize++
}

// GetLogs get logs
func (s *StateDB) GetLogs(hash corecrypto.HashType) []*vmtypes.Log {
	return s.logs[hash]
}

// Logs logs
func (s *StateDB) Logs() []*vmtypes.Log {
	var logs []*vmtypes.Log
	for _, lgs := range s.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

// AddPreimage records a SHA3 preimage seen by the VM.
func (s *StateDB) AddPreimage(hash corecrypto.HashType, preimage []byte) {
	if _, ok := s.preimages[hash]; !ok {
		s.journal.append(addPreimageChange{hash: hash})
		pi := make([]byte, len(preimage))
		copy(pi, preimage)
		s.preimages[hash] = pi
	}
}

// Preimages returns a list of SHA3 preimages that have been submitted.
func (s *StateDB) Preimages() map[corecrypto.HashType][]byte {
	return s.preimages
}

// AddRefund add refund
func (s *StateDB) AddRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	s.refund += gas
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (s *StateDB) SubRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	if gas > s.refund {
		panic("Refund counter below zero")
	}
	s.refund -= gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (s *StateDB) Exist(addr types.AddressHash) bool {
	return s.getStateObject(addr) != nil
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (s *StateDB) Empty(addr types.AddressHash) bool {
	so := s.getStateObject(addr)
	return so == nil || so.empty()
}

// GetBalance Retrieve the balance from the given address or 0 if object not found
func (s *StateDB) GetBalance(addr types.AddressHash) *big.Int {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance()
	}
	return common.Big0
}

// GetNonce get nonce
func (s *StateDB) GetNonce(addr types.AddressHash) uint64 {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce()
	}

	return 0
}

// GetCode get code.
func (s *StateDB) GetCode(addr types.AddressHash) []byte {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Code()
	}
	return nil
}

// GetCodeSize get code size.
func (s *StateDB) GetCodeSize(addr types.AddressHash) int {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		return 0
	}
	if code := stateObject.Code(); code != nil {
		return len(code)
	}
	return 0
}

// GetCodeHash get code hash.
func (s *StateDB) GetCodeHash(addr types.AddressHash) corecrypto.HashType {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		return corecrypto.HashType{}
	}
	return corecrypto.BytesToHash(stateObject.CodeHash())
}

// GetState get state.
func (s *StateDB) GetState(a types.AddressHash, b corecrypto.HashType) corecrypto.HashType {
	stateObject := s.getStateObject(a)
	if stateObject != nil {
		return stateObject.GetState(b)
	}
	return corecrypto.HashType{}
}

// GetCommittedState retrieves a value from the given account's committed storage trie.
func (s *StateDB) GetCommittedState(addr types.AddressHash, hash corecrypto.HashType) corecrypto.HashType {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetCommittedState(hash)
	}
	return corecrypto.HashType{}
}

// Database retrieves the low level database supporting the lower level trie ops.
func (s *StateDB) Database() storage.Table {
	return s.db
}

// StorageTrie returns the storage trie of an account.
// The return value is a copy and is nil for non-existent accounts.
func (s *StateDB) StorageTrie(a types.AddressHash) *trie.Trie {
	stateObject := s.getStateObject(a)
	if stateObject == nil {
		return nil
	}
	cpy := stateObject.deepCopy(s)
	return cpy.updateTrie()
}

// HasSuicided judge hasSuicided.
func (s *StateDB) HasSuicided(addr types.AddressHash) bool {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.suicided
	}
	return false
}

/*
 * SETTERS
 */

// AddBalance adds amount to the account associated with addr
func (s *StateDB) AddBalance(addr types.AddressHash, amount *big.Int) {
	stateObject := s.getOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount)
	}
}

// SubBalance subtracts amount from the account associated with addr
func (s *StateDB) SubBalance(addr types.AddressHash, amount *big.Int) {
	stateObject := s.getOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

// SetBalance set balance.
func (s *StateDB) SetBalance(addr types.AddressHash, amount *big.Int) {
	stateObject := s.getOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

// SetNonce set nonce.
func (s *StateDB) SetNonce(addr types.AddressHash, nonce uint64) {
	stateObject := s.getOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

// SetCode set code.
func (s *StateDB) SetCode(addr types.AddressHash, code []byte) {
	stateObject := s.getOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

// SetState set state.
func (s *StateDB) SetState(addr types.AddressHash, key corecrypto.HashType, value corecrypto.HashType) {
	stateObject := s.getOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(key, value)
	}
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (s *StateDB) Suicide(addr types.AddressHash) bool {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	s.journal.append(suicideChange{
		account:     &addr,
		prev:        stateObject.suicided,
		prevbalance: new(big.Int).Set(stateObject.Balance()),
	})
	stateObject.markSuicided()
	stateObject.data.Balance = new(big.Int)

	return true
}

//
// Setting, updating & deleting state object methods
//

// updateStateObject writes the given object to the trie.
func (s *StateDB) updateStateObject(stateObject *stateObject) {
	addr := stateObject.Address()
	data, err := rlp.EncodeToBytes(stateObject)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	s.setError(s.trie.Update(addr[:], data))
}

// deleteStateObject removes the given object from the state trie.
func (s *StateDB) deleteStateObject(stateObject *stateObject) {
	stateObject.deleted = true
	addr := stateObject.Address()
	s.setError(s.trie.Delete(addr[:]))
}

// Retrieve a state object given my the address. Returns nil if not found.
func (s *StateDB) getStateObject(addr types.AddressHash) (stateObject *stateObject) {
	// Prefer 'live' objects.
	if obj := s.stateObjects[addr]; obj != nil {
		if obj.deleted {
			return nil
		}
		return obj
	}

	// Load the object from the database.
	enc, err := s.trie.Get(addr[:])
	if len(enc) == 0 {
		s.setError(err)
		return nil
	}
	var data Account
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		return nil
	}
	// Insert into the live set.
	obj := newObject(s, addr, data)
	s.setStateObject(obj)
	return obj
}

func (s *StateDB) setStateObject(object *stateObject) {
	s.stateObjects[object.Address()] = object
}

// getOrNewStateObject Retrieve a state object or create a new state object if nil
func (s *StateDB) getOrNewStateObject(addr types.AddressHash) *stateObject {
	stateObject := s.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		stateObject, _ = s.createObject(addr)
	}
	return stateObject
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (s *StateDB) createObject(addr types.AddressHash) (newobj, prev *stateObject) {
	prev = s.getStateObject(addr)
	newobj = newObject(s, addr, Account{})
	newobj.setNonce(0) // sets the object to dirty
	if prev == nil {
		s.journal.append(createObjectChange{account: &addr})
	} else {
		s.journal.append(resetObjectChange{prev: prev})
	}
	s.setStateObject(newobj)
	return newobj, prev
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the EVM CREATE operation. The situation might arise that
// a contract does the following:
//
//   1. sends funds to sha(account ++ (nonce + 1))
//   2. tx_create(sha(account ++ nonce)) (note that this gets the address of 1)
//
// Carrying over the balance ensures that Ether doesn't disappear.
func (s *StateDB) CreateAccount(addr types.AddressHash) {
	new, prev := s.createObject(addr)
	if prev != nil {
		new.setBalance(prev.data.Balance)
	}
}

// ForEachStorage traverse the storage.
func (s *StateDB) ForEachStorage(addr types.AddressHash, cb func(key, value corecrypto.HashType) bool) {
	// so := db.getStateObject(addr)
	// if so == nil {
	// 	return
	// }
	// it := trie.NewIterator(so.getTrie(db.db).NodeIterator(nil))
	// for it.Next() {
	// 	key := common.BytesToHash(db.trie.GetKey(it.Key))
	// 	if value, dirty := so.dirtyStorage[key]; dirty {
	// 		cb(key, value)
	// 		continue
	// 	}
	// 	cb(key, common.BytesToHash(it.Value))
	// }
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (s *StateDB) Copy() *StateDB {
	// Copy all the basic fields, initialize the memory ones
	state := &StateDB{
		db:   s.db,
		trie: nil,
		// trie:              s.db.CopyTrie(s.trie),
		stateObjects:      make(map[types.AddressHash]*stateObject, len(s.journal.dirties)),
		stateObjectsDirty: make(map[types.AddressHash]struct{}, len(s.journal.dirties)),
		refund:            s.refund,
		logs:              make(map[corecrypto.HashType][]*vmtypes.Log, len(s.logs)),
		logSize:           s.logSize,
		preimages:         make(map[corecrypto.HashType][]byte),
		journal:           newJournal(),
	}
	// Copy the dirty states, logs, and preimages
	for addr := range s.journal.dirties {
		// As documented [here](https://github.com/ethereum/go-ethereum/pull/16485#issuecomment-380438527),
		// and in the Finalise-method, there is a case where an object is in the journal but not
		// in the stateObjects: OOG after touch on ripeMD prior to Byzantium. Thus, we need to check for
		// nil
		if object, exist := s.stateObjects[addr]; exist {
			state.stateObjects[addr] = object.deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
	}
	// Above, we don't copy the actual journal. This means that if the copy is copied, the
	// loop above will be a no-op, since the copy's journal is empty.
	// Thus, here we iterate over stateObjects, to enable copies of copies
	for addr := range s.stateObjectsDirty {
		if _, exist := state.stateObjects[addr]; !exist {
			state.stateObjects[addr] = s.stateObjects[addr].deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
	}
	for hash, logs := range s.logs {
		cpy := make([]*vmtypes.Log, len(logs))
		for i, l := range logs {
			cpy[i] = new(vmtypes.Log)
			*cpy[i] = *l
		}
		state.logs[hash] = cpy
	}
	for hash, preimage := range s.preimages {
		state.preimages[hash] = preimage
	}
	return state
}

// Snapshot returns an identifier for the current revision of the state.
func (s *StateDB) Snapshot() int {
	id := s.nextRevisionID
	s.nextRevisionID++
	s.validRevisions = append(s.validRevisions, revision{id, s.journal.length()})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (s *StateDB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(s.validRevisions), func(i int) bool {
		return s.validRevisions[i].id >= revid
	})
	if idx == len(s.validRevisions) || s.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := s.validRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	s.journal.revert(s, snapshot)
	s.validRevisions = s.validRevisions[:idx]
}

// GetRefund returns the current value of the refund counter.
func (s *StateDB) GetRefund() uint64 {
	return s.refund
}

// Finalise finalises the state by removing the s destructed objects
// and clears the journal as well as the refunds.
func (s *StateDB) finalise(deleteEmptyObjects bool) {
	for addr := range s.journal.dirties {
		stateObject, exist := s.stateObjects[addr]
		if !exist {
			// ripeMD is 'touched' at block 1714175, in tx 0x1237f737031e40bcde4a8b7e717b2d15e3ecadfe49bb1bbc71ee9deb09c6fcf2
			// That tx goes out of gas, and although the notion of 'touched' does not exist there, the
			// touch-event will still be recorded in the journal. Since ripeMD is a special snowflake,
			// it will persist in the journal even though the journal is reverted. In this special circumstance,
			// it may exist in `s.journal.dirties` but not in `s.stateObjects`.
			// Thus, we can safely ignore it here
			continue
		}

		if stateObject.suicided || (deleteEmptyObjects && stateObject.empty()) {
			s.deleteStateObject(stateObject)
		} else {
			stateObject.updateRoot()
			s.updateStateObject(stateObject)
		}
		s.stateObjectsDirty[addr] = struct{}{}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	s.clearJournalAndRefund()
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (s *StateDB) IntermediateRoot(deleteEmptyObjects bool) corecrypto.HashType {
	s.finalise(deleteEmptyObjects)
	return s.trie.Hash()
}

// Prepare sets the current transaction hash and index and block hash which is
// used when the EVM emits new state logs.
func (s *StateDB) Prepare(thash, bhash corecrypto.HashType, ti int) {
	s.thash = thash
	s.bhash = bhash
	s.txIndex = ti
}

func (s *StateDB) clearJournalAndRefund() {
	s.journal = newJournal()
	s.validRevisions = s.validRevisions[:0]
	s.refund = 0
}

// Commit writes the state to the underlying in-memory trie database.
func (s *StateDB) Commit(deleteEmptyObjects bool) (*corecrypto.HashType, *corecrypto.HashType, error) {
	defer s.clearJournalAndRefund()
	s.finalise(deleteEmptyObjects)

	for addr := range s.journal.dirties {
		s.stateObjectsDirty[addr] = struct{}{}
	}
	// Commit objects to the trie.
	for addr, stateObject := range s.stateObjects {
		_, isDirty := s.stateObjectsDirty[addr]
		switch {
		case stateObject.suicided || (isDirty && deleteEmptyObjects && stateObject.empty()):
			// If the object has been removed, don't bother syncing it
			// and just mark it for deletion in the trie.
			s.deleteStateObject(stateObject)
		case isDirty:
			// Write any contract code associated with the state object
			if stateObject.code != nil && stateObject.dirtyCode {
				logger.Errorf("put enable: %v, hash: %s, code: %v", s.db.IsInBatch(), stateObject.CodeHash(), stateObject.code[:])
				s.db.Put(stateObject.CodeHash(), stateObject.code[:])
				stateObject.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie.
			if _, err := stateObject.CommitTrie(); err != nil {
				return nil, nil, err
			}
			// Update the object in the main account trie.
			s.updateStateObject(stateObject)
		}
		delete(s.stateObjectsDirty, addr)
	}
	// Write trie changes.
	root, err := s.trie.Commit()
	if err != nil {
		logger.Error(err)
		return nil, nil, err
	}
	// for utxo trie root hash
	utxoRoot, err := s.utxoTrie.Commit()
	if err != nil {
		logger.Error(err)
		return nil, nil, err
	}
	return root, utxoRoot, nil
}

// UpdateUtxo updates statedb utxo at given contract addr
func (s *StateDB) UpdateUtxo(addr types.AddressHash, utxoBytes []byte) error {
	return s.utxoTrie.Update(addr[:], utxoBytes)
}
