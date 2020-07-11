// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package chain
package chain

import (
	"bytes"
	"encoding/hex"
	"io/ioutil"
	"log"
	"math/big"
	"strings"
	"testing"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/abi"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/facebookgo/ensure"
)

const (
	testContractDir = "./test_contracts/"
)

func TestExtractBoxTx(t *testing.T) {
	// contract Temp {
	//     function () payable {}
	// }
	testVMScriptCode := "6060604052346000575b60398060166000396000f30060606040525b600b" +
		"5b5b565b0000a165627a7a723058209cedb722bf57a30e3eb00eeefc392103ea791a2001deed29" +
		"f5c3809ff10eb1dd0029"
	var tests = []struct {
		value          uint64
		fromStr, toStr string
		code           string
		limit          uint64
		nonce          uint64
		version        int32
		err            error
	}{
		{100, "b1VAnrX665aeExMaPeW6pk3FZKCLuywUaHw", "b5WYphc4yBPH18gyFthS1bHyRcEvM6xANuT",
			testVMScriptCode, 20000, 1, 0, nil},
		{0, "b1VAnrX665aeExMaPeW6pk3FZKCLuywUaHw", "", testVMScriptCode, 20000, 2, 0, nil},
	}
	db, _ := memdb.NewMemoryDB("", nil)
	stateDB, _ := state.New(nil, nil, db)
	for _, tc := range tests {
		var (
			from, to      types.Address
			toAddressHash *types.AddressHash
			cs            *script.Script
			err           error
		)
		from, _ = types.NewAddress(tc.fromStr)
		code, _ := hex.DecodeString(tc.code)
		if tc.toStr != "" {
			to, _ = types.NewContractAddress(tc.toStr)
			toAddressHash = to.Hash160()
		} else {
			toAddressHash = new(types.AddressHash)
		}
		cs, err = script.MakeContractScriptPubkey(from.Hash160(), toAddressHash,
			tc.limit, tc.nonce, tc.version)
		if err != nil {
			t.Fatal(err)
		}
		hash := new(crypto.HashType)
		testExtractPrevHash := "c0e96e998eb01eea5d5acdaeb80acd943477e6119dcd82a419089331229c7453"
		hashBytes, _ := hex.DecodeString(testExtractPrevHash)
		hash.SetBytes(hashBytes)
		prevOp := types.NewOutPoint(hash, 0)
		txin := types.NewTxIn(prevOp, nil, 0)
		txout := types.NewTxOut(tc.value, *cs)
		tx := types.NewTx(0, 4455, 100).AppendVin(txin).AppendVout(txout).
			WithData(types.ContractDataType, code)
		btx, err := ExtractVMTransaction(tx, stateDB)
		if err != nil {
			t.Fatal(err)
		}
		// check
		hashWith, _ := tx.TxHash()
		if *btx.OriginTxHash() != *hashWith ||
			*btx.From() != *from.Hash160() ||
			(btx.To() != nil && *btx.To() != *toAddressHash) ||
			btx.Value().Cmp(big.NewInt(int64(tc.value))) != 0 ||
			btx.Gas() != tc.limit || btx.Nonce() != tc.nonce ||
			btx.Version() != tc.version {
			t.Fatalf("want: %+v originTxHash: %s, got BoxTransaction: %+v", tc, hashWith, btx)
		}
	}
}

var (
	userBalance, minerBalance, contractBalance uint64
)

type testContractParam struct {
	vmValue, gasLimit, contractBalance, userRecv uint64

	contractAddr *types.AddressContract
}

func genTestChain(t *testing.T, blockChain *BlockChain) *types.Block {
	var err error
	b0 := blockChain.tail

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1
	timestamp = startTime
	b1 := nextBlockV3(b0, blockChain, 0)
	if err := connectBlock(b0, b1, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b1, nil, 1, b1)

	// check balance
	contractKey, _ := types.NewContractAddressFromHash(ContractAddr.Bytes())
	balance := getBalance(contractKey.Hash160(), blockChain.db)
	addr, _ := types.NewAddress(ContractAddr.String())
	stateBalance := blockChain.tailState.GetBalance(*addr.Hash160()).Uint64()
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, BaseSubsidy)
	t.Logf("b0 -> b1 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b1 -> b2
	// transfer some box to minerAddr and userAddr
	// generate b2
	minerBalance = uint64(1000000000)
	prevHash := new(crypto.HashType)
	prevHash.SetString("543b802cab2e20848c24c51c8d13a8644866766b48f85ced5fe4773351296020")
	toMinerTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(txlogic.MakeVout(minerAddr.Hash160(), minerBalance)).
		AppendVout(txlogic.MakeVout(preAddr.Hash160(), 33000000000000000-minerBalance-core.TransferFee))
	err = txlogic.SignTx(toMinerTx, privKeyPre, pubKeyPre)
	ensure.DeepEqual(t, err, nil)

	userBalance = uint64(300000000)
	prevHash, _ = toMinerTx.TxHash()
	toUserTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), userBalance)).
		AppendVout(txlogic.MakeVout(minerAddr.Hash160(), minerBalance-userBalance-core.TransferFee))
	err = txlogic.SignTx(toUserTx, privKeyMiner, pubKeyMiner)
	ensure.DeepEqual(t, err, nil)
	b2 := nextBlockV3(b1, blockChain, 0, toMinerTx, toUserTx)
	if err := connectBlock(b1, b2, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b2, nil, 2, b2)
	// check balance
	// for userAddr
	t.Logf("miner address: %x", minerAddr.Hash160()[:])
	balance = getBalance(userAddr.Hash160(), blockChain.db)
	stateBalance = blockChain.tailState.GetBalance(*userAddr.Hash160()).Uint64()
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, userBalance)
	t.Logf("user balance: %d", userBalance)
	// for miner
	balance = getBalance(minerAddr.Hash160(), blockChain.db)
	stateBalance = blockChain.tailState.GetBalance(*minerAddr.Hash160()).Uint64()
	minerBalance = minerBalance - userBalance - core.TransferFee + 2*core.TransferFee
	t.Logf("expect miner balance: %d", minerBalance)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, minerBalance)
	t.Logf("b1 -> b2 passed, now tail height: %d", blockChain.LongestChainHeight)
	return b2
}

