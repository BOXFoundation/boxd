// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package state

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/BOXFoundation/boxd/core/trie"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/facebookgo/ensure"
)

type StateSuite struct {
	db    *storage.Database
	state *StateDB
}

var toAddr = types.BytesToAddressHash

// func (s *StateSuite) TestDump(t *testing.T) {
// 	// generate a few entries
// 	obj1 := s.state.getOrNewStateObject(toAddr([]byte{0x01}))
// 	obj1.AddBalance(big.NewInt(22))
// 	obj2 := s.state.getOrNewStateObject(toAddr([]byte{0x01, 0x02}))
// 	obj2.SetCode(vmcrypto.Keccak256Hash([]byte{3, 3, 3, 3, 3, 3, 3}), []byte{3, 3, 3, 3, 3, 3, 3})
// 	obj3 := s.state.getOrNewStateObject(toAddr([]byte{0x02}))
// 	obj3.SetBalance(big.NewInt(44))

// 	// write some of them to the trie
// 	s.state.updateStateObject(obj1)
// 	s.state.updateStateObject(obj2)
// 	s.state.Commit(false)

// 	// check that dump contains the state objects that are in trie
// 	got := string(s.state.Dump())
// 	want := `{
//     "root": "71edff0130dd2385947095001c73d9e28d862fc286fca2b922ca6f6f3cddfdd2",
//     "accounts": {
//         "0000000000000000000000000000000000000001": {
//             "balance": "22",
//             "nonce": 0,
//             "root": "56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
//             "codeHash": "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
//             "code": "",
//             "storage": {}
//         },
//         "0000000000000000000000000000000000000002": {
//             "balance": "44",
//             "nonce": 0,
//             "root": "56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
//             "codeHash": "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
//             "code": "",
//             "storage": {}
//         },
//         "0000000000000000000000000000000000000102": {
//             "balance": "0",
//             "nonce": 0,
//             "root": "56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
//             "codeHash": "87874902497a5bb968da31a2998d8f22e949d1ef6214bcdedd8bae24cca4b9e3",
//             "code": "03030303030303",
//             "storage": {}
//         }
//     }
// }`
// 	if got != want {
// 		fmt.Errorf("dump mismatch:\ngot: %s\nwant: %s\n", got, want)
// 	}
// }

func (s *StateSuite) SetUpTest(t *testing.T) {
	s.db = initDB()
	s.state, _ = New(nil, nil, s.db)
}

func (s *StateSuite) TestNull(t *testing.T) {
	address := types.HexToAddressHash("0x823140710bf13990e4500136726d8b55")
	s.state.CreateAccount(address)
	var value crypto.HashType

	s.state.SetState(address, crypto.HashType{}, value)
	s.state.Commit(false)

	if value := s.state.GetState(address, crypto.HashType{}); value != (crypto.HashType{}) {
		fmt.Errorf("expected empty current value, got %x", value)
	}
	if value := s.state.GetCommittedState(address, crypto.HashType{}); value != (crypto.HashType{}) {
		fmt.Errorf("expected empty committed value, got %x", value)
	}
}

func (s *StateSuite) TestSnapshot(t *testing.T) {
	stateobjaddr := toAddr([]byte("aa"))
	var storageaddr crypto.HashType
	data1 := crypto.BytesToHash([]byte{42})
	data2 := crypto.BytesToHash([]byte{43})

	// snapshot the genesis state
	genesis := s.state.Snapshot()

	// set initial state object value
	s.state.SetState(stateobjaddr, storageaddr, data1)
	snapshot := s.state.Snapshot()

	// set a new state object value, revert it and ensure correct content
	s.state.SetState(stateobjaddr, storageaddr, data2)
	s.state.RevertToSnapshot(snapshot)

	ensure.DeepEqual(t, s.state.GetState(stateobjaddr, storageaddr), data1)
	ensure.DeepEqual(t, s.state.GetCommittedState(stateobjaddr, storageaddr), crypto.HashType{})

	// revert up to the genesis state and ensure correct content
	s.state.RevertToSnapshot(genesis)
	ensure.DeepEqual(t, s.state.GetState(stateobjaddr, storageaddr), crypto.HashType{})
	ensure.DeepEqual(t, s.state.GetCommittedState(stateobjaddr, storageaddr), crypto.HashType{})
}

func (s *StateSuite) TestSnapshotEmpty(t *testing.T) {
	s.state.RevertToSnapshot(s.state.Snapshot())
}

