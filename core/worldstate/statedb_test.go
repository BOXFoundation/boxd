// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package worldstate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage"
	_ "github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/BOXFoundation/boxd/util"
	"github.com/jbenet/goprocess"
)

func initDB() *storage.Database {
	dbCfg := &storage.Config{
		Name: "memdb",
		Path: "~/tmp",
	}
	proc := goprocess.WithSignals(os.Interrupt)
	database, _ := storage.NewDatabase(proc, dbCfg)
	return database
}

// Tests that updating a state trie does not leak any database writes prior to
// actually committing the state.
func TestUpdateLeaks(t *testing.T) {
	// Create an empty state database
	db := initDB()
	state, _ := New(nil, nil, db)

	last := 0
	// Update it with some accounts
	for i := byte(0); i < 255; i++ {
		addr := types.BytesToAddressHash([]byte{i})
		state.AddBalance(addr, big.NewInt(int64(11*i)))
		state.SetNonce(addr, uint64(42*i))
		if i%2 == 0 {
			state.SetState(addr, crypto.BytesToHash([]byte{i, i, i}), crypto.BytesToHash([]byte{i, i, i, i}))
		}
		if i%3 == 0 {
			state.SetCode(addr, []byte{i, i, i, i, i})
		}
		state.IntermediateRoot(false)
		if len(db.Keys()) != last {
			last = len(db.Keys())
			fmt.Println(i, " ", len(db.Keys()))
		}
	}
	// Ensure that no data was leaked into the database
	// for i, key := range db.Keys() {
	// 	value, _ := db.Get(key)
	// 	fmt.Printf("%x. State leaked into database: %x -> %x\n", i, key, value)
	// }
}

// Tests that no intermediate state of an object is stored into the database,
// only the one right before the commit.
func TestIntermediateLeaks(t *testing.T) {
	// Create two state databases, one transitioning to the final state, the other final from the beginning
	transDb := initDB()
	finalDb := initDB()
	transState, _ := New(nil, nil, transDb)
	finalState, _ := New(nil, nil, finalDb)

	modify := func(state *StateDB, addr types.AddressHash, i, tweak byte) {
		state.SetBalance(addr, big.NewInt(int64(11*i)+int64(tweak)))
		state.SetNonce(addr, uint64(42*i+tweak))
		if i%2 == 0 {
			state.SetState(addr, crypto.HashType{i, i, i, 0}, crypto.HashType{})
			state.SetState(addr, crypto.HashType{i, i, i, tweak}, crypto.HashType{i, i, i, i, tweak})
		}
		if i%3 == 0 {
			state.SetCode(addr, []byte{i, i, i, i, i, tweak})
		}
	}

	// Modify the transient state.
	for i := byte(0); i < 255; i++ {
		modify(transState, types.AddressHash{byte(i)}, i, 0)
	}
	// Write modifications to trie.
	transState.IntermediateRoot(false)

	// Overwrite all the data with new values in the transient database.
	for i := byte(0); i < 255; i++ {
		modify(transState, types.AddressHash{byte(i)}, i, 99)
		modify(finalState, types.AddressHash{byte(i)}, i, 99)
	}

	// Commit and cross check the databases.
	if _, _, err := transState.Commit(false); err != nil {
		t.Fatalf("failed to commit transition state: %v", err)
	}
	if _, _, err := finalState.Commit(false); err != nil {
		t.Fatalf("failed to commit final state: %v", err)
	}
	for _, key := range finalDb.Keys() {
		if _, err := transDb.Get(key); err != nil {
			val, _ := finalDb.Get(key)
			fmt.Printf("entry missing from the transition database: %x -> %x", key, val)
		}
	}
	for _, key := range transDb.Keys() {
		if _, err := finalDb.Get(key); err != nil {
			val, _ := transDb.Get(key)
			fmt.Printf("extra entry in the transition database: %x -> %x", key, val)
		}
	}
}

func TestSnapshotRandom(t *testing.T) {
	config := &quick.Config{MaxCount: 1000}
	err := quick.Check((*snapshotTest).run, config)
	if cerr, ok := err.(*quick.CheckError); ok {
		test := cerr.In[0].(*snapshotTest)
		fmt.Printf("%v:\n%s", test.err, test)
	} else if err != nil {
		fmt.Println(err)
	}
}