func contractBlockHandle(
	t *testing.T, blockChain *BlockChain, block, tail *types.Block,
	param *testContractParam, err error,
) {

	verifyProcessBlock(t, blockChain, block, err, tail.Header.Height, tail)
	// check balance
	expectUserBalance := userBalance - param.vmValue - block.Header.GasUsed + param.userRecv
	expectMinerBalance := minerBalance
	if err != nil {
		expectUserBalance, expectMinerBalance = userBalance, minerBalance
	} else if len(block.Txs[0].Vout) == 2 {
		expectMinerBalance += block.Txs[0].Vout[1].Value
	}
	// for userAddr
	checkTestAddrBalance(t, blockChain, userAddr, expectUserBalance)
	t.Logf("user %s balance: %d", userAddr, expectUserBalance)
	userBalance = expectUserBalance
	// for miner
	checkTestAddrBalance(t, blockChain, minerAddr, expectMinerBalance)
	t.Logf("user %s balance: %d", minerAddr, expectMinerBalance)
	minerBalance = expectMinerBalance
	// for contract address
	checkTestAddrBalance(t, blockChain, param.contractAddr, param.contractBalance)
	t.Logf("contract %s balance: %d", param.contractAddr, param.contractBalance)
	contractBalance = param.contractBalance
	// for admin contract address
	bonusContractAddr, _ := types.NewContractAddressFromHash(ContractAddr[:])
	bonusBalance := blockChain.tailState.GetBalance(ContractAddr).Uint64()
	t.Logf("bonus contract %s balance: %d", bonusContractAddr, bonusBalance)
}

func checkTestAddrBalance(t *testing.T, bc *BlockChain, addr types.Address, expect uint64) {
	utxoBalance := getBalance(addr.Hash160(), bc.db)
	stateBalance := bc.tailState.GetBalance(*addr.Hash160()).Uint64()
	t.Logf("%s utxo balance: %d state balance: %d, expect: %d", addr, utxoBalance,
		stateBalance, expect)
	ensure.DeepEqual(t, utxoBalance, expect, "incorrect utxo balance for "+addr.String())
	ensure.DeepEqual(t, stateBalance, expect, "incorrect state balance for "+addr.String())
}

const (
	/*
			pragma solidity ^0.5.1;
			contract Faucet {
		    // Give out ether to anyone who asks
		    function withdraw(uint withdraw_amount) public {
		        // Limit withdrawal amount
		        require(withdraw_amount <= 10000);
		        // Send the amount to the address that requested it
		        msg.sender.transfer(withdraw_amount);
		    }
		    // Accept any incoming amount
		    function () external payable  {}
		    // Create a new ballot with $(_numProposals) different proposals.
		    constructor() public payable {}
			}
	*/
	testFaucetContract = "608060405260f7806100126000396000f3fe60806040526004361060395" +
		"76000357c0100000000000000000000000000000000000000000000000000000000900480632e1a7" +
		"d4d14603b575b005b348015604657600080fd5b50607060048036036020811015605b57600080fd5" +
		"b81019080803590602001909291905050506072565b005b6127108111151515608257600080fd5b3" +
		"373ffffffffffffffffffffffffffffffffffffffff166108fc82908115029060405160006040518" +
		"0830381858888f1935050505015801560c7573d6000803e3d6000fd5b505056fea165627a7a72305" +
		"82041951f9857bb67cda6bccbb59f6fdbf38eeddc244530e577d8cad6194941d38c0029"
	// withdraw 2000
	testFaucetCall = "2e1a7d4d00000000000000000000000000000000000000000000000000000000000007d0"
	// withdraw 20000
	testFaucetCall2 = "2e1a7d4d0000000000000000000000000000000000000000000000000000000000004e20"
)

