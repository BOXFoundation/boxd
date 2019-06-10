// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"sync/atomic"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/integration_tests/utils"
	"github.com/BOXFoundation/boxd/rpc/rpcutil"
	acc "github.com/BOXFoundation/boxd/wallet/account"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

const (
	gasPrice  = 2
	createGas = uint64(1040000) * gasPrice
	sendGas   = uint64(40000) * 500 * gasPrice
)

// ContractTest manage circulation of ERC20 contract
type ContractTest struct {
	*BaseFmw
}

func contractTest() {
	t := NewContractTest(utils.ContractAccounts(), utils.ContractUnitAccounts())
	defer t.TearDown()

	// print tx count per TickerDurationTxs
	if scopeValue(*scope) == continueScope {
		go CountTxs(&tokenTestTxCnt, &t.txCnt)
	}

	t.Run(t.HandleFunc)
	logger.Info("done contract test")
}

// NewContractTest construct a ContractTest instance
func NewContractTest(accCnt int, partLen int) *ContractTest {
	t := &ContractTest{}
	t.BaseFmw = NewBaseFmw(accCnt, partLen)
	return t
}

// HandleFunc hooks test func
func (t *ContractTest) HandleFunc(addrs []string, index *int) (exit bool) {
	defer func() {
		if x := recover(); x != nil {
			utils.TryRecordError(fmt.Errorf("%v", x))
			logger.Error(x)
		}
	}()
	peerAddr := peersAddr[(*index)%len(peersAddr)]
	(*index)++
	//
	miner, ok := PickOneMiner()
	if !ok {
		logger.Warnf("have no miner address to pick")
		return true
	}
	defer UnpickMiner(miner)
	//
	logger.Infof("waiting for minersAddr %s has %d at least on %s for contract test",
		miner, createGas+2*sendGas, peerAddr)
	_, err := utils.WaitBalanceEnough(miner, createGas+sendGas, peerAddr, timeoutToChain)
	if err != nil {
		logger.Error(err)
		return true
	}
	if len(addrs) < 3 {
		logger.Errorf("contract test require 3 accounts at leat, now %d", len(addrs))
		return true
	}
	owner, spender, receivers := addrs[0], addrs[1], addrs[2:]
	conn, err := rpcutil.GetGRPCConn(peerAddr)
	if err != nil {
		logger.Warn(err)
		return false
	}
	defer conn.Close()
	minerAcc, _ := AddrToAcc.Load(miner)
	tx, _, _, err := rpcutil.NewTx(minerAcc.(*acc.Account), []string{owner, spender},
		[]uint64{createGas, sendGas}, conn)
	if err != nil {
		logger.Error(err)
		return
	}
	if _, err := rpcutil.SendTransaction(conn, tx); err != nil &&
		!strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		logger.Error(err)
		return
	}
	UnpickMiner(miner)
	atomic.AddUint64(&t.txCnt, 1)
	curTimes := utils.ContractRepeatTxTimes()
	if utils.ContractRepeatRandom() {
		curTimes = rand.Intn(utils.ContractRepeatTxTimes())
	}
	contractRepeatTest(owner, spender, receivers[0], curTimes, &t.txCnt, peerAddr)
	//
	return
}

