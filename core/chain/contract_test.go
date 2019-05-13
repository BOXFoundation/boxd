// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package chain
package chain

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/BOXFoundation/boxd/core/state"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/facebookgo/ensure"
)

const (
	testERC20Contract = "60806040523480156200001157600080fd5b50604080519081016040528" +
		"0600b81526020017f53696d706c65546f6b656e0000000000000000000000000000000000000000" +
		"008152506040805190810160405280600381526020017f53494d000000000000000000000000000" +
		"0000000000000000000000000000000815250601282600390805190602001906200009892919062" +
		"000365565b508160049080519060200190620000b192919062000365565b5080600560006101000" +
		"a81548160ff021916908360ff160217905550505050620000f633601260ff16600a0a6127100262" +
		"0000fc640100000000026401000000009004565b62000414565b600073fffffffffffffffffffff" +
		"fffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16141515156200" +
		"01a2576040517f08c379a0000000000000000000000000000000000000000000000000000000008" +
		"15260040180806020018281038252601f8152602001807f45524332303a206d696e7420746f2074" +
		"6865207a65726f20616464726573730081525060200191505060405180910390fd5b620001c7816" +
		"00254620002da6401000000000262000fc3179091906401000000009004565b6002819055506200" +
		"022e816000808573ffffffffffffffffffffffffffffffffffffffff1673fffffffffffffffffff" +
		"fffffffffffffffffffff16815260200190815260200160002054620002da640100000000026200" +
		"0fc3179091906401000000009004565b6000808473fffffffffffffffffffffffffffffffffffff" +
		"fff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081" +
		"9055508173ffffffffffffffffffffffffffffffffffffffff16600073fffffffffffffffffffff" +
		"fffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4d" +
		"f523b3ef836040518082815260200191505060405180910390a35050565b6000808284019050838" +
		"1101515156200035b576040517f08c379a000000000000000000000000000000000000000000000" +
		"000000000000815260040180806020018281038252601b8152602001807f536166654d6174683a2" +
		"06164646974696f6e206f766572666c6f77000000000081525060200191505060405180910390fd" +
		"5b8091505092915050565b828054600181600116156101000203166002900490600052602060002" +
		"090601f016020900481019282601f10620003a857805160ff1916838001178555620003d9565b82" +
		"800160010185558215620003d9579182015b82811115620003d8578251825591602001919060010" +
		"190620003bb565b5b509050620003e89190620003ec565b5090565b6200041191905b8082111562" +
		"00040d576000816000905550600101620003f3565b5090565b90565b61107980620004246000396" +
		"000f3fe6080604052600436106100c5576000357c01000000000000000000000000000000000000" +
		"00000000000000000000900463ffffffff16806306fdde03146100ca578063095ea7b31461015a5" +
		"7806318160ddd146101cd57806323b872dd146101f85780632e0f26251461028b5780632ff2e9dc" +
		"146102bc578063313ce567146102e7578063395093511461031857806370a082311461038b57806" +
		"395d89b41146103f0578063a457c2d714610480578063a9059cbb146104f3578063dd62ed3e1461" +
		"0566575b600080fd5b3480156100d657600080fd5b506100df6105eb565b6040518080602001828" +
		"103825283818151815260200191508051906020019080838360005b8381101561011f5780820151" +
		"81840152602081019050610104565b50505050905090810190601f16801561014c5780820380516" +
		"001836020036101000a031916815260200191505b509250505060405180910390f35b3480156101" +
		"6657600080fd5b506101b36004803603604081101561017d57600080fd5b81019080803573fffff" +
		"fffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050" +
		"61068d565b604051808215151515815260200191505060405180910390f35b3480156101d957600" +
		"080fd5b506101e26106a4565b6040518082815260200191505060405180910390f35b3480156102" +
		"0457600080fd5b506102716004803603606081101561021b57600080fd5b81019080803573fffff" +
		"fffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffff" +
		"ffffffffffffffffffff169060200190929190803590602001909291905050506106ae565b60405" +
		"1808215151515815260200191505060405180910390f35b34801561029757600080fd5b506102a0" +
		"61075f565b604051808260ff1660ff16815260200191505060405180910390f35b3480156102c85" +
		"7600080fd5b506102d1610764565b6040518082815260200191505060405180910390f35b348015" +
		"6102f357600080fd5b506102fc610773565b604051808260ff1660ff16815260200191505060405" +
		"180910390f35b34801561032457600080fd5b506103716004803603604081101561033b57600080" +
		"fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803" +
		"5906020019092919050505061078a565b6040518082151515158152602001915050604051809103" +
		"90f35b34801561039757600080fd5b506103da600480360360208110156103ae57600080fd5b810" +
		"19080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506108" +
		"2f565b6040518082815260200191505060405180910390f35b3480156103fc57600080fd5b50610" +
		"405610877565b604051808060200182810382528381815181526020019150805190602001908083" +
		"8360005b8381101561044557808201518184015260208101905061042a565b50505050905090810" +
		"190601f1680156104725780820380516001836020036101000a031916815260200191505b509250" +
		"505060405180910390f35b34801561048c57600080fd5b506104d9600480360360408110156104a" +
		"357600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190" +
		"92919080359060200190929190505050610919565b6040518082151515158152602001915050604" +
		"05180910390f35b3480156104ff57600080fd5b5061054c60048036036040811015610516576000" +
		"80fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908" +
		"03590602001909291905050506109be565b60405180821515151581526020019150506040518091" +
		"0390f35b34801561057257600080fd5b506105d56004803603604081101561058957600080fd5b8" +
		"1019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ff" +
		"ffffffffffffffffffffffffffffffffffffff1690602001909291905050506109d5565b6040518" +
		"082815260200191505060405180910390f35b606060038054600181600116156101000203166002" +
		"900480601f016020809104026020016040519081016040528092919081815260200182805460018" +
		"1600116156101000203166002900480156106835780601f10610658576101008083540402835291" +
		"60200191610683565b820191906000526020600020905b815481529060010190602001808311610" +
		"66657829003601f168201915b5050505050905090565b600061069a338484610a5c565b60019050" +
		"92915050565b6000600254905090565b60006106bb848484610cdd565b610754843361074f85600" +
		"160008a73ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffff" +
		"ffffffffffffff16815260200190815260200160002060003373fffffffffffffffffffffffffff" +
		"fffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020" +
		"0160002054610f3890919063ffffffff16565b610a5c565b600190509392505050565b601281565" +
		"b601260ff16600a0a6127100281565b6000600560009054906101000a900460ff16905090565b60" +
		"00610825338461082085600160003373ffffffffffffffffffffffffffffffffffffffff1673fff" +
		"fffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008973ffff" +
		"ffffffffffffffffffffffffffffffffffff1673fffffffffffffffffffffffffffffffffffffff" +
		"f16815260200190815260200160002054610fc390919063ffffffff16565b610a5c565b60019050" +
		"92915050565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673fffffffff" +
		"fffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b60" +
		"6060048054600181600116156101000203166002900480601f01602080910402602001604051908" +
		"101604052809291908181526020018280546001816001161561010002031660029004801561090f" +
		"5780601f106108e45761010080835404028352916020019161090f565b820191906000526020600" +
		"020905b8154815290600101906020018083116108f257829003601f168201915b50505050509050" +
		"90565b60006109b433846109af85600160003373fffffffffffffffffffffffffffffffffffffff" +
		"f1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000" +
		"8973ffffffffffffffffffffffffffffffffffffffff1673fffffffffffffffffffffffffffffff" +
		"fffffffff16815260200190815260200160002054610f3890919063ffffffff16565b610a5c565b" +
		"6001905092915050565b60006109cb338484610cdd565b6001905092915050565b6000600160008" +
		"473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffff" +
		"ffffffff16815260200190815260200160002060008373fffffffffffffffffffffffffffffffff" +
		"fffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000" +
		"2054905092915050565b600073ffffffffffffffffffffffffffffffffffffffff168373fffffff" +
		"fffffffffffffffffffffffffffffffff1614151515610b27576040517f08c379a0000000000000" +
		"0000000000000000000000000000000000000000000081526004018080602001828103825260248" +
		"152602001807f45524332303a20617070726f76652066726f6d20746865207a65726f2061646481" +
		"526020017f726573730000000000000000000000000000000000000000000000000000000081525" +
		"060400191505060405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff16" +
		"8273ffffffffffffffffffffffffffffffffffffffff1614151515610bf2576040517f08c379a00" +
		"0000000000000000000000000000000000000000000000000000000815260040180806020018281" +
		"03825260228152602001807f45524332303a20617070726f766520746f20746865207a65726f206" +
		"16464726581526020017f7373000000000000000000000000000000000000000000000000000000" +
		"00000081525060400191505060405180910390fd5b80600160008573fffffffffffffffffffffff" +
		"fffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152" +
		"60200160002060008473ffffffffffffffffffffffffffffffffffffffff1673fffffffffffffff" +
		"fffffffffffffffffffffffff168152602001908152602001600020819055508173ffffffffffff" +
		"ffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff167f8" +
		"c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b9258360405180828152" +
		"60200191505060405180910390a3505050565b600073fffffffffffffffffffffffffffffffffff" +
		"fffff168273ffffffffffffffffffffffffffffffffffffffff1614151515610da8576040517f08" +
		"c379a00000000000000000000000000000000000000000000000000000000081526004018080602" +
		"001828103825260238152602001807f45524332303a207472616e7366657220746f20746865207a" +
		"65726f206164647281526020017f657373000000000000000000000000000000000000000000000" +
		"000000000000081525060400191505060405180910390fd5b610df9816000808673ffffffffffff" +
		"ffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526" +
		"0200190815260200160002054610f3890919063ffffffff16565b6000808573ffffffffffffffff" +
		"ffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200" +
		"190815260200160002081905550610e8c816000808573ffffffffffffffffffffffffffffffffff" +
		"ffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002" +
		"054610fc390919063ffffffff16565b6000808473ffffffffffffffffffffffffffffffffffffff" +
		"ff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819" +
		"055508173ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffff" +
		"ffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df52" +
		"3b3ef836040518082815260200191505060405180910390a3505050565b6000828211151515610f" +
		"b2576040517f08c379a000000000000000000000000000000000000000000000000000000000815" +
		"260040180806020018281038252601e8152602001807f536166654d6174683a2073756274726163" +
		"74696f6e206f766572666c6f77000081525060200191505060405180910390fd5b6000828403905" +
		"08091505092915050565b6000808284019050838110151515611043576040517f08c379a0000000" +
		"0000000000000000000000000000000000000000000000000081526004018080602001828103825" +
		"2601b8152602001807f536166654d6174683a206164646974696f6e206f766572666c6f77000000" +
		"000081525060200191505060405180910390fd5b809150509291505056fea165627a7a72305820e" +
		"d358c743c065288f0823a763466b98875b047133a773c461b8053ed09ce79fa0029"

	testERC20Abi = `[
	{
		"constant": true,
		"inputs": [],
		"name": "name",
		"outputs": [ { "name": "", "type": "string" } ],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}, {
		"constant": false,
		"inputs": [ { "name": "spender", "type": "address" }, { "name": "value", "type": "uint256" } ],
		"name": "approve",
		"outputs": [ { "name": "", "type": "bool" } ],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}, {
		"constant": true,
		"inputs": [],
		"name": "totalSupply",
		"outputs": [ { "name": "", "type": "uint256" } ],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}, {
		"constant": false,
		"inputs": [ { "name": "from", "type": "address" },
			{ "name": "to", "type": "address" }, { "name": "value", "type": "uint256" } ],
		"name": "transferFrom",
		"outputs": [ { "name": "", "type": "bool" } ],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}, {
		"constant": true,
		"inputs": [],
		"name": "DECIMALS",
		"outputs": [ { "name": "", "type": "uint8" } ],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}, {
		"constant": true,
		"inputs": [],
		"name": "INITIAL_SUPPLY",
		"outputs": [ { "name": "", "type": "uint256" } ],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}, {
		"constant": true,
		"inputs": [],
		"name": "decimals",
		"outputs": [ { "name": "", "type": "uint8" } ],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}, {
		"constant": false,
		"inputs": [ { "name": "spender", "type": "address" }, { "name": "addedValue", "type": "uint256" } ],
		"name": "increaseAllowance",
		"outputs": [ { "name": "", "type": "bool" } ],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}, {
		"constant": true,
		"inputs": [ { "name": "owner", "type": "address" } ],
		"name": "balanceOf",
		"outputs": [ { "name": "", "type": "uint256" } ],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}, {
		"constant": true,
		"inputs": [],
		"name": "symbol",
		"outputs": [ { "name": "", "type": "string" } ],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}, {
		"constant": false,
		"inputs": [
			{ "name": "spender", "type": "address" }, { "name": "subtractedValue", "type": "uint256" } ],
		"name": "decreaseAllowance",
		"outputs": [ { "name": "", "type": "bool" } ],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}, {
		"constant": false,
		"inputs": [ { "name": "to", "type": "address" }, { "name": "value", "type": "uint256" } ],
		"name": "transfer",
		"outputs": [ { "name": "", "type": "bool" } ],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}, {
		"constant": true,
		"inputs": [ { "name": "owner", "type": "address" }, { "name": "spender", "type": "address" } ],
		"name": "allowance",
		"outputs": [ { "name": "", "type": "uint256" } ],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}, {
		"inputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "constructor"
	}, {
		"anonymous": false,
		"inputs": [ { "indexed": true, "name": "from", "type": "address" },
			{ "indexed": true, "name": "to", "type": "address" },
			{ "indexed": false, "name": "value", "type": "uint256" } ],
		"name": "Transfer",
		"type": "event"
	}, {
		"anonymous": false,
		"inputs": [ { "indexed": true, "name": "owner", "type": "address" },
			{ "indexed": true, "name": "spender", "type": "address" },
			{ "indexed": false, "name": "value", "type": "uint256" } ],
		"name": "Approval",
		"type": "event"
	}
]`
)