func TestFaucetContract(t *testing.T) {
	blockChain := NewTestBlockChain()
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)

	// contract blocks test
	b2 := genTestChain(t, blockChain)

	// b2 -> b21
	// normal transfer to a contract address that have not been created
	contractAddr, _ := types.MakeContractAddress(userAddr, 1)
	prevHash, _ := b2.Txs[2].TxHash()
	toContractAmount := uint64(1000)
	changeValue2 := userBalance - toContractAmount - core.TransferFee
	toContractTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(txlogic.MakeVout(contractAddr.Hash160(), toContractAmount)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue2))
	txlogic.SignTx(toContractTx, privKey, pubKey)
	userBalance = changeValue2
	b21 := nextBlockV3(b2, blockChain, 0, toContractTx)
	if err := connectBlock(b2, b21, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b21, nil, 3, b21)
	minerBalance = blockChain.tailState.GetBalance(*minerAddr.Hash160()).Uint64()
	t.Logf("b2 -> b21 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain, with pay to contract address tx and contract creation tx
	// b21 -> b3
	// normal transfer to a contract address that have not been created
	prevHash, _ = b21.Txs[1].TxHash()
	changeValue2 -= toContractAmount + core.TransferFee
	toContractTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(contractAddr.Hash160(), toContractAmount)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue2))
	txlogic.SignTx(toContractTx, privKey, pubKey)
	userBalance = changeValue2

	vmValue, gasLimit := uint64(10000), uint64(200000)
	vmParam := &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, vmValue + 2*toContractAmount, 0, nil,
	}

	byteCode, _ := hex.DecodeString(testFaucetContract)
	nonce := uint64(1)
	contractVout, err := txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	prevHash, _ = toContractTx.TxHash()
	changeValue2 = changeValue2 - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue2)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	contractAddr, _ = types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	b3 := nextBlockV3(b21, blockChain, 0, toContractTx, vmTx)
	if err := connectBlock(b21, b3, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b3, b3, vmParam, nil)
	stateDB, err := state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)

	gasRefundValue := gasPrice*vmParam.gasLimit - b3.Header.GasUsed
	t.Logf("b21 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b4
	// make call contract tx
	vmValue, gasLimit = uint64(0), uint64(20000)
	contractBalance -= 2000 // withdraw 2000, construct contract with 10000
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, contractBalance, 2000, contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3.InternalTxs[0].TxHash()
	changeValue3 := gasRefundValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue3)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)

	b4 := nextBlockV3(b3, blockChain, 0, vmTx)
	if err := connectBlock(b3, b4, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b4, b4, vmParam, nil)
	stateDB, err = state.New(&b4.Header.RootHash, &b4.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)
	t.Logf("b3 -> b4 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b4 -> b5
	// make creation contract tx with insufficient gas
	vmValue, gasLimit = uint64(1000), uint64(20000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, contractBalance, vmValue, vmParam.contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetContract)
	nonce++
	contractVout, err = txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	prevHash, _ = b3.Txs[2].TxHash()
	changeValue4 := changeValue2 - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue4)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)

	b5 := nextBlockV3(b4, blockChain, 0, vmTx)
	if err := connectBlock(b4, b5, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b5, b5, vmParam, nil)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)
	t.Logf("b4 -> b5 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make creation contract tx with insufficient balance
	vmValue, gasLimit = uint64(1000), uint64(600000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, contractBalance, vmValue, vmParam.contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetContract)
	nonce++
	contractVout, err = txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	prevHash, _ = b5.InternalTxs[0].TxHash()
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b6 := nextBlockV3(b5, blockChain, 0, vmTx)
	if err := connectBlock(b5, b6, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b6, b5, vmParam, core.ErrInvalidFee)
	nonce--
	t.Logf("b5 -> b6 failed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make call contract tx with insufficient contract balance
	vmValue, gasLimit = uint64(0), uint64(20000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, contractBalance, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall2)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue6 := changeValue4 - vmValue - gasPrice*gasLimit
	logger.Warnf("utxo value: %d, gas: %d", changeValue4, gasPrice*gasLimit)
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue6)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b6 = nextBlockV3(b5, blockChain, 0, vmTx)
	if err := connectBlock(b5, b6, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b6, b6, vmParam, nil)
	stateDB, err = state.New(&b6.Header.RootHash, &b6.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)
	t.Logf("b5 -> b6 passed, now tail height: %d", blockChain.LongestChainHeight)

	// b6 - b7
	// normal transfer to a contract address that have not been created
	prevHash, _ = b6.Txs[1].TxHash()
	changeValue6 -= toContractAmount + core.TransferFee
	toContractTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(txlogic.MakeVout(contractAddr.Hash160(), toContractAmount)).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue6))
	txlogic.SignTx(toContractTx, privKey, pubKey)
	toContractTxHash, _ := toContractTx.TxHash()
	t.Logf("to contract tx hash: %s", toContractTxHash)
	userBalance -= toContractAmount
	vmParam.contractBalance += toContractAmount
	b7 := nextBlockV3(b6, blockChain, 0, toContractTx)
	if err := connectBlock(b6, b7, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b7, b7, vmParam, nil)
	t.Logf("b6 -> b7 passed, now tail height: %d", blockChain.LongestChainHeight)
}