func contractRepeatTest(
	owner, spender, receiver string, times int, txCnt *uint64, peerAddr string,
) {
	logger.Info("=== RUN   contractRepeatTest")
	defer logger.Info("=== DONE   contractRepeatTest")

	if times <= 0 {
		logger.Warn("times is 0, exit")
		return
	}

	conn, err := rpcutil.GetGRPCConn(peerAddr)
	if err != nil {
		logger.Panic(err)
	}
	defer conn.Close()

	// issue some token
	logger.Infof("%s issue 10000*10^8 token to %s", owner, spender)
	ownerAcc, _ := AddrToAcc.Load(owner)
	erc20Bytes, _ := hex.DecodeString(testERC20Contract)
	gasLimit, nonce := uint64(1000000), uint64(1)
	tx, contractAddr, err := rpcutil.NewContractDeployTx(ownerAcc.(*acc.Account), gasPrice,
		gasLimit, nonce, erc20Bytes, conn)
	if err != nil {
		logger.Panic(err)
	}
	logger.Infof("contract addr: %s", contractAddr)
	nonce++
	if _, err := rpcutil.SendTransaction(conn, tx); err != nil &&
		!strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		logger.Panic(err)
	}
	atomic.AddUint64(txCnt, 1)

	// check issue result
	totalAmount := uint64(10000 * 100000000)
	logger.Infof("wait for ERC20 balance of owner %s equal to %d, timeout %v",
		spender, totalAmount, timeoutToChain)
	code := erc20BalanceOfCall(owner)
	err = utils.WaitERC20BalanceEqualTo(owner, contractAddr, totalAmount, code,
		peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}

	// approve spender 10000*10^8
	logger.Infof("%s approve spender %s 10000*10^8 token", owner, spender)
	gasLimit = 40000
	code = erc20ApproveCall(spender, totalAmount)
	tx, err = rpcutil.NewContractCallTx(ownerAcc.(*acc.Account), contractAddr, gasPrice,
		gasLimit, nonce, code, conn)
	if err != nil {
		logger.Panic(err)
	}
	nonce++
	if _, err := rpcutil.SendTransaction(conn, tx); err != nil &&
		!strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		logger.Panic(err)
	}
	atomic.AddUint64(txCnt, 1)

	// check approve result
	logger.Infof("wait for ERC20 allowance of spender %s equal to %d, timeout %v",
		spender, totalAmount, timeoutToChain)
	code = erc20AllowanceCall(owner, spender)
	err = utils.WaitERC20AllowanceEqualTo(spender, contractAddr, totalAmount, code,
		peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}

	// transferFrom contract token times
	logger.Infof("%s transfer %d times contract token to %s", spender, times, receiver)
	spenderAcc, _ := AddrToAcc.Load(spender)
	amount, spenderNonce := uint64(80000), uint64(1)
	code = erc20TransferFromCall(owner, receiver, amount)
	tx, err = rpcutil.NewContractCallTx(spenderAcc.(*acc.Account), contractAddr,
		gasPrice, gasLimit, spenderNonce, code, conn)
	if _, err := rpcutil.SendTransaction(conn, tx); err != nil &&
		!strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
		logger.Panic(err)
	}
	atomic.AddUint64(txCnt, 1)

	//logger.Infof("%s transfer %d times contract token to %s", spender, times, receiver)
	//spenderAcc, _ := AddrToAcc.Load(spender)
	//ave := totalAmount / uint64(times)
	//amount := ave/2 + uint64(rand.Int63n(int64(ave/2)))
	//txs, err := utils.NewContractTransferFromTxs(spenderAcc, contractAddr, gasPrice,
	//	gasLimit, nonce, transferFromCall, conn)
	//// cend contract transferFrom txs
	//for _, tx := range txs {
	//	if _, err := rpcutil.SendTransaction(conn, tx); err != nil &&
	//		!strings.Contains(err.Error(), core.ErrOrphanTransaction.Error()) {
	//		logger.Panic(err)
	//	}
	//	atomic.AddUint64(txCnt, 1)
	//	time.Sleep(2 * time.Millisecond)
	//}
	//logger.Infof("%s sent %d times total %d token tx to %s", spender, times,
	//	txTotalAmount, receiver)

	// check transferFrom result
	// for owner
	logger.Infof("wait for ERC20 balance of owner %s equal to %d, timeout %v",
		owner, totalAmount-amount, timeoutToChain)
	code = erc20BalanceOfCall(owner)
	err = utils.WaitERC20BalanceEqualTo(owner, contractAddr, totalAmount-amount,
		code, peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}
	// for receiver
	logger.Infof("wait for ERC20 balance of receiver %s equal to %d, timeout %v",
		receiver, amount, timeoutToChain)
	code = erc20BalanceOfCall(receiver)
	err = utils.WaitERC20BalanceEqualTo(receiver, contractAddr, amount, code,
		peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}
	// for owner box
	gasUsed := uint64(995919 + 24815)
	logger.Infof("wait for box balance of owner %s equal to %d, timeout %v",
		owner, createGas-gasUsed*gasPrice, timeoutToChain)
	_, err = utils.WaitBalanceEqual(owner, createGas-gasUsed*gasPrice, peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}
	// for spender box
	gasUsed = uint64(39782)
	logger.Infof("wait for box balance of spender %s equal to %d, timeout %v",
		receiver, sendGas-gasUsed*gasPrice, timeoutToChain)
	_, err = utils.WaitBalanceEqual(spender, sendGas-gasUsed*gasPrice, peerAddr, timeoutToChain)
	if err != nil {
		logger.Panic(err)
	}
}

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

var (
	abiObj, _ = abi.JSON(strings.NewReader(testERC20Abi))

	erc20BalanceOfCall = func(addr string) []byte {
		address, _ := types.NewAddress(addr)
		input, _ := abiObj.Pack("balanceOf", *address.Hash160())
		return input
	}
	erc20ApproveCall = func(addr string, amount uint64) []byte {
		address, _ := types.NewAddress(addr)
		input, _ := abiObj.Pack("approve", *address.Hash160(), big.NewInt(int64(amount)))
		return input
	}
	erc20AllowanceCall = func(owner, spender string) []byte {
		ownerAddr, _ := types.NewAddress(owner)
		spenderAddr, _ := types.NewAddress(spender)
		input, _ := abiObj.Pack("allowance", *ownerAddr.Hash160(), *spenderAddr.Hash160())
		return input
	}
	erc20TransferFromCall = func(from, to string, amount uint64) []byte {
		fromAddr, _ := types.NewAddress(from)
		toAddr, _ := types.NewAddress(to)
		input, _ := abiObj.Pack("transferFrom", *fromAddr.Hash160(), *toAddr.Hash160(),
			big.NewInt(int64(amount)))
		return input
	}
)