// A snapshotTest checks that reverting StateDB snapshots properly undoes all changes
// captured by the snapshot. Instances of this test with pseudorandom content are created
// by Generate.
//
// The test works as follows:
//
// A new state is created and all actions are applied to it. Several snapshots are taken
// in between actions. The test then reverts each snapshot. For each snapshot the actions
// leading up to it are replayed on a fresh, empty state. The behaviour of all public
// accessor methods on the reverted state must match the return value of the equivalent
// methods on the replayed state.
type snapshotTest struct {
	addrs     []types.AddressHash // all account addresses
	actions   []testAction        // modifications to the state
	snapshots []int               // actions indexes at which snapshot is taken
	err       error               // failure details are reported through this field
}

type testAction struct {
	name   string
	fn     func(testAction, *StateDB)
	args   []int64
	noAddr bool
}

// newTestAction creates a random action that changes state.
func newTestAction(addr types.AddressHash, r *rand.Rand) testAction {
	actions := []testAction{
		{
			name: "SetBalance",
			fn: func(a testAction, s *StateDB) {
				s.SetBalance(addr, big.NewInt(a.args[0]))
			},
			args: make([]int64, 1),
		},
		{
			name: "AddBalance",
			fn: func(a testAction, s *StateDB) {
				s.AddBalance(addr, big.NewInt(a.args[0]))
			},
			args: make([]int64, 1),
		},
		{
			name: "SetNonce",
			fn: func(a testAction, s *StateDB) {
				s.SetNonce(addr, uint64(a.args[0]))
			},
			args: make([]int64, 1),
		},
		{
			name: "SetState",
			fn: func(a testAction, s *StateDB) {
				var key, val crypto.HashType
				binary.BigEndian.PutUint16(key[:], uint16(a.args[0]))
				binary.BigEndian.PutUint16(val[:], uint16(a.args[1]))
				s.SetState(addr, key, val)
			},
			args: make([]int64, 2),
		},
		{
			name: "SetCode",
			fn: func(a testAction, s *StateDB) {
				code := make([]byte, 16)
				binary.BigEndian.PutUint64(code, uint64(a.args[0]))
				binary.BigEndian.PutUint64(code[8:], uint64(a.args[1]))
				s.SetCode(addr, code)
			},
			args: make([]int64, 2),
		},
		{
			name: "CreateAccount",
			fn: func(a testAction, s *StateDB) {
				s.CreateAccount(addr)
			},
		},
		{
			name: "Suicide",
			fn: func(a testAction, s *StateDB) {
				s.Suicide(addr)
			},
		},
		{
			name: "AddRefund",
			fn: func(a testAction, s *StateDB) {
				s.AddRefund(uint64(a.args[0]))
			},
			args:   make([]int64, 1),
			noAddr: true,
		},
		{
			name: "AddLog",
			fn: func(a testAction, s *StateDB) {
				data := make([]byte, 2)
				binary.BigEndian.PutUint16(data, uint16(a.args[0]))
				s.AddLog(&types.Log{Address: addr, Data: data})
			},
			args: make([]int64, 1),
		},
	}
	action := actions[r.Intn(len(actions))]
	var nameargs []string
	if !action.noAddr {
		nameargs = append(nameargs, util.Hex(addr[:]))
	}
	for _, i := range action.args {
		action.args[i] = rand.Int63n(100)
		nameargs = append(nameargs, fmt.Sprint(action.args[i]))
	}
	action.name += strings.Join(nameargs, ", ")
	return action
}

// Generate returns a new snapshot test of the given size. All randomness is
// derived from r.
func (*snapshotTest) Generate(r *rand.Rand, size int) reflect.Value {
	// Generate random actions.
	addrs := make([]types.AddressHash, 50)
	for i := range addrs {
		addrs[i][0] = byte(i)
	}
	actions := make([]testAction, size)
	for i := range actions {
		addr := addrs[r.Intn(len(addrs))]
		actions[i] = newTestAction(addr, r)
	}
	// Generate snapshot indexes.
	nsnapshots := int(math.Sqrt(float64(size)))
	if size > 0 && nsnapshots == 0 {
		nsnapshots = 1
	}
	snapshots := make([]int, nsnapshots)
	snaplen := len(actions) / nsnapshots
	for i := range snapshots {
		// Try to place the snapshots some number of actions apart from each other.
		snapshots[i] = (i * snaplen) + r.Intn(snaplen)
	}
	return reflect.ValueOf(&snapshotTest{addrs, actions, snapshots, nil})
}

func (test *snapshotTest) String() string {
	out := new(bytes.Buffer)
	sindex := 0
	for i, action := range test.actions {
		if len(test.snapshots) > sindex && i == test.snapshots[sindex] {
			fmt.Fprintf(out, "---- snapshot %d ----\n", sindex)
			sindex++
		}
		fmt.Fprintf(out, "%4d: %s\n", i, action.name)
	}
	return out.String()
}