const (
	/*
		pragma solidity ^0.5.6;  //The lowest compiler version

		contract Coin {
		    // The keyword "public" makes those variables
		    // readable from outside.
		    address public minter;
		    mapping (address => uint) public balances;

		    // Events allow light clients to react on
		    // changes efficiently.
		    event Sent(address from, address to, uint amount);

		    // This is the constructor whose code is
		    // run only when the contract is created.
		    constructor() public {
		        minter = msg.sender;
		    }

		    function mint(address receiver, uint amount) public {
		        if (msg.sender != minter) return;
		        balances[receiver] += amount;
		    }

		    function send(address receiver, uint amount) public {
		        if (balances[msg.sender] < amount) return ;
		        balances[msg.sender] -= amount;
		        balances[receiver] += amount;
		        emit Sent(msg.sender, receiver, amount);
		    }
		}
	*/
	testCoinContract = "608060405234801561001057600080fd5b50336000806101000a81548173" +
		"ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffff" +
		"ffffffffffff16021790555061042d806100606000396000f3fe6080604052348015610010576000" +
		"80fd5b506004361061004c5760003560e01c8063075461721461005157806327e235e31461009b57" +
		"806340c10f19146100f3578063d0679d3414610141575b600080fd5b61005961018f565b60405180" +
		"8273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffff" +
		"ffffffff16815260200191505060405180910390f35b6100dd600480360360208110156100b15760" +
		"0080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190" +
		"5050506101b4565b6040518082815260200191505060405180910390f35b61013f60048036036040" +
		"81101561010957600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16" +
		"9060200190929190803590602001909291905050506101cc565b005b61018d600480360360408110" +
		"1561015757600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060" +
		"20019092919080359060200190929190505050610277565b005b6000809054906101000a900473ff" +
		"ffffffffffffffffffffffffffffffffffffff1681565b6001602052806000526040600020600091" +
		"5090505481565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff" +
		"1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffff" +
		"ffffffffff161461022557610273565b80600160008473ffffffffffffffffffffffffffffffffff" +
		"ffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020" +
		"600082825401925050819055505b5050565b80600160003373ffffffffffffffffffffffffffffff" +
		"ffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160" +
		"00205410156102c3576103fd565b80600160003373ffffffffffffffffffffffffffffffffffffff" +
		"ff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000" +
		"828254039250508190555080600160008473ffffffffffffffffffffffffffffffffffffffff1673" +
		"ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254" +
		"01925050819055507f3990db2d31862302a685e8086b5755072a6e2b5b780af1ee81ece35ee3cd33" +
		"45338383604051808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffff" +
		"ffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff" +
		"1673ffffffffffffffffffffffffffffffffffffffff168152602001828152602001935050505060" +
		"405180910390a15b505056fea165627a7a723058200cf7e1d90be79a04377bc832a4cd9b545f25e8" +
		"253d7c83b1c72529f73c0888c60029"

	coinAbi = `[{"constant":true,"inputs":[],"name":"minter","outputs":[{"name":"",
	"type":"address"}],"payable":false,"stateMutability":"view","type":"function"},
	{"constant":true,"inputs":[{"name":"","type":"address"}],"name":"balances",
	"outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view",
	"type":"function"},{"constant":false,"inputs":[{"name":"receiver","type":"address"},
	{"name":"amount","type":"uint256"}],"name":"mint","outputs":[],"payable":false,
	"stateMutability":"nonpayable","type":"function"},{"constant":false,
	"inputs":[{"name":"receiver","type":"address"},{"name":"amount","type":"uint256"}],
	"name":"send","outputs":[],"payable":false,"stateMutability":"nonpayable",
	"type":"function"},{"inputs":[],"payable":false,"stateMutability":"nonpayable",
	"type":"constructor"},{"anonymous":false,"inputs":[{"indexed":false,"name":"from",
	"type":"address"},{"indexed":false,"name":"to","type":"address"},{"indexed":false,
	"name":"amount","type":"uint256"}],"name":"Sent","type":"event"}]`
)

var mintCall, sendCall, balancesUserCall, balancesReceiverCall string

func init() {
	// balances
	receiver, err := types.NewContractAddress("b5WYphc4yBPH18gyFthS1bHyRcEvM6xANuT")
	if err != nil {
		log.Fatal(err)
	}
	abiObj, err := abi.JSON(strings.NewReader(coinAbi))
	if err != nil {
		log.Fatal(err)
	}
	// mint 8000000
	//toAddress := types.BytesToAddressHash([]byte("andone"))
	input, err := abiObj.Pack("mint", *userAddr.Hash160(), big.NewInt(8000000))
	//input, err := abiObj.Pack("mint", toAddress, big.NewInt(8000000))
	if err != nil {
		log.Fatal(err)
	}
	mintCall = hex.EncodeToString(input)
	// sent 2000000
	input, err = abiObj.Pack("send", *receiver.Hash160(), big.NewInt(2000000))
	if err != nil {
		log.Fatal(err)
	}
	sendCall = hex.EncodeToString(input)
	// balances user addr
	input, err = abiObj.Pack("balances", *userAddr.Hash160())
	if err != nil {
		log.Fatal(err)
	}
	balancesUserCall = hex.EncodeToString(input)
	// balances test Addr
	input, err = abiObj.Pack("balances", receiver.Hash160())
	if err != nil {
		log.Fatal(err)
	}
	balancesReceiverCall = hex.EncodeToString(input)
}