func TestERC20Contract(t *testing.T) {

	var transferCall, balanceOfUserCall, balanceOfReceiverCall string
	// balances
	receiver, err := types.NewContractAddress("b5WYphc4yBPH18gyFthS1bHyRcEvM6xANuT")
	if err != nil {
		t.Fatal(err)
	}
	func() {
		abiObj, err := abi.JSON(strings.NewReader(testERC20Abi))
		if err != nil {
			t.Fatal(err)
		}
		// transfer 2000000
		input, err := abiObj.Pack("transfer", *receiver.Hash160(), big.NewInt(2000000))
		if err != nil {
			t.Fatal(err)
		}
		transferCall = hex.EncodeToString(input)
		t.Logf("transfer 2000000: %s", transferCall)
		// balances user addr
		input, err = abiObj.Pack("balanceOf", *userAddr.Hash160())
		if err != nil {
			t.Fatal(err)
		}
		balanceOfUserCall = hex.EncodeToString(input)
		t.Logf("balanceUser: %s", balanceOfUserCall)
		// balances test Addr
		input, err = abiObj.Pack("balanceOf", receiver.Hash160())
		if err != nil {
			t.Fatal(err)
		}
		balanceOfReceiverCall = hex.EncodeToString(input)
		t.Logf("balance of %s: %s", receiver, balanceOfReceiverCall)
	}()

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
	gasUsed, vmValue, gasPrice, gasLimit := uint64(982928), uint64(0), uint64(2), uint64(2000000)
	vmParam := &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, vmValue, 0, nil,
	}
	byteCode, _ := hex.DecodeString(testERC20Contract)
	contractVout, err := txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	stateDB, err := state.New(&b2.Header.RootHash, &b2.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce := stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	//
	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce+1)
	rootHashStr := "971775d17af23b0e028510aa8864cf3192dbed70fa9a3a4d96c3d17df45f3d65"
	utxoRootHashStr := "f73785065253033248f0bdea11cc96f56ccae34bf4c52cd0c922c80da39742f1"
	b3 := contractBlockHandle(t, blockChain, vmTx, b2, rootHashStr, utxoRootHashStr,
		vmParam, nil, gasRefundTx)
	//refundValue := vmParam.gasPrice * (vmParam.gasLimit - vmParam.gasUsed)
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b4
	// make transfer 2000000 call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(30808), uint64(0), uint64(6), uint64(40000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(transferCall)
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	stateDB, err = state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce = stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce+1)
	rootHashStr = "9a839ede941c0e50919bd07a2c909c61690168a57676cc6acea20fd255a794ab"
	utxoRootHashStr = "f73785065253033248f0bdea11cc96f56ccae34bf4c52cd0c922c80da39742f1"
	b4 := contractBlockHandle(t, blockChain, vmTx, b3, rootHashStr, utxoRootHashStr,
		vmParam, nil, gasRefundTx)
	t.Logf("b3 -> b4 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b4 -> b5
	// make balances user call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(3017), uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfUserCall)
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b4.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	stateDB, err = state.New(&b4.Header.RootHash, &b4.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce = stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce+1)
	rootHashStr = "8049bd5a97924dea1bb9aa3a05aa5891f3f039f2804d0e5f2d23d16f3536cf15"
	utxoRootHashStr = "f73785065253033248f0bdea11cc96f56ccae34bf4c52cd0c922c80da39742f1"
	b5 := contractBlockHandle(t, blockChain, vmTx, b4, rootHashStr, utxoRootHashStr,
		vmParam, nil, gasRefundTx)
	t.Logf("b4 -> b5 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 807b21b2bac9e0191e0200000000000000000000000000000000000000000000
	// 0x021319e0c9bab2217b80 = , check okay

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make balances receiver call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(3017), uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfReceiverCall)
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	nonce = stateDB.GetNonce(*userAddr.Hash160())
	t.Logf("user nonce: %d", nonce)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce+1)
	gasRefundTxHash, _ := gasRefundTx.TxHash()
	t.Logf("refund tx: %s", gasRefundTxHash)
	rootHashStr = "9e845be1773451ed14ad0b988135bae657bb7c08cc2dd4ed7a1bfd0e5f93e929"
	utxoRootHashStr = "f73785065253033248f0bdea11cc96f56ccae34bf4c52cd0c922c80da39742f1"
	contractBlockHandle(t, blockChain, vmTx, b5, rootHashStr, utxoRootHashStr,
		vmParam, nil, gasRefundTx)
	t.Logf("b5 -> b6 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 0x1e8480 = 2000000, check okay
}