func (test *snapshotTest) run() bool {
	// Run all actions and create snapshots.
	var (
		state, _     = New(nil, nil, initDB())
		snapshotRevs = make([]int, len(test.snapshots))
		sindex       = 0
	)
	for i, action := range test.actions {
		if len(test.snapshots) > sindex && i == test.snapshots[sindex] {
			snapshotRevs[sindex] = state.Snapshot()
			sindex++
		}
		action.fn(action, state)
	}
	// Revert all snapshots in reverse order. Each revert must yield a state
	// that is equivalent to fresh state with all actions up the snapshot applied.
	for sindex--; sindex >= 0; sindex-- {
		checkstate, _ := New(nil, nil, state.Database())
		for _, action := range test.actions[:test.snapshots[sindex]] {
			action.fn(action, checkstate)
		}
		state.RevertToSnapshot(snapshotRevs[sindex])
		if err := test.checkEqual(state, checkstate); err != nil {
			test.err = fmt.Errorf("state mismatch after revert to snapshot %d\n%v", sindex, err)
			return false
		}
	}
	return true
}

// checkEqual checks that methods of state and checkstate return the same values.
func (test *snapshotTest) checkEqual(state, checkstate *StateDB) error {
	for _, addr := range test.addrs {
		var err error
		checkeq := func(op string, a, b interface{}) bool {
			if err == nil && !reflect.DeepEqual(a, b) {
				err = fmt.Errorf("got %s(%s) == %v, want %v", op, util.Hex(addr[:]), a, b)
				return false
			}
			return true
		}
		// Check basic accessor methods.
		checkeq("Exist", state.Exist(addr), checkstate.Exist(addr))
		checkeq("HasSuicided", state.HasSuicided(addr), checkstate.HasSuicided(addr))
		checkeq("GetBalance", state.GetBalance(addr), checkstate.GetBalance(addr))
		checkeq("GetNonce", state.GetNonce(addr), checkstate.GetNonce(addr))
		checkeq("GetCode", state.GetCode(addr), checkstate.GetCode(addr))
		checkeq("GetCodeHash", state.GetCodeHash(addr), checkstate.GetCodeHash(addr))
		checkeq("GetCodeSize", state.GetCodeSize(addr), checkstate.GetCodeSize(addr))
		// Check storage.
		if obj := state.getStateObject(addr); obj != nil {
			state.ForEachStorage(addr, func(key, value crypto.HashType) bool {
				return checkeq("GetState("+util.Hex(key[:])+")", checkstate.GetState(addr, key), value)
			})
			checkstate.ForEachStorage(addr, func(key, value crypto.HashType) bool {
				return checkeq("GetState("+util.Hex(key[:])+")", checkstate.GetState(addr, key), value)
			})
		}
		if err != nil {
			return err
		}
	}

	if state.GetRefund() != checkstate.GetRefund() {
		return fmt.Errorf("got GetRefund() == %d, want GetRefund() == %d",
			state.GetRefund(), checkstate.GetRefund())
	}
	if !reflect.DeepEqual(state.GetLogs(crypto.HashType{}), checkstate.GetLogs(crypto.HashType{})) {
		return fmt.Errorf("got GetLogs(crypto.HashType{}) == %v, want GetLogs(crypto.HashType{}) == %v",
			state.GetLogs(crypto.HashType{}), checkstate.GetLogs(crypto.HashType{}))
	}
	return nil
}

func (s *StateSuite) TestTouchDelete(t *testing.T) {
	s.state.getOrNewStateObject(types.AddressHash{})
	s.state.Commit(false)
	s.state.Reset()

	snapshot := s.state.Snapshot()
	s.state.AddBalance(types.AddressHash{}, new(big.Int))

	if len(s.state.journal.dirties) != 1 {
		fmt.Printf("expected one dirty state object")
	}
	s.state.RevertToSnapshot(snapshot)
	if len(s.state.journal.dirties) != 0 {
		fmt.Printf("expected no dirty state object")
	}
}

// TestCopyOfCopy tests that modified objects are carried over to the copy, and the copy of the copy.
func TestCopyOfCopy(t *testing.T) {
	sdb, _ := New(nil, nil, initDB())
	addr := types.HexToAddressHash("aaaa")
	sdb.SetBalance(addr, big.NewInt(42))

	if got := sdb.Copy().GetBalance(addr).Uint64(); got != 42 {
		t.Fatalf("1st copy fail, expected 42, got %v", got)
	}
	if got := sdb.Copy().Copy().GetBalance(addr).Uint64(); got != 42 {
		t.Fatalf("2nd copy fail, expected 42, got %v", got)
	}
}