func TestCoinContract(t *testing.T) {

	blockChain := NewTestBlockChain()
	// blockchain
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)
	// contract blocks test
	b2 := genTestChain(t, blockChain)
	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b2 -> b3
	// make creation contract tx
	vmValue, gasLimit := uint64(0), uint64(400000)
	vmParam := &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, vmValue, 0, nil,
	}
	byteCode, _ := hex.DecodeString(testCoinContract)
	nonce := uint64(1)
	contractVout, err := txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[2].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	b3 := nextBlockV3(b2, blockChain, 0, vmTx)
	if err := connectBlock(b2, b3, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b3, b3, vmParam, nil)
	stateDB, err := state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b4
	// make mint 8000000 call contract tx
	vmValue, gasLimit = uint64(0), uint64(30000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(mintCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b4 := nextBlockV3(b3, blockChain, 0, vmTx)

	if err := connectBlock(b3, b4, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b4, b4, vmParam, nil)
	stateDB, err = state.New(&b4.Header.RootHash, &b4.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b3 -> b4 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b4 -> b5
	// make send 2000000 call contract tx
	vmValue, gasLimit = uint64(0), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(sendCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b4.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b5 := nextBlockV3(b4, blockChain, 0, vmTx)

	if err := connectBlock(b4, b5, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b5, b5, vmParam, nil)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b4 -> b5 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make balances user call contract tx
	vmValue, gasLimit = uint64(0), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balancesUserCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b6 := nextBlockV3(b5, blockChain, 0, vmTx)

	if err := connectBlock(b5, b6, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b6, b6, vmParam, nil)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b5 -> b6 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 0x5b8d80 = 6000000, check okay

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b6 -> b7
	// make balances receiver call contract tx
	vmValue, gasLimit = uint64(0), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balancesReceiverCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b6.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b7 := nextBlockV3(b6, blockChain, 0, vmTx)

	if err := connectBlock(b6, b7, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b7, b7, vmParam, nil)
	stateDB, err = state.New(&b7.Header.RootHash, &b7.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b6 -> b7 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 0x1e8480 = 2000000, check okay
}

func TestERC20Contract(t *testing.T) {

	testContractDir := "./test_contracts/"
	binFile, abiFile := testContractDir+"erc20.bin", testContractDir+"erc20.abi"
	code, err := ioutil.ReadFile(binFile)
	if err != nil {
		t.Fatal(err)
	}
	erc20Code, _ := hex.DecodeString(string(bytes.TrimSpace(code)))
	abiObj, err := ReadAbi(abiFile)
	if err != nil {
		t.Fatal(err)
	}
	// balances
	userA, _ := types.NewAddress("b1afgd4BC3Y81ni3ds2YETikEkprG9Bxo98")
	// inputs
	// transfer 2000000
	transferCall, err := abiObj.Pack("transfer", *userA.Hash160(), big.NewInt(2000000))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("transfer 2000000: %x", transferCall)
	// balances user addr
	balanceOfUserCall, err := abiObj.Pack("balanceOf", *userAddr.Hash160())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("balanceUser: %x", balanceOfUserCall)
	// balances test Addr
	balanceOfReceiverCall, err := abiObj.Pack("balanceOf", userA.Hash160())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("balance of %s: %x", userA, balanceOfReceiverCall)
	// transferFrom, sender miner, from userAddr, to userA
	transferFromCall, err := abiObj.Pack("transferFrom", *userAddr.Hash160(), *userA.Hash160(), big.NewInt(50000))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("transferFrom %s to %s 50000: %x", userAddr, userA, transferFromCall)
	// aprove 40000 for miner, sender userAddr, spender miner
	approveCall, err := abiObj.Pack("approve", *minerAddr.Hash160(), big.NewInt(40000))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("approve miner %s can spend %s 40000: %x", minerAddr, userAddr, approveCall)
	// increaseAllowance 20000 for miner, sender userAddr, spender miner
	increaseAllowanceCall, err := abiObj.Pack("increaseAllowance", *minerAddr.Hash160(), big.NewInt(20000))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("increase %s's allowance 20000: %x", minerAddr, increaseAllowanceCall)
	// allowance owner user for spender miner
	allowanceCall, err := abiObj.Pack("allowance", *userAddr.Hash160(), *minerAddr.Hash160())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("allowance user %s miner %s: %x", userAddr, minerAddr, allowanceCall)
	//
	blockChain := NewTestBlockChain()
	// blockchain
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)
	// contract blocks test
	b2 := genTestChain(t, blockChain)
	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b2 -> b3
	// deploy contract
	vmValue, gasLimit := uint64(0), uint64(2000000)
	vmParam := &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, vmValue, 0, nil,
	}
	nonce := uint64(1)
	contractVout, err := txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[2].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, erc20Code)
	txlogic.SignTx(vmTx, privKey, pubKey)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	t.Logf("user addr: %s", hex.EncodeToString(userAddr.Hash160()[:]))
	b3 := nextBlockV3(b2, blockChain, 0, vmTx)

	if err := connectBlock(b2, b3, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b3, b3, vmParam, nil)
	stateDB, err := state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b3A
	// make balances user call contract tx
	vmValue, gasLimit = uint64(0), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, balanceOfUserCall)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b3A := nextBlockV3(b3, blockChain, 0, vmTx)

	if err := connectBlock(b3, b3A, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b3A, b3A, vmParam, nil)
	stateDB, err = state.New(&b3A.Header.RootHash, &b3A.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b3 -> b3A passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 0010a5d4e8000000000000000000000000000000000000000000000000000000
	// 0xe8d4a51000 = 10000*10^8, check okay
	userBal, ok := new(big.Int).SetString("000000000000000000000000000000000000000000000000000000e8d4a51000", 16)
	if !ok {
		t.Fatal("parse user balance failed")
	}
	t.Logf("balance of user %s return value: %d", userAddr, userBal)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3A -> b4
	// make transfer 2000000 call contract tx
	vmValue, gasLimit = uint64(0), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3A.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, transferCall)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b4 := nextBlockV3(b3A, blockChain, 0, vmTx)

	if err := connectBlock(b3A, b4, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b4, b4, vmParam, nil)
	stateDB, err = state.New(&b4.Header.RootHash, &b4.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b3A -> b4 passed, now tail height: %d", blockChain.LongestChainHeight)
	// execute transfer, return "0100000000000000000000000000000000000000000000000000000000000000"
	// true

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b4 -> b5
	// make balances user call contract tx
	vmValue, gasLimit = uint64(0), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b4.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, balanceOfUserCall)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b5 := nextBlockV3(b4, blockChain, 0, vmTx)

	if err := connectBlock(b4, b5, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b5, b5, vmParam, nil)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b4 -> b5 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 808b86d4e8000000000000000000000000000000000000000000000000000000
	// 0xe8d4868b80 = 999998000000, check okay
	userBal, ok = new(big.Int).SetString("e8d4868b80", 16)
	if !ok {
		t.Fatal("parse user balance failed")
	}
	t.Logf("call balance of user return value: %d", userBal)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make balances userA call contract tx
	vmValue, gasLimit = uint64(0), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, balanceOfReceiverCall)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b6 := nextBlockV3(b5, blockChain, 0, vmTx)

	if err := connectBlock(b5, b6, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b6, b6, vmParam, nil)
	stateDB, err = state.New(&b6.Header.RootHash, &b6.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b5 -> b6 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 80841e0000000000000000000000000000000000000000000000000000000000
	// 0x1e8480 = 2000000, check okay

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b6 -> b7
	// transferFrom user to userA 40000, failed contract execution
	vmValue, gasLimit = uint64(0), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	// minerNonce := uint64(9)
	minerNonce := stateDB.GetNonce(*minerAddr.Hash160()) + 1 // note: coinbase tx has already add 1.
	contractVout, err = txlogic.MakeContractCallVout(minerAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, minerNonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b2.Txs[2].TxHash()
	minerChangeValue := b2.Txs[2].Vout[1].Value - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(minerAddr.Hash160(), minerChangeValue)).
		WithData(types.ContractDataType, transferFromCall)
	txlogic.SignTx(vmTx, privKeyMiner, pubKeyMiner)
	b7 := nextBlockV3(b6, blockChain, 0, vmTx)
	if err := connectBlock(b6, b7, blockChain); err != nil {
		t.Fatal(err)
	}
	userBalance += b7.Header.GasUsed
	minerBalance -= b7.Header.GasUsed
	contractBlockHandle(t, blockChain, b7, b7, vmParam, nil)
	stateDB, err = state.New(&b7.Header.RootHash, &b7.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("miner nonce: %d", stateDB.GetNonce(*minerAddr.Hash160()))
	t.Logf("b6 -> b7 passed, now tail height: %d", blockChain.LongestChainHeight)
	// transferFrom returns
	// return 0000776f6c667265766f206e6f697463617274627573203a6874614d65666153

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b7 -> b8
	// approve miner spend user 40000
	vmValue, gasLimit = uint64(0), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b6.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, approveCall)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b8 := nextBlockV3(b7, blockChain, 0, vmTx)
	if err := connectBlock(b7, b8, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b8, b8, vmParam, nil)
	stateDB, err = state.New(&b8.Header.RootHash, &b8.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b7 -> b8 passed, now tail height: %d", blockChain.LongestChainHeight)
	// transferFrom returns
	// return 0000776f6c667265766f206e6f697463617274627573203a6874614d65666153

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b8 -> b9
	// increaseAllowance miner to spend user 20000
	vmValue, gasLimit = uint64(0), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b8.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, increaseAllowanceCall)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b9 := nextBlockV3(b8, blockChain, 0, vmTx)
	if err := connectBlock(b8, b9, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b9, b9, vmParam, nil)
	stateDB, err = state.New(&b9.Header.RootHash, &b9.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b8 -> b9 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b9 -> b10
	// allowance user miner
	vmValue, gasLimit = uint64(0), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b9.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, allowanceCall)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b10 := nextBlockV3(b9, blockChain, 0, vmTx)

	if err := connectBlock(b9, b10, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b10, b10, vmParam, nil)
	stateDB, err = state.New(&b10.Header.RootHash, &b10.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b9 -> b10 passed, now tail height: %d", blockChain.LongestChainHeight)
	// allowance user miner, return "60ea000000000000000000000000000000000000000000000000000000000000"
	// 60000

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b10 -> b11
	// transferFrom user to userA 50000 by miner, successful contract execution
	vmValue, gasLimit = uint64(0), uint64(40000)
	vmParam = &testContractParam{
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	minerNonce = stateDB.GetNonce(*minerAddr.Hash160()) + 1 // note: coinbase tx has already add 1.
	contractVout, err = txlogic.MakeContractCallVout(minerAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, minerNonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b7.Txs[1].TxHash()
	minerChangeValue = b7.Txs[1].Vout[1].Value - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(minerAddr.Hash160(), minerChangeValue)).
		WithData(types.ContractDataType, transferFromCall)
	txlogic.SignTx(vmTx, privKeyMiner, pubKeyMiner)
	b11 := nextBlockV3(b10, blockChain, 0, vmTx)

	if err := connectBlock(b10, b11, blockChain); err != nil {
		t.Fatal(err)
	}
	userBalance += b11.Header.GasUsed
	minerBalance -= b11.Header.GasUsed
	contractBlockHandle(t, blockChain, b11, b11, vmParam, nil)
	stateDB, err = state.New(&b11.Header.RootHash, &b11.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("miner nonce: %d", stateDB.GetNonce(*minerAddr.Hash160()))
	t.Logf("b10 -> b11 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b11 -> b12
	// make balances user call contract tx
	vmValue, gasLimit = uint64(0), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b10.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, balanceOfUserCall)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b12 := nextBlockV3(b11, blockChain, 0, vmTx)

	if err := connectBlock(b11, b12, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b12, b12, vmParam, nil)
	stateDB, err = state.New(&b12.Header.RootHash, &b12.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b11 -> b12 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 30c885d4e8000000000000000000000000000000000000000000000000000000
	// 0xe8d485c830 = 999997950000,  check okay
	userBal, ok = new(big.Int).SetString("e8d485c830", 16)
	if !ok {
		t.Fatal("parse user balance failed")
	}
	t.Logf("call balance of user return value: %d", userBal)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b12 -> b13
	// make balances userA call contract tx
	vmValue, gasLimit = uint64(0), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasLimit, 0, 0, contractAddr,
	}
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b12.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, balanceOfReceiverCall)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b13 := nextBlockV3(b12, blockChain, 0, vmTx)

	if err := connectBlock(b12, b13, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b13, b13, vmParam, nil)
	stateDB, err = state.New(&b13.Header.RootHash, &b13.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b12 -> b13 passed, now tail height: %d", blockChain.LongestChainHeight)
}

func TestCallBetweenContracts(t *testing.T) {
	// token
	tokenBinFile, tokenAbiFile := testContractDir+"token.bin", testContractDir+"token.abi"
	code, _ := ioutil.ReadFile(tokenBinFile)
	tokenCode, _ := hex.DecodeString(string(bytes.TrimSpace(code)))
	tokenAbi, _ := ReadAbi(tokenAbiFile)
	transferCall := func(addr *types.AddressHash, amount uint64) []byte {
		input, _ := tokenAbi.Pack("transfer", *addr, big.NewInt(int64(amount)))
		return input
	}
	// bank
	bankBinFile, bankAbiFile := testContractDir+"bank.bin", testContractDir+"bank.abi"
	code, _ = ioutil.ReadFile(bankBinFile)
	bankCode, _ := hex.DecodeString(string(bytes.TrimSpace(code)))
	bankAbi, _ := ReadAbi(bankAbiFile)
	rechargeCall, _ := bankAbi.Pack("recharge")
	t.Logf("user address: %x", userAddr.Hash160()[:])

	// deploy token contract
	blockChain := NewTestBlockChain()
	b2 := genTestChain(t, blockChain)
	vmValue, gasLimit := uint64(0), uint64(800000)
	nonce := uint64(1)
	contractVout, _ := txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, nonce)
	prevHash, _ := b2.Txs[2].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx1 := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, tokenCode)
	txlogic.SignTx(vmTx1, privKey, pubKey)
	tokenAddr, _ := types.MakeContractAddress(userAddr, nonce)
	t.Logf("token contract address: %x", tokenAddr.Hash160()[:])
	// deploy bank contract
	vmValue, gasLimit = 80000, 200000
	bankCode = append(bankCode, types.NormalizeAddressHash(tokenAddr.Hash160())[:]...)
	nonce++
	contractVout, _ = txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, nonce)
	prevHash, _ = vmTx1.TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx2 := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, bankCode)
	txlogic.SignTx(vmTx2, privKey, pubKey)
	bankAddr, _ := types.MakeContractAddress(userAddr, nonce)
	bankBalance := vmValue
	userBalance -= vmValue
	t.Logf("bank contract address: %x", bankAddr.Hash160()[:])
	// call token.transfer api
	vmValue, gasLimit = 0, 40000
	nonce++
	prevHash, _ = vmTx2.TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	input := transferCall(bankAddr.Hash160(), changeValue)
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(),
		tokenAddr.Hash160(), vmValue, gasLimit, nonce)
	vmTx3 := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, input)
	txlogic.SignTx(vmTx3, privKey, pubKey)
	// call bank.recharge api
	vmValue, gasLimit = 20000, 30000
	nonce++
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(),
		bankAddr.Hash160(), vmValue, gasLimit, nonce)
	prevHash, _ = vmTx3.TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx4 := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, rechargeCall)
	txlogic.SignTx(vmTx4, privKey, pubKey)
	tokenBalance := vmValue
	userBalance -= vmValue

	// bring them on chain
	b3 := nextBlockV3(b2, blockChain, 0, vmTx1, vmTx2, vmTx3, vmTx4)
	if err := connectBlock(b2, b3, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b3, nil, 3, b3)
	t.Logf("b3 block hash: %s", b3.BlockHash())
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// check balance
	checkTestAddrBalance(t, blockChain, tokenAddr, tokenBalance)
	checkTestAddrBalance(t, blockChain, bankAddr, bankBalance)
	userBalance -= b3.Header.GasUsed
	checkTestAddrBalance(t, blockChain, userAddr, userBalance)

	// check receipt
	vmTx4Hash, _ := vmTx4.TxHash()
	rc, _, err := blockChain.GetTxReceipt(vmTx4Hash)
	if err != nil {
		t.Fatal(err)
	}
	if *vmTx4Hash != rc.TxHash {
		t.Fatalf("receipt hash mismatch, want: %s, got: %s", vmTx4Hash, rc.TxHash)
	}
}

