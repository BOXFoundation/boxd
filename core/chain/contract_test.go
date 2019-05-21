// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package chain
package chain

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/facebookgo/ensure"
)

const (
	testERC20Contract = "60806040523480156200001157600080fd5b5060408051908101604052806" +
		"00b81526020017f53696d706c65546f6b656e0000000000000000000000000000000000000000008" +
		"152506040805190810160405280600381526020017f53494d0000000000000000000000000000000" +
		"00000000000000000000000000081525060088260039080519060200190620000989291906200036" +
		"5565b508160049080519060200190620000b192919062000365565b5080600560006101000a81548" +
		"160ff021916908360ff160217905550505050620000f633600860ff16600a0a61271002620000fc6" +
		"40100000000026401000000009004565b62000414565b600073fffffffffffffffffffffffffffff" +
		"fffffffffff168273ffffffffffffffffffffffffffffffffffffffff1614151515620001a257604" +
		"0517f08c379a00000000000000000000000000000000000000000000000000000000081526004018" +
		"0806020018281038252601f8152602001807f45524332303a206d696e7420746f20746865207a657" +
		"26f20616464726573730081525060200191505060405180910390fd5b620001c781600254620002d" +
		"a6401000000000262001002179091906401000000009004565b6002819055506200022e816000808" +
		"573ffffffffffffffffffffffffffffffffffffffff1673fffffffffffffffffffffffffffffffff" +
		"fffffff16815260200190815260200160002054620002da640100000000026200100217909190640" +
		"1000000009004565b6000808473ffffffffffffffffffffffffffffffffffffffff1673fffffffff" +
		"fffffffffffffffffffffffffffffff168152602001908152602001600020819055508173fffffff" +
		"fffffffffffffffffffffffffffffffff16600073fffffffffffffffffffffffffffffffffffffff" +
		"f167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef83604051808" +
		"2815260200191505060405180910390a35050565b60008082840190508381101515156200035b576" +
		"040517f08c379a000000000000000000000000000000000000000000000000000000000815260040" +
		"180806020018281038252601b8152602001807f536166654d6174683a206164646974696f6e206f7" +
		"66572666c6f77000000000081525060200191505060405180910390fd5b8091505092915050565b8" +
		"28054600181600116156101000203166002900490600052602060002090601f01602090048101928" +
		"2601f10620003a857805160ff1916838001178555620003d9565b82800160010185558215620003d" +
		"9579182015b82811115620003d8578251825591602001919060010190620003bb565b5b509050620" +
		"003e89190620003ec565b5090565b6200041191905b808211156200040d576000816000905550600" +
		"101620003f3565b5090565b90565b6110b880620004246000396000f3fe608060405260043610610" +
		"0c5576000357c0100000000000000000000000000000000000000000000000000000000900463fff" +
		"fffff16806306fdde03146100ca578063095ea7b31461015a57806318160ddd146101cd57806323b" +
		"872dd146101f85780632e0f26251461028b5780632ff2e9dc146102bc578063313ce567146102e75" +
		"78063395093511461031857806370a082311461038b57806395d89b41146103f0578063a457c2d71" +
		"4610480578063a9059cbb146104f3578063dd62ed3e14610566575b600080fd5b3480156100d6576" +
		"00080fd5b506100df6105eb565b60405180806020018281038252838181518152602001915080519" +
		"06020019080838360005b8381101561011f578082015181840152602081019050610104565b50505" +
		"050905090810190601f16801561014c5780820380516001836020036101000a03191681526020019" +
		"1505b509250505060405180910390f35b34801561016657600080fd5b506101b3600480360360408" +
		"1101561017d57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169" +
		"0602001909291908035906020019092919050505061068d565b60405180821515151581526020019" +
		"1505060405180910390f35b3480156101d957600080fd5b506101e26106a4565b604051808281526" +
		"0200191505060405180910390f35b34801561020457600080fd5b506102716004803603606081101" +
		"561021b57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602" +
		"00190929190803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359" +
		"0602001909291905050506106ae565b604051808215151515815260200191505060405180910390f" +
		"35b34801561029757600080fd5b506102a061075f565b604051808260ff1660ff168152602001915" +
		"05060405180910390f35b3480156102c857600080fd5b506102d1610764565b60405180828152602" +
		"00191505060405180910390f35b3480156102f357600080fd5b506102fc610773565b60405180826" +
		"0ff1660ff16815260200191505060405180910390f35b34801561032457600080fd5b50610371600" +
		"4803603604081101561033b57600080fd5b81019080803573fffffffffffffffffffffffffffffff" +
		"fffffffff1690602001909291908035906020019092919050505061078a565b60405180821515151" +
		"5815260200191505060405180910390f35b34801561039757600080fd5b506103da6004803603602" +
		"08110156103ae57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1" +
		"6906020019092919050505061082f565b6040518082815260200191505060405180910390f35b348" +
		"0156103fc57600080fd5b50610405610877565b60405180806020018281038252838181518152602" +
		"00191508051906020019080838360005b83811015610445578082015181840152602081019050610" +
		"42a565b50505050905090810190601f1680156104725780820380516001836020036101000a03191" +
		"6815260200191505b509250505060405180910390f35b34801561048c57600080fd5b506104d9600" +
		"480360360408110156104a357600080fd5b81019080803573fffffffffffffffffffffffffffffff" +
		"fffffffff16906020019092919080359060200190929190505050610919565b60405180821515151" +
		"5815260200191505060405180910390f35b3480156104ff57600080fd5b5061054c6004803603604" +
		"081101561051657600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1" +
		"69060200190929190803590602001909291905050506109be565b604051808215151515815260200" +
		"191505060405180910390f35b34801561057257600080fd5b506105d560048036036040811015610" +
		"58957600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019" +
		"0929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610" +
		"9d5565b6040518082815260200191505060405180910390f35b60606003805460018160011615610" +
		"1000203166002900480601f016020809104026020016040519081016040528092919081815260200" +
		"1828054600181600116156101000203166002900480156106835780601f106106585761010080835" +
		"4040283529160200191610683565b820191906000526020600020905b81548152906001019060200" +
		"180831161066657829003601f168201915b5050505050905090565b600061069a338484610a5c565" +
		"b6001905092915050565b6000600254905090565b60006106bb848484610cdd565b6107548433610" +
		"74f85600160008a73ffffffffffffffffffffffffffffffffffffffff1673fffffffffffffffffff" +
		"fffffffffffffffffffff16815260200190815260200160002060003373fffffffffffffffffffff" +
		"fffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815" +
		"260200160002054610f7790919063ffffffff16565b610a5c565b600190509392505050565b60088" +
		"1565b600860ff16600a0a6127100281565b6000600560009054906101000a900460ff16905090565" +
		"b6000610825338461082085600160003373ffffffffffffffffffffffffffffffffffffffff1673f" +
		"fffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008973fff" +
		"fffffffffffffffffffffffffffffffffffff1673fffffffffffffffffffffffffffffffffffffff" +
		"f1681526020019081526020016000205461100290919063ffffffff16565b610a5c565b600190509" +
		"2915050565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673fffffffffff" +
		"fffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b60606" +
		"0048054600181600116156101000203166002900480601f016020809104026020016040519081016" +
		"04052809291908181526020018280546001816001161561010002031660029004801561090f57806" +
		"01f106108e45761010080835404028352916020019161090f565b820191906000526020600020905" +
		"b8154815290600101906020018083116108f257829003601f168201915b5050505050905090565b6" +
		"0006109b433846109af85600160003373ffffffffffffffffffffffffffffffffffffffff1673fff" +
		"fffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008973fffff" +
		"fffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1" +
		"6815260200190815260200160002054610f7790919063ffffffff16565b610a5c565b60019050929" +
		"15050565b60006109cb338484610cdd565b6001905092915050565b6000600160008473fffffffff" +
		"fffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815" +
		"260200190815260200160002060008373ffffffffffffffffffffffffffffffffffffffff1673fff" +
		"fffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490509291505" +
		"0565b600073ffffffffffffffffffffffffffffffffffffffff168373fffffffffffffffffffffff" +
		"fffffffffffffffff1614151515610b27576040517f08c379a000000000000000000000000000000" +
		"00000000000000000000000000081526004018080602001828103825260248152602001807f45524" +
		"332303a20617070726f76652066726f6d20746865207a65726f2061646481526020017f726573730" +
		"00000000000000000000000000000000000000000000000000000008152506040019150506040518" +
		"0910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273fffffffffffffffff" +
		"fffffffffffffffffffffff1614151515610bf2576040517f08c379a000000000000000000000000" +
		"00000000000000000000000000000000081526004018080602001828103825260228152602001807" +
		"f45524332303a20617070726f766520746f20746865207a65726f20616464726581526020017f737" +
		"300000000000000000000000000000000000000000000000000000000000081525060400191505060" +
		"405180910390fd5b80600160008573ffffffffffffffffffffffffffffffffffffffff1673ffffff" +
		"ffffffffffffffffffffffffffffffffff16815260200190815260200160002060008473ffffffff" +
		"ffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681" +
		"52602001908152602001600020819055508173ffffffffffffffffffffffffffffffffffffffff16" +
		"8373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3" +
		"dd0314c0f7b2291e5b200ac8c7c3b925836040518082815260200191505060405180910390a35050" +
		"50565b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffff" +
		"ffffffffffffffffff1614151515610da8576040517f08c379a00000000000000000000000000000" +
		"000000000000000000000000000081526004018080602001828103825260238152602001807f4552" +
		"4332303a207472616e7366657220746f20746865207a65726f206164647281526020017f65737300" +
		"00000000000000000000000000000000000000000000000000000000815250604001915050604051" +
		"80910390fd5b7fa9a26360ded17bbe6528a5ec42df34cc22964927204d7a69575e12c6839d426c81" +
		"82604051808381526020018281526020019250505060405180910390a1610e38816000808673ffff" +
		"ffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff" +
		"16815260200190815260200160002054610f7790919063ffffffff16565b6000808573ffffffffff" +
		"ffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152" +
		"60200190815260200160002081905550610ecb816000808573ffffffffffffffffffffffffffffff" +
		"ffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160" +
		"00205461100290919063ffffffff16565b6000808473ffffffffffffffffffffffffffffffffffff" +
		"ffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081" +
		"9055508173ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffff" +
		"ffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523" +
		"b3ef836040518082815260200191505060405180910390a3505050565b6000828211151515610ff1" +
		"576040517f08c379a000000000000000000000000000000000000000000000000000000000815260" +
		"040180806020018281038252601e8152602001807f536166654d6174683a20737562747261637469" +
		"6f6e206f766572666c6f77000081525060200191505060405180910390fd5b600082840390508091" +
		"505092915050565b6000808284019050838110151515611082576040517f08c379a0000000000000" +
		"00000000000000000000000000000000000000000000815260040180806020018281038252601b81" +
		"52602001807f536166654d6174683a206164646974696f6e206f766572666c6f7700000000008152" +
		"5060200191505060405180910390fd5b809150509291505056fea165627a7a7230582006ba3acd85" +
		"76f5234662cdfb3fa7d6ab075457ed18279e2402485a6a20b8ed060029"

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

	var (
		transferCall, balanceOfUserCall, balanceOfReceiverCall string
		transferFromCall, approveCall, increaseAllowanceCall   string
		allowanceCall                                          string
	)
	// balances
	userA, _ := types.NewAddress("b1afgd4BC3Y81ni3ds2YETikEkprG9Bxo98")
	//userB, _ := types.NewAddress("b1bgU3pRjrW2AXZ5DtumFJwrh69QTsErhAD")
	func() {
		abiObj, err := abi.JSON(strings.NewReader(testERC20Abi))
		if err != nil {
			t.Fatal(err)
		}
		// transfer 2000000
		input, err := abiObj.Pack("transfer", *userA.Hash160(), big.NewInt(2000000))
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
		input, err = abiObj.Pack("balanceOf", userA.Hash160())
		if err != nil {
			t.Fatal(err)
		}
		balanceOfReceiverCall = hex.EncodeToString(input)
		t.Logf("balance of %s: %s", userA, balanceOfReceiverCall)
		// transferFrom, sender miner, from userAddr, to userA
		input, err = abiObj.Pack("transferFrom", *userAddr.Hash160(), *userA.Hash160(), big.NewInt(50000))
		if err != nil {
			t.Fatal(err)
		}
		transferFromCall = hex.EncodeToString(input)
		t.Logf("transferFrom %s to %s 50000: %s", userAddr, userA, transferFromCall)
		// aprove 40000 for miner, sender userAddr, spender miner
		input, err = abiObj.Pack("approve", *minerAddr.Hash160(), big.NewInt(40000))
		if err != nil {
			t.Fatal(err)
		}
		approveCall = hex.EncodeToString(input)
		t.Logf("approve miner %s can spend %s 40000: %s", minerAddr, userAddr, approveCall)
		// increaseAllowance 20000 for miner, sender userAddr, spender miner
		input, err = abiObj.Pack("increaseAllowance", *minerAddr.Hash160(), big.NewInt(20000))
		if err != nil {
			t.Fatal(err)
		}
		increaseAllowanceCall = hex.EncodeToString(input)
		t.Logf("increase %s's allowance 20000: %s", minerAddr, increaseAllowanceCall)
		// allowance owner user for spender miner
		input, err = abiObj.Pack("allowance", *userAddr.Hash160(), *minerAddr.Hash160())
		if err != nil {
			t.Fatal(err)
		}
		allowanceCall = hex.EncodeToString(input)
		t.Logf("allowance user %s miner %s: %s", userAddr, minerAddr, allowanceCall)
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
	gasUsed, vmValue, gasPrice, gasLimit := uint64(995919), uint64(0), uint64(2), uint64(2000000)
	vmParam := &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, vmValue, 0, nil,
	}
	byteCode, _ := hex.DecodeString(testERC20Contract)
	nonce := uint64(1)
	contractVout, err := txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	t.Logf("user addr: %s", hex.EncodeToString(userAddr.Hash160()[:]))
	//
	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b3 := nextBlockWithTxs(b2, vmTx)
	b3.Header.RootHash.SetString("12af9eb263693849c4a3ac4c905e6800178a55c53fdafc48e475a8ae47019475")
	b3.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ := vmTx.TxHash()
	receipt := types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b3.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b2, b3, vmParam, nil, gasRefundTx)
	stateDB, err := state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	//refundValue := vmParam.gasPrice * (vmParam.gasLimit - vmParam.gasUsed)
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b3A
	// make balances user call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(3017), uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfUserCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b3A := nextBlockWithTxs(b3, vmTx)
	b3A.Header.RootHash.SetString("589346e63deeb5494472d63c30407e32bced300b23249b48bb2d7876bfe533b1")
	b3A.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b3A.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b3, b3A, vmParam, nil, gasRefundTx)
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
	gasUsed, vmValue, gasPrice, gasLimit = uint64(32148), uint64(0), uint64(6), uint64(40000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(transferCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3A.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b4 := nextBlockWithTxs(b3A, vmTx)
	b4.Header.RootHash.SetString("a57bc9ea5a0886f7aa8d876c906a1f081affcbdb724e2be71eff940bcc14aae4")
	b4.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b4.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b3A, b4, vmParam, nil, gasRefundTx)
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
	gasUsed, vmValue, gasPrice, gasLimit = uint64(3017), uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfUserCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b4.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b5 := nextBlockWithTxs(b4, vmTx)
	b5.Header.RootHash.SetString("e0ae25cc6a503fbd7ffed836cc3515c29a024b30b87a0985e4fbe3b5116c5c08")
	b5.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b5.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b4, b5, vmParam, nil, gasRefundTx)
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
	gasUsed, vmValue, gasPrice, gasLimit = uint64(3017), uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfReceiverCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b6 := nextBlockWithTxs(b5, vmTx)
	b6.Header.RootHash.SetString("737d10267bc52fa6770e002a3953ad8322e2947f638f07c278f955640e073b01")
	b6.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b6.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b5, b6, vmParam, nil, gasRefundTx)
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
	gasUsed, vmValue, gasPrice, gasLimit = uint64(17616), uint64(0), uint64(2), uint64(40000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(transferFromCall)
	minerNonce := uint64(1)
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, minerNonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b2.Txs[0].TxHash()
	minerChangeValue := b2.Txs[0].Vout[0].Value - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(minerAddr.String(), minerChangeValue))
	signTx(vmTx, privKeyMiner, pubKeyMiner)
	gasRefundTx = createGasRefundUtxoTx(minerAddr.Hash160(), gasPrice*(gasLimit-gasUsed), minerNonce)
	b7 := nextBlockWithTxs(b6, vmTx)
	b7.Header.RootHash.SetString("7bddc1b5a0c6d6d17bc1aa015b725a28be6ced4c3238f4724f7760b134ee3647")
	b7.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), true, gasUsed).WithTxIndex(1)
	b7.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	userBalance += gasUsed * gasPrice
	minerBalance -= gasUsed * gasPrice
	contractBlockHandle(t, blockChain, b6, b7, vmParam, nil, gasRefundTx)
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
	gasUsed, vmValue, gasPrice, gasLimit = uint64(24805), uint64(0), uint64(2), uint64(40000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(approveCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b6.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b8 := nextBlockWithTxs(b7, vmTx)
	b8.Header.RootHash.SetString("7863f5f7f50133ad28982898a25413531ae13811c565cf11d6d6a5fb29e6c169")
	b8.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b8.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b7, b8, vmParam, nil, gasRefundTx)
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
	gasUsed, vmValue, gasPrice, gasLimit = uint64(10430), uint64(0), uint64(2), uint64(40000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(increaseAllowanceCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b8.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b9 := nextBlockWithTxs(b8, vmTx)
	b9.Header.RootHash.SetString("f93c9a36cd480b3033cb985b986b0b42bdf5f64850dd0234ac36d88683c31c55")
	b9.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b9.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b8, b9, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b9.Header.RootHash, &b9.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b8 -> b9 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b9 -> b10
	// allowance user miner
	gasUsed, vmValue, gasPrice, gasLimit = uint64(3362), uint64(0), uint64(2), uint64(40000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(allowanceCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b9.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b10 := nextBlockWithTxs(b9, vmTx)
	b10.Header.RootHash.SetString("52f3c0d3ddb5486d09dafb9b72856a98a2e91a77d16c40dc63433155e65158bb")
	b10.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b10.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b9, b10, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b10.Header.RootHash, &b10.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b9 -> b10 passed, now tail height: %d", blockChain.LongestChainHeight)
	// allowance user miner, return "60ea000000000000000000000000000000000000000000000000000000000000"
	// 60000

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b10 -> b11
	// transferFrom user to userA 40000 by miner, successful contract execution
	gasUsed, vmValue, gasPrice, gasLimit = uint64(24777), uint64(0), uint64(2), uint64(40000)
	vmParam = &testContractParam{
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(transferFromCall)
	minerNonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, minerNonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b9.Txs[0].TxHash()
	minerChangeValue = b9.Txs[0].Vout[0].Value - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(minerAddr.String(), minerChangeValue))
	signTx(vmTx, privKeyMiner, pubKeyMiner)
	gasRefundTx = createGasRefundUtxoTx(minerAddr.Hash160(), gasPrice*(gasLimit-gasUsed), minerNonce)
	b11 := nextBlockWithTxs(b10, vmTx)
	b11.Header.RootHash.SetString("23aeaf0832fb690a899fc5938328eb13f2fe6def116dc8b783fa6cf06363e5ec")
	b11.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b11.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	userBalance += gasUsed * gasPrice
	minerBalance -= gasUsed * gasPrice
	contractBlockHandle(t, blockChain, b10, b11, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b11.Header.RootHash, &b11.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("miner nonce: %d", stateDB.GetNonce(*minerAddr.Hash160()))
	t.Logf("b10 -> b11 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b11 -> b12
	// make balances user call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(3017), uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfUserCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b10.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b12 := nextBlockWithTxs(b11, vmTx)
	b12.Header.RootHash.SetString("9d080a0ffd7487b1a9fea1d6e034b67b6630e37042822cfddfda7838fbd68d52")
	b12.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b12.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b11, b12, vmParam, nil, gasRefundTx)
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
	gasUsed, vmValue, gasPrice, gasLimit = uint64(3017), uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfReceiverCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b12.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b13 := nextBlockWithTxs(b12, vmTx)
	b13.Header.RootHash.SetString("1f3ae111528982e5cb70fbac187d25644c33836949e61c4c201b28890612e491")
	b13.Header.UtxoRoot.SetString("a1fe9e077b04c4ae4fbae1edce9da439ca587e3c3e0032068f525d7342d3bd36")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed).WithTxIndex(1)
	b13.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b12, b13, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b13.Header.RootHash, &b13.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b12 -> b13 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return d0471f0000000000000000000000000000000000000000000000000000000000
	// 0x1f47d0 = 2050000, check okay
}
