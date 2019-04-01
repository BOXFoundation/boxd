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

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/xujingshi/go-evm/common"
	"github.com/xujingshi/go-evm/common/hexutil"
	"github.com/xujingshi/go-evm/common/types"
	"github.com/xujingshi/go-evm/core"
	"github.com/xujingshi/go-evm/state"
	"github.com/xujingshi/go-evm/vm"
)

var (
	testHash    = common.BytesToHash([]byte("xujingshi"))
	fromAddress = common.BytesToAddress([]byte("xujingshi"))
	toAddress   = common.BytesToAddress([]byte("andone"))
	amount      = big.NewInt(0)
	nonce       = uint64(0)
	gasLimit    = uint64(100000)
	coinbase    = fromAddress
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

// func getVariables(statedb *state.StateDB, hash common.Address) {
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
func (cc ChainContext) GetHeader(hash common.Hash, number uint64) *types.Header {

	return &types.Header{
		// ParentHash: common.Hash{},
		// UncleHash:  common.Hash{},
		Coinbase: common.Address(fromAddress),
		//	Root:        common.Hash{},
		//	TxHash:      common.Hash{},
		//	ReceiptHash: common.Hash{},
		//	Bloom:      types.BytesToBloom([]byte("xujingshi")),
		Difficulty: big.NewInt(1),
		Number:     big.NewInt(1).SetUint64(number),
		GasLimit:   gasLimit,
		GasUsed:    0,
		Time:       big.NewInt(time.Now().Unix()),
		Extra:      nil,
		//MixDigest:  testHash,
		//Nonce:      types.EncodeNonce(1),
	}
}

func TestEVMWithoutStorage(t *testing.T) {
	abiFileName := "./coin_sol_Coin.abi"
	binFileName := "./coin_sol_Coin.bin"
	// 从bin读出bytecode，前缀加0x，hex decode成十进制 []uint8
	data := loadBin(binFileName)

	// init db
	msg := core.NewMessage(fromAddress, &toAddress, nonce, amount, gasLimit, big.NewInt(0), data, false)
	cc := ChainContext{}
	ctx := core.NewEVMContext(msg, cc.GetHeader(testHash, 7280001), cc, &fromAddress)
	dataPath := "/Users/mac/development/src/github.com/xujingshi/go-evm/test/data"
	os.Remove(dataPath)

	balance := uint64(big.NewInt(1e18).Uint64())
	fmt.Println("init balance =", balance)

	// log config
	config := types.MainnetChainConfig
	logConfig := vm.LogConfig{}
	// common.Address => Storage
	structLogger := vm.NewStructLogger(&logConfig)
	vmConfig := vm.Config{Debug: true, Tracer: structLogger /*, JumpTable: vm.NewByzantiumInstructionSet()*/}

	// load evm
	evm := vm.NewEVM(ctx, state.NewMockdb(), config, vmConfig)
	// caller
	contractRef := vm.AccountRef(fromAddress)
	// all balance used to create contract as contract.gas
	contractCode, contractAddr, balance, vmerr := evm.Create(contractRef, data, balance, big.NewInt(0))
	must(vmerr)
	//fmt.Printf("getcode:%x\n%x\n", contractCode, statedb.GetCode(contractAddr))
	fmt.Println("contractCode length = ", len(contractCode))

	fmt.Println("after create contract, balance =", balance)

	abiObj := loadAbi(abiFileName)

	// method_id(4B) + args0(32B) + args1(32B) + ...
	input, err := abiObj.Pack("minter")
	must(err)
	// 调用minter()获取矿工地址
	outputs, balance, vmerr := evm.Call(contractRef, contractAddr, input, balance, big.NewInt(0))
	must(vmerr)

	// fmt.Printf("minter is %x\n", common.BytesToAddress(outputs))
	// fmt.Printf("call address %x\n", contractRef)

	sender := common.BytesToAddress(outputs)

	if !bytes.Equal(sender.Bytes(), fromAddress.Bytes()) {
		fmt.Println("caller are not equal to minter!!")
		os.Exit(-1)
	}

	senderAcc := vm.AccountRef(sender)

	// mint
	input, err = abiObj.Pack("mint", sender, big.NewInt(1000000))
	must(err)
	outputs, balance, vmerr = evm.Call(senderAcc, contractAddr, input, balance, big.NewInt(0))
	must(vmerr)
	fmt.Println("after mint, balance =", balance)

	//send
	input, err = abiObj.Pack("send", toAddress, big.NewInt(11))
	outputs, balance, vmerr = evm.Call(senderAcc, contractAddr, input, balance, big.NewInt(0))
	must(vmerr)
	fmt.Println("after send 11, balance =", balance)

	//send
	input, err = abiObj.Pack("send", toAddress, big.NewInt(19))
	must(err)
	outputs, balance, vmerr = evm.Call(senderAcc, contractAddr, input, balance, big.NewInt(0))
	must(vmerr)
	fmt.Println("after send 19, balance =", balance)

	// get receiver balance
	input, err = abiObj.Pack("balances", toAddress)
	must(err)
	outputs, balance, vmerr = evm.Call(contractRef, contractAddr, input, balance, big.NewInt(0))
	must(vmerr)
	Print(outputs, "balances")
	fmt.Println("after get receiver balance, balance =", balance)

	// get sender balance
	input, err = abiObj.Pack("balances", sender)
	must(err)
	outputs, balance, vmerr = evm.Call(contractRef, contractAddr, input, balance, big.NewInt(0))
	must(vmerr)
	Print(outputs, "balances")
	fmt.Println("after get sender balance, balance =", balance)

	for _, log := range structLogger.StructLogs() {
		fmt.Println(log)
	}
}

func TestMsg(t *testing.T) {

	dataA := loadBin("./msg_sol_Msg.bin")
	dataB := loadBin("./msg_sol_Msg_2.bin")

	// init db
	msg := core.NewMessage(fromAddress, &toAddress, nonce, amount, gasLimit, big.NewInt(0), dataA, false)
	cc := ChainContext{}
	ctxA := core.NewEVMContext(msg, cc.GetHeader(testHash, 7280001), cc, &fromAddress)

	msg = core.NewMessage(fromAddress, &toAddress, nonce, amount, gasLimit, big.NewInt(0), dataB, false)
	ctxB := core.NewEVMContext(msg, cc.GetHeader(testHash, 7280001), cc, &fromAddress)
	dataPath := "/Users/mac/development/src/github.com/xujingshi/go-evm/test/data"
	os.Remove(dataPath)

	balance := uint64(big.NewInt(1e18).Uint64())
	fmt.Println("init balance =", balance)

	// log config
	config := types.MainnetChainConfig
	logConfig := vm.LogConfig{}
	// common.Address => Storage
	structLogger := vm.NewStructLogger(&logConfig)
	vmConfig := vm.Config{Debug: true, Tracer: structLogger /*, JumpTable: vm.NewByzantiumInstructionSet()*/}

	// load evm
	evmA := vm.NewEVM(ctxA, state.NewMockdb(), config, vmConfig)
	evmB := vm.NewEVM(ctxB, state.NewMockdb(), config, vmConfig)

	// caller
	contractRef := vm.AccountRef(fromAddress)
	contractCodeA, contractAddrA, balance, vmerr := evmA.Create(contractRef, dataA, balance, big.NewInt(0))
	must(vmerr)
	fmt.Println("contractCode length = ", len(contractCodeA))
	fmt.Println("after create contract, balance =", balance)
	contractCodeB, contractAddrB, balance, vmerr := evmA.Create(contractRef, dataB, balance, big.NewInt(0))
	must(vmerr)
	fmt.Println("contractCode length = ", len(contractCodeB))
	fmt.Println("after create contract, balance =", balance)

	abiObjA := loadAbi("./msg_sol_Msg.abi")
	abiObjB := loadAbi("./msg_sol_Msg_2.abi")

	// method_id(4B) + args0(32B) + args1(32B) + ...
	input, err := abiObjA.Pack("x")
	must(err)
	// 调用x()获取x
	outputs, balance, vmerr := evmA.Call(contractRef, contractAddrA, input, balance, big.NewInt(0))
	must(vmerr)
	fmt.Println(outputs, balance, vmerr)

	input, err = abiObjB.Pack("x")
	must(err)
	// 调用x()获取x
	outputs, balance, vmerr = evmB.Call(contractRef, contractAddrB, input, balance, big.NewInt(0))
	must(vmerr)
	fmt.Println(outputs, balance, vmerr)

	// sender := common.BytesToAddress(outputs)

	// if !bytes.Equal(sender.Bytes(), fromAddress.Bytes()) {
	// 	fmt.Println("caller are not equal to minter!!")
	// 	os.Exit(-1)
	// }

	// senderAcc := vm.AccountRef(sender)

	// // mint
	// input, err = abiObj.Pack("mint", sender, big.NewInt(1000000))
	// must(err)
	// outputs, balance, vmerr = evm.Call(senderAcc, contractAddr, input, balance, big.NewInt(0))
	// must(vmerr)
	// fmt.Println("after mint, balance =", balance)

	// //send
	// input, err = abiObj.Pack("send", toAddress, big.NewInt(11))
	// outputs, balance, vmerr = evm.Call(senderAcc, contractAddr, input, balance, big.NewInt(0))
	// must(vmerr)
	// fmt.Println("after send 11, balance =", balance)

	// //send
	// input, err = abiObj.Pack("send", toAddress, big.NewInt(19))
	// must(err)
	// outputs, balance, vmerr = evm.Call(senderAcc, contractAddr, input, balance, big.NewInt(0))
	// must(vmerr)
	// fmt.Println("after send 19, balance =", balance)

	// // get receiver balance
	// input, err = abiObj.Pack("balances", toAddress)
	// must(err)
	// outputs, balance, vmerr = evm.Call(contractRef, contractAddr, input, balance, big.NewInt(0))
	// must(vmerr)
	// Print(outputs, "balances")
	// fmt.Println("after get receiver balance, balance =", balance)

	// // get sender balance
	// input, err = abiObj.Pack("balances", sender)
	// must(err)
	// outputs, balance, vmerr = evm.Call(contractRef, contractAddr, input, balance, big.NewInt(0))
	// must(vmerr)
	// Print(outputs, "balances")
	// fmt.Println("after get sender balance, balance =", balance)

	// for _, log := range structLogger.StructLogs() {
	// 	fmt.Println(log)
	// }
}

// func TestEVM(t *testing.T) {
// 	abiFileName := "./coin_sol_Coin.abi"
// 	binFileName := "./coin_sol_Coin.bin"
// 	data := loadBin(binFileName)

// 	// init db
// 	msg := ec.NewMessage(fromAddress, &toAddress, nonce, amount, gasLimit, big.NewInt(0), data, false)
// 	cc := ChainContext{}
// 	ctx := ec.NewEVMContext(msg, cc.GetHeader(testHash, 7280001), cc, &fromAddress)
// 	dataPath := "/Users/mac/development/src/github.com/xujingshi/go-evm/test/data"
// 	os.Remove(dataPath)
// 	mdb, err := ethdb.NewLDBDatabase(dataPath, 100, 100)
// 	must(err)

// 	// FIXME: whats the diff from db and statedb?
// 	db := state.NewDatabase(mdb)
// 	root := common.Hash{}
// 	statedb, err := state.New(root, db)
// 	must(err)

// 	//set balance
// 	statedb.GetOrNewStateObject(fromAddress)
// 	statedb.GetOrNewStateObject(toAddress)
// 	statedb.AddBalance(fromAddress, big.NewInt(1e18))
// 	fmt.Println("init testBalance =", statedb.GetBalance(fromAddress))
// 	must(err)

// 	// log config
// 	config := params.MainnetChainConfig
// 	logConfig := vm.LogConfig{}
// 	// common.Address => Storage
// 	structLogger := vm.NewStructLogger(&logConfig)
// 	vmConfig := vm.Config{Debug: true, Tracer: structLogger /*, JumpTable: vm.NewByzantiumInstructionSet()*/}

// 	// load evm
// 	evm := vm.NewEVM(ctx, statedb, config, vmConfig)
// 	// caller
// 	contractRef := vm.AccountRef(fromAddress)
// 	contractCode, contractAddr, gasLeftover, vmerr := evm.Create(contractRef, data, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
// 	must(vmerr)
// 	//fmt.Printf("getcode:%x\n%x\n", contractCode, statedb.GetCode(contractAddr))

// 	statedb.SetBalance(fromAddress, big.NewInt(0).SetUint64(gasLeftover))
// 	fmt.Println("after create contract, testBalance =", statedb.GetBalance(fromAddress))

// 	abiObj := loadAbi(abiFileName)

// 	// method_id(4B) + args0(32B) + args1(32B) + ...
// 	input, err := abiObj.Pack("minter")
// 	must(err)
// 	// 调用minter()获取矿工地址
// 	outputs, gasLeftover, vmerr := evm.Call(contractRef, contractAddr, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
// 	must(vmerr)

// 	// fmt.Printf("minter is %x\n", common.BytesToAddress(outputs))
// 	// fmt.Printf("call address %x\n", contractRef)

// 	sender := common.BytesToAddress(outputs)

// 	if !bytes.Equal(sender.Bytes(), fromAddress.Bytes()) {
// 		fmt.Println("caller are not equal to minter!!")
// 		os.Exit(-1)
// 	}

// 	senderAcc := vm.AccountRef(sender)

// 	// mint
// 	input, err = abiObj.Pack("mint", sender, big.NewInt(1000000))
// 	must(err)
// 	outputs, gasLeftover, vmerr = evm.Call(senderAcc, contractAddr, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
// 	must(vmerr)
// 	statedb.SetBalance(fromAddress, big.NewInt(0).SetUint64(gasLeftover))
// 	fmt.Println("after mint, testBalance =", gasLeftover)

// 	//send
// 	input, err = abiObj.Pack("send", toAddress, big.NewInt(11))
// 	outputs, gasLeftover, vmerr = evm.Call(senderAcc, contractAddr, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
// 	must(vmerr)
// 	fmt.Println("after send 11, testBalance =", gasLeftover)

// 	//send
// 	input, err = abiObj.Pack("send", toAddress, big.NewInt(19))
// 	must(err)
// 	outputs, gasLeftover, vmerr = evm.Call(senderAcc, contractAddr, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
// 	must(vmerr)
// 	fmt.Println("after send 19, testBalance =", gasLeftover)

// 	// get receiver balance
// 	input, err = abiObj.Pack("balances", toAddress)
// 	must(err)
// 	outputs, gasLeftover, vmerr = evm.Call(contractRef, contractAddr, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
// 	must(vmerr)
// 	Print(outputs, "balances")
// 	fmt.Println("after get receiver balance, testBalance =", gasLeftover)

// 	// get sender balance
// 	input, err = abiObj.Pack("balances", sender)
// 	must(err)
// 	outputs, gasLeftover, vmerr = evm.Call(contractRef, contractAddr, input, statedb.GetBalance(fromAddress).Uint64(), big.NewInt(0))
// 	must(vmerr)
// 	Print(outputs, "balances")
// 	fmt.Println("after get sender balance, testBalance =", gasLeftover)

// 	// get event
// 	logs := statedb.Logs()

// 	for _, log := range logs {
// 		fmt.Printf("%#v\n", log)
// 		for _, topic := range log.Topics {
// 			fmt.Printf("topic: %#v\n", topic)
// 		}
// 		fmt.Printf("data: %#v\n", log.Data)
// 	}

// 	root, err = statedb.Commit(true)
// 	must(err)
// 	err = db.TrieDB().Commit(root, true)
// 	must(err)

// 	fmt.Println("Root Hash", root.Hex())
// 	mdb.Close()

// 	mdb2, err := ethdb.NewLDBDatabase(dataPath, 100, 100)
// 	defer mdb2.Close()
// 	must(err)
// 	db2 := state.NewDatabase(mdb2)
// 	statedb2, err := state.New(root, db2)
// 	must(err)
// 	fmt.Println("get testBalance =", statedb2.GetBalance(fromAddress))
// 	if !bytes.Equal(contractCode, statedb2.GetCode(contractAddr)) {
// 		fmt.Println("BUG!,the code was changed!")
// 		os.Exit(-1)
// 	}
// 	getVariables(statedb2, contractAddr)
// }