func TestNewContractInContract(t *testing.T) {
	// code and abi
	tokenCreatorBinFile := testContractDir + "token_creator.bin"
	tokenCreatorAbiFile := testContractDir + "token_creator.abi"
	//tokenCreatorOwnedAbiFile := testContractDir + "token_creator_owned.abi"
	//tokenCreatorTagAbiFile := testContractDir + "token_creator_tag.abi"
	code, _ := ioutil.ReadFile(tokenCreatorBinFile)
	tokenCreatorCode, _ := hex.DecodeString(string(bytes.TrimSpace(code)))
	tokenCreatorAbi, _ := ReadAbi(tokenCreatorAbiFile)
	//tokenCreatorOwnedAbi, _ := ReadAbi(tokenCreatorOwnedAbiFile)
	//tokenCreatorTagAbi, _ := ReadAbi(tokenCreatorTagAbiFile)
	createTokenCall := func(name string) []byte {
		input, _ := tokenCreatorAbi.Pack("createToken", name)
		return input
	}
	//ownedTokenGetTagCall, _ := tokenAbi.Pack("getTag")
	//tagGetSymCall, _ := tokenAbi.Pack("getSym")

	// deploy contract
	blockChain := NewTestBlockChain()
	b2 := genTestChain(t, blockChain)
	vmValue, gasLimit := uint64(2000000), uint64(1000000)
	nonce := uint64(1)
	contractVout, _ := txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, nonce)
	prevHash, _ := b2.Txs[2].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, tokenCreatorCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	tokenCreatorAddr, _ := types.MakeContractAddress(userAddr, nonce)
	t.Logf("token creator contract address: %x", tokenCreatorAddr.Hash160()[:])
	//
	b3 := nextBlockV3(b2, blockChain, 0, vmTx)
	if err := connectBlock(b2, b3, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b3, nil, 3, b3)
	t.Logf("b3 block hash: %s", b3.BlockHash())
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)
	// first send createToken call to create token owned contract and tag contract
	// then check token owned contract balance whether it equals to 1000
	// and check tag contract whether it's sym is "ABC"
	nonce++
	vmValue, gasLimit = uint64(0), uint64(600000)
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(),
		tokenCreatorAddr.Hash160(), vmValue, gasLimit, nonce)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx1 := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.Hash160(), changeValue)).
		WithData(types.ContractDataType, createTokenCall("test owned token"))
	txlogic.SignTx(vmTx1, privKey, pubKey)
	//
	b4 := nextBlockV3(b3, blockChain, 0, vmTx1)
	if err := connectBlock(b3, b4, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b4, nil, 4, b4)
	t.Logf("b4 block hash: %s", b4.BlockHash())
	t.Logf("b3 -> b4 passed, now tail height: %d", blockChain.LongestChainHeight)
	//
}
