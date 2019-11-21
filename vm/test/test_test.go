// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/consensus/bpos"
	"github.com/BOXFoundation/boxd/core/abi"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	coretypes "github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	corecrypto "github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage"
	_ "github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/BOXFoundation/boxd/vm"
	"github.com/BOXFoundation/boxd/vm/common/hexutil" // "github.com/BOXFoundation/boxd/vm/core"
	"github.com/facebookgo/ensure"
	"github.com/jbenet/goprocess"
)

var (
	testHash     = corecrypto.BytesToHash([]byte("xujingshi"))
	fromAddress  = coretypes.BytesToAddressHash([]byte("xujingshi"))
	toAddress    = coretypes.BytesToAddressHash([]byte("andone"))
	amount       = big.NewInt(0)
	nonce        = uint64(0)
	gasLimit     = uint64(100000)
	coinbase     = fromAddress
	blockChain   = chain.NewTestBlockChain()
	adminAddr, _ = types.NewAddress("b1na9uCQXA26d94w1tWrttnnsfjKNz9M2EF")
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func loadBin(filename string) []byte {
	code, err := ioutil.ReadFile(filename)
	must(err)
	return hexutil.MustDecode("0x" + strings.TrimSpace(string(code)))
	//return []byte("0x" + string(code))
}
func loadAbi(filename string) abi.ABI {
	abiFile, err := os.Open(filename)
	must(err)
	defer abiFile.Close()
	abiObj, err := abi.JSON(abiFile)
	must(err)
	return abiObj
}

func initDB() *storage.Database {
	dbCfg := &storage.Config{
		Name: "memdb",
		Path: "~/tmp",
	}
	proc := goprocess.WithSignals(os.Interrupt)
	database, _ := storage.NewDatabase(proc, dbCfg)
	return database
}

// func getVariables(statedb *StateDB, hash common.Address) {
// 	cb := func(key, value common.Hash) bool {
// 		fmt.Printf("key=%x,value=%x\n", key, value)
// 		return true
// 	}

// 	statedb.ForEachStorage(hash, cb)

// }

func Print(outputs []byte, name string) {
	fmt.Printf("method=%s, output=%x\n", name, outputs)
}

type ChainContext struct{}

// get block header
func (cc ChainContext) GetHeader(number uint32) *coretypes.BlockHeader {

	return &coretypes.BlockHeader{
		Height:    number,
		TimeStamp: time.Now().Unix(),
	}
}

func TestGenesisContract(t *testing.T) {
	abiFileName := "./bonus.abi"
	binFileName := "./bonus.bin"
	data := loadBin(binFileName)

	stateDb, _ := state.New(nil, nil, initDB())

	msg := NewMessage(&fromAddress, big.NewInt(0))
	cc := ChainContext{}
	ctx := chain.NewEVMContext(msg, cc.GetHeader(0), blockChain)

	stateDb.SetBalance(fromAddress, big.NewInt(1e18))

	logConfig := vm.LogConfig{}
	structLogger := vm.NewStructLogger(&logConfig)
	vmConfig := vm.Config{Debug: true, Tracer: structLogger /*, JumpTable: vm.NewByzantiumInstructionSet()*/}

	evm := vm.NewEVM(ctx, stateDb, vmConfig)
	contractRef := vm.AccountRef(fromAddress)
	_, contractAddr, _, vmerr := evm.Create(contractRef, data, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	ensure.Nil(t, vmerr)

	ctx = chain.NewEVMContext(msg, cc.GetHeader(15000), blockChain)
	evm = vm.NewEVM(ctx, stateDb, vmConfig)
	abiObj := loadAbi(abiFileName)
	input, err := abiObj.Pack("pledge", "12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cob")
	must(err)
	_, _, vmerr = evm.Call(contractRef, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(1800000*1e8))
	ensure.Nil(t, vmerr)

	input, err = abiObj.Pack("myPledge")
	must(err)
	output, _, vmerr := evm.Call(contractRef, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	ensure.Nil(t, vmerr)
	var result *big.Int
	ensure.Nil(t, abiObj.Unpack(&result, "myPledge", output))
	ensure.DeepEqual(t, result, big.NewInt(1800000*1e8))

	input, err = abiObj.Pack("calcScore", []*big.Int{big.NewInt(100), big.NewInt(200), big.NewInt(300), big.NewInt(500), big.NewInt(800), big.NewInt(999)})
	must(err)
	_, _, vmerr = evm.Call(contractRef, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	ensure.Nil(t, vmerr)

	ctx = chain.NewEVMContext(msg, cc.GetHeader(15249), blockChain)
	evm = vm.NewEVM(ctx, stateDb, vmConfig)
	input, err = abiObj.Pack("execBonus")
	must(err)
	_, _, vmerr = evm.Call(contractRef, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	ensure.Nil(t, vmerr)

	input, err = abiObj.Pack("getLastEpoch")
	must(err)
	output, _, vmerr = evm.Call(contractRef, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	ensure.Nil(t, vmerr)
	var delegates []bpos.Delegate
	ensure.Nil(t, abiObj.Unpack(&delegates, "getLastEpoch", output))
	var res []uint64
	for _, v := range delegates {
		res = append(res, v.Score.Uint64())
	}
	expect := []uint64{999, 800, 500, 300, 200, 100, 0}
	ensure.DeepEqual(t, res, expect)

	input, err = abiObj.Pack("getNetParams")
	must(err)
	output, _, vmerr = evm.Call(contractRef, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	ensure.Nil(t, vmerr)
	var resParams [14]*big.Int
	ensure.Nil(t, abiObj.Unpack(&resParams, "getNetParams", output))
	ensure.DeepEqual(t, resParams[1].Int64(), int64(250))
	ensure.DeepEqual(t, resParams[10].Int64(), int64(200))

}

func _TestEVM(t *testing.T) {
	abiFileName := "./coin_sol_Coin.abi"
	binFileName := "./coin_sol_Coin.bin"
	data := loadBin(binFileName)

	// init db
	stateDb, err := state.New(nil, nil, initDB())

	msg := NewMessage(&fromAddress, big.NewInt(0))
	cc := ChainContext{}
	ctx := chain.NewEVMContext(msg, cc.GetHeader(0), blockChain)

	stateDb.SetBalance(fromAddress, big.NewInt(1e18))
	fmt.Println("init balance =", stateDb.GetBalance(fromAddress))

	// log config
	logConfig := vm.LogConfig{}
	// common.Address => Storage
	structLogger := vm.NewStructLogger(&logConfig)
	vmConfig := vm.Config{Debug: true, Tracer: structLogger /*, JumpTable: vm.NewByzantiumInstructionSet()*/}

	// load evm
	evm := vm.NewEVM(ctx, stateDb, vmConfig)
	// caller
	contractRef := vm.AccountRef(fromAddress)
	// all balance used to create contract as contract.gas
	contractCode, contractAddr, balance, vmerr := evm.Create(contractRef, data, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	stateDb.SetBalance(fromAddress, big.NewInt(int64(balance)))

	must(vmerr)
	//fmt.Printf("getcode:%x\n%x\n", contractCode, statedb.GetCode(contractAddr))
	fmt.Println("contractCode length = ", len(contractCode))

	fmt.Println("after create contract, balance =", balance)

	abiObj := loadAbi(abiFileName)

	// method_id(4B) + args0(32B) + args1(32B) + ...
	input, err := abiObj.Pack("minter")
	must(err)
	outputs, balance, vmerr := evm.Call(contractRef, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	stateDb.SetBalance(fromAddress, big.NewInt(int64(balance)))
	must(vmerr)

	// fmt.Printf("minter is %x\n", common.BytesToAddress(outputs))
	// fmt.Printf("call address %x\n", contractRef)

	sender := coretypes.BytesToAddressHash(outputs)

	if !bytes.Equal(sender.Bytes(), fromAddress.Bytes()) {
		fmt.Println("caller are not equal to minter!!")
		os.Exit(-1)
	}

	senderAcc := vm.AccountRef(sender)

	// mint
	input, err = abiObj.Pack("mint", sender, big.NewInt(1000000))
	must(err)
	outputs, balance, vmerr = evm.Call(senderAcc, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	stateDb.SetBalance(fromAddress, big.NewInt(int64(balance)))
	must(vmerr)
	fmt.Println("after mint, balance =", balance)

	//send
	input, err = abiObj.Pack("send", toAddress, big.NewInt(11))
	outputs, balance, vmerr = evm.Call(senderAcc, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	stateDb.SetBalance(fromAddress, big.NewInt(int64(balance)))
	must(vmerr)
	fmt.Println("after send 11, balance =", balance)

	type RecEvent struct {
		From   coretypes.AddressHash
		To     coretypes.AddressHash
		Amount *big.Int
	}

	var eve RecEvent
	err = abiObj.Unpack(&eve, "Sent", stateDb.Logs()[0].Data)

	fmt.Println(eve)
	must(err)

	//send
	input, err = abiObj.Pack("send", toAddress, big.NewInt(19))
	must(err)
	outputs, balance, vmerr = evm.Call(senderAcc, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	stateDb.SetBalance(fromAddress, big.NewInt(int64(balance)))
	must(vmerr)
	fmt.Println("after send 19, balance =", balance)

	// for _, v := range stateDb.Logs() {
	// 	fmt.Printf("%v\n", v.Data)
	// }

	_, _, err = stateDb.Commit(false)
	must(err)
	err = stateDb.Reset()
	must(err)

	// get receiver balance
	input, err = abiObj.Pack("balances", toAddress)
	must(err)
	outputs, balance, vmerr = evm.Call(contractRef, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	stateDb.SetBalance(fromAddress, big.NewInt(int64(balance)))
	must(vmerr)
	Print(outputs, "balances")
	fmt.Println("after get receiver balance, balance =", balance)

	// get sender balance
	input, err = abiObj.Pack("balances", sender)
	must(err)
	outputs, balance, vmerr = evm.Call(contractRef, contractAddr, input, stateDb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
	stateDb.SetBalance(fromAddress, big.NewInt(int64(balance)))
	must(vmerr)
	Print(outputs, "balances")
	fmt.Println("after get sender balance, balance =", balance)

	stateDb.Commit(true)
	// for _, log := range structLogger.StructLogs() {
	// 	fmt.Println(log)
	// }
}