// use testing instead of checker because checker does not support
// printing/logging in tests (-check.vv does not work)
func TestSnapshot2(t *testing.T) {
	state, _ := New(nil, nil, initDB())

	stateobjaddr0 := toAddr([]byte("so0"))
	stateobjaddr1 := toAddr([]byte("so1"))
	var storageaddr crypto.HashType

	data0 := crypto.BytesToHash([]byte{17})
	data1 := crypto.BytesToHash([]byte{18})

	state.SetState(stateobjaddr0, storageaddr, data0)
	state.SetState(stateobjaddr1, storageaddr, data1)

	// db, trie are already non-empty values
	so0 := state.getStateObject(stateobjaddr0)
	so0.SetBalance(big.NewInt(42))
	so0.SetNonce(43)
	so0.SetCode(vmcrypto.Keccak256Hash([]byte{'c', 'a', 'f', 'e'}), []byte{'c', 'a', 'f', 'e'})
	so0.suicided = false
	so0.deleted = false
	state.setStateObject(so0)

	state.Commit(false)
	state.Reset()

	// and one with deleted == true
	so1 := state.getStateObject(stateobjaddr1)
	so1.SetBalance(big.NewInt(52))
	so1.SetNonce(53)
	so1.SetCode(vmcrypto.Keccak256Hash([]byte{'c', 'a', 'f', 'e', '2'}), []byte{'c', 'a', 'f', 'e', '2'})
	so1.suicided = true
	so1.deleted = true
	state.setStateObject(so1)

	so1 = state.getStateObject(stateobjaddr1)
	if so1 != nil {
		t.Fatalf("deleted object not nil when getting")
	}
}
func TestSnapshot3(t *testing.T) {
	// state, _ := New(nil, nil, initDB())

	stateobjaddr0 := toAddr([]byte("so0"))
	stateobjaddr1 := toAddr([]byte("so1"))
	var storageaddr crypto.HashType

	data0 := crypto.BytesToHash([]byte{17})
	data1 := crypto.BytesToHash([]byte{18})

	db := initDB()

	tr, _ := trie.New(nil, db)

	state, _ := New(nil, nil, db)

	state.SetState(stateobjaddr0, storageaddr, data0)
	state.SetState(stateobjaddr1, storageaddr, data1)

	// db, trie are already non-empty values
	so0 := state.getStateObject(stateobjaddr0)
	so1 := state.getStateObject(stateobjaddr1)
	d0, _ := rlp.EncodeToBytes(so0)
	d1, _ := rlp.EncodeToBytes(so1)

	tr.Update(stateobjaddr0[:], d0)
	tr.Update(stateobjaddr1[:], d1)
	root, _ := tr.Commit()

	trNew, _ := trie.New(root, db)
	dd, err := trNew.Get(stateobjaddr1[:])

	fmt.Println(string(dd))
	fmt.Println(err)
}

func compareStateObjects(so0, so1 *stateObject, t *testing.T) {
	if so0.Address() != so1.Address() {
		t.Fatalf("Address mismatch: have %v, want %v", so0.address, so1.address)
	}
	if so0.Balance().Cmp(so1.Balance()) != 0 {
		t.Fatalf("Balance mismatch: have %v, want %v", so0.Balance(), so1.Balance())
	}
	if so0.Nonce() != so1.Nonce() {
		t.Fatalf("Nonce mismatch: have %v, want %v", so0.Nonce(), so1.Nonce())
	}
	if so0.data.Root != so1.data.Root {
		t.Errorf("Root mismatch: have %x, want %x", so0.data.Root[:], so1.data.Root[:])
	}
	if !bytes.Equal(so0.CodeHash(), so1.CodeHash()) {
		t.Fatalf("CodeHash mismatch: have %v, want %v", so0.CodeHash(), so1.CodeHash())
	}
	if !bytes.Equal(so0.code, so1.code) {
		t.Fatalf("Code mismatch: have %v, want %v", so0.code, so1.code)
	}

	if len(so1.dirtyStorage) != len(so0.dirtyStorage) {
		t.Errorf("Dirty storage size mismatch: have %d, want %d", len(so1.dirtyStorage), len(so0.dirtyStorage))
	}
	for k, v := range so1.dirtyStorage {
		if so0.dirtyStorage[k] != v {
			t.Errorf("Dirty storage key %x mismatch: have %v, want %v", k, so0.dirtyStorage[k], v)
		}
	}
	for k, v := range so0.dirtyStorage {
		if so1.dirtyStorage[k] != v {
			t.Errorf("Dirty storage key %x mismatch: have %v, want none.", k, v)
		}
	}
	if len(so1.originStorage) != len(so0.originStorage) {
		t.Errorf("Origin storage size mismatch: have %d, want %d", len(so1.originStorage), len(so0.originStorage))
	}
	for k, v := range so1.originStorage {
		if so0.originStorage[k] != v {
			t.Errorf("Origin storage key %x mismatch: have %v, want %v", k, so0.originStorage[k], v)
		}
	}
	for k, v := range so0.originStorage {
		if so1.originStorage[k] != v {
			t.Errorf("Origin storage key %x mismatch: have %v, want none.", k, v)
		}
	}
}
