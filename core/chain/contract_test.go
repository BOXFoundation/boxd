// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package chain
package chain

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/vm/common/hexutil"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/facebookgo/ensure"
)

var unmarshalLogTests = map[string]struct {
	input     string
	want      *types.Log
	wantError error
}{
	"ok": {
		input: `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x000000000000000000000000000000000000000000000001a055690d9db80000","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		want: &types.Log{
			Address:     types.HexToAddressHash("0xecf8f87f810ecf450940c9f60066b4a7a501d6a7"),
			BlockHash:   crypto.BytesToHash([]byte("0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056")),
			BlockNumber: 2019236,
			Data:        hexutil.MustDecode("0x000000000000000000000000000000000000000000000001a055690d9db80000"),
			Index:       2,
			TxIndex:     3,
			TxHash:      crypto.BytesToHash([]byte("0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e")),
			Topics: []crypto.HashType{
				crypto.BytesToHash([]byte("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")),
			},
		},
	},
	"empty data": {
		input: `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		want: &types.Log{
			Address:     types.HexToAddressHash("0xecf8f87f810ecf450940c9f60066b4a7a501d6a7"),
			BlockHash:   crypto.BytesToHash([]byte("0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056")),
			BlockNumber: 2019236,
			Data:        []byte{},
			Index:       2,
			TxIndex:     3,
			TxHash:      crypto.BytesToHash([]byte("0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e")),
			Topics: []crypto.HashType{
				crypto.BytesToHash([]byte("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")),
				crypto.BytesToHash([]byte("0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615")),
			},
		},
	},
	"missing block fields (pending logs)": {
		input: `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","data":"0x","logIndex":"0x0","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		want: &types.Log{
			Address:     types.HexToAddressHash("0xecf8f87f810ecf450940c9f60066b4a7a501d6a7"),
			BlockHash:   crypto.HashType{},
			BlockNumber: 0,
			Data:        []byte{},
			Index:       0,
			TxIndex:     3,
			TxHash:      crypto.BytesToHash([]byte("0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e")),
			Topics: []crypto.HashType{
				crypto.BytesToHash([]byte("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")),
			},
		},
	},
	"Removed: true": {
		input: `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","data":"0x","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3","removed":true}`,
		want: &types.Log{
			Address:     types.HexToAddressHash("0xecf8f87f810ecf450940c9f60066b4a7a501d6a7"),
			BlockHash:   crypto.BytesToHash([]byte("0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056")),
			BlockNumber: 2019236,
			Data:        []byte{},
			Index:       2,
			TxIndex:     3,
			TxHash:      crypto.BytesToHash([]byte("0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e")),
			Topics: []crypto.HashType{
				crypto.BytesToHash([]byte("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")),
			},
			Removed: true,
		},
	},
	"missing data": {
		input:     `{"address":"0xecf8f87f810ecf450940c9f60066b4a7a501d6a7","blockHash":"0x656c34545f90a730a19008c0e7a7cd4fb3895064b48d6d69761bd5abad681056","blockNumber":"0x1ecfa4","logIndex":"0x2","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x00000000000000000000000080b2c9d7cbbf30a1b0fc8983c647d754c6525615","0x000000000000000000000000f9dff387dcb5cc4cca5b91adb07a95f54e9f1bb6"],"transactionHash":"0x3b198bfd5d2907285af009e9ae84a0ecd63677110d89d7e030251acb87f6487e","transactionIndex":"0x3"}`,
		wantError: fmt.Errorf("missing required field 'data' for Log"),
	},
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

// func TestAbi(t *testing.T) {
// 	aaa := abi.ABI{}
// 	aaa.UnmarshalJSON([]byte(testERC20Abi))
// 	fmt.Printf("%x", aaa.Events["Transfer"].Id())
// }

func _TestExtractBoxTx(t *testing.T) {
	// contract Temp {
	//     function () payable {}
	// }
	testVMScriptCode := "6060604052346000575b60398060166000396000f30060606040525b600b" +
		"5b5b565b0000a165627a7a723058209cedb722bf57a30e3eb00eeefc392103ea791a2001deed29" +
		"f5c3809ff10eb1dd0029"
	var tests = []struct {
		value        uint64
		addrStr      string
		code         string
		price, limit uint64
		nonce        uint64
		version      int32
		err          error
	}{
		{100, "b1YMx5kufN2qELzKaoaBWzks2MZknYqqPnh", testVMScriptCode, 100, 20000, 1, 0, nil},
		{0, "", testVMScriptCode, 100, 20000, 2, 0, nil},
	}
	blockChain := NewTestBlockChain()
	for _, tc := range tests {
		var addr types.Address
		if tc.addrStr != "" {
			addr, _ = types.NewAddress(tc.addrStr)
		}
		code, _ := hex.DecodeString(tc.code)
		cs, err := script.MakeContractScriptPubkey(addr, code, tc.price, tc.limit, tc.nonce, tc.version)
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
		tx := types.NewTx(0, 4455, 100).AppendVin(txin).AppendVout(txout)
		btx, err := blockChain.ExtractVMTransactions(tx)
		if err != nil {
			t.Fatal(err)
		}
		// check
		sender, _ := types.NewAddress("b1ndoQmEd83y4Fza5PzbUQDYpT3mV772J5o")
		hashWith, _ := tx.TxHash()
		if *btx.OriginTxHash() != *hashWith ||
			*btx.From() != *sender.Hash160() ||
			(btx.To() != nil && *btx.To() != *addr.Hash160()) ||
			btx.Value().Cmp(big.NewInt(int64(tc.value))) != 0 ||
			btx.GasPrice().Cmp(big.NewInt(int64(tc.price))) != 0 ||
			btx.Gas() != tc.limit || btx.Nonce() != tc.nonce ||
			btx.Version() != tc.version {
			t.Fatalf("want: %+v, got BoxTransaction: %+v", tc, btx)
		}
	}
}

// generate a child block with contract tx
func nextBlockWithTxs(parent *types.Block, txs ...*types.Transaction) *types.Block {
	timestamp++
	newBlock := types.NewBlock(parent)

	coinbaseTx, _ := CreateCoinbaseTx(minerAddr.Hash(), parent.Header.Height+1)
	// use time to ensure we create a different/unique block each time
	coinbaseTx.Vin[0].Sequence = uint32(time.Now().UnixNano())
	newBlock.Txs = append(newBlock.Txs, coinbaseTx)
	newBlock.Txs = append(newBlock.Txs, txs...)
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
	newBlock.Header.TimeStamp = timestamp
	return newBlock
}

var (
	userBalance, minerBalance, contractBalance uint64
)

type testContractParam struct {
	gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv uint64

	contractAddr *types.AddressContract
}

func genTestChain(t *testing.T, blockChain *BlockChain) *types.Block {
	b0 := getTailBlock(blockChain)
	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1
	b1 := nextBlock(b0)
	b1.Header.RootHash.SetString("c1fe380cc58ea1bfdb6bf400f925b8d28df9699018d99e03edb15f855cf334e6")
	verifyProcessBlock(t, blockChain, b1, nil, 1, b1)
	balance := getBalance(minerAddr.String(), blockChain.db)
	stateBalance, _ := blockChain.GetBalance(minerAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, BaseSubsidy)
	t.Logf("b0 -> b1 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b1 -> b2
	// transfer some box to userAddr
	userBalance = uint64(6000000)
	prevHash, _ := b1.Txs[0].TxHash()
	tx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), userBalance)).
		AppendVout(txlogic.MakeVout(minerAddr.String(), BaseSubsidy-userBalance))
	err := signTx(tx, privKeyMiner, pubKeyMiner)
	ensure.DeepEqual(t, err, nil)

	b2 := nextBlockWithTxs(b1, tx)
	b2.Header.RootHash.SetString("97057e28c8faf3dc1343890faa4aaf65765c6ea726eb0125d9188c202bb8a3d7")
	verifyProcessBlock(t, blockChain, b2, nil, 2, b2)
	// check balance
	// for userAddr
	balance = getBalance(userAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(userAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, userBalance)
	t.Logf("user balance: %d", userBalance)
	// for miner
	balance = getBalance(minerAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(minerAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, 2*BaseSubsidy-userBalance)
	minerBalance = balance
	t.Logf("b1 -> b2 passed, now tail height: %d", blockChain.LongestChainHeight)
	return b2
}

func contractBlockHandle(
	t *testing.T, blockChain *BlockChain, parent, block, tail *types.Block,
	param *testContractParam, err error, internalTxs ...*types.Transaction,
) {

	gasCost := param.gasUsed * param.gasPrice
	if len(internalTxs) > 0 {
		block.InternalTxs = append(block.InternalTxs, internalTxs...)
		block.Header.InternalTxsRoot.SetBytes(CalcTxsHash(block.InternalTxs)[:])
	}
	block.Header.GasUsed = param.gasUsed
	block.Txs[0].Vout[0].Value += gasCost
	block.Txs[0].ResetTxHash()
	block.Header.TxsRoot = *CalcTxsHash(block.Txs)
	height := tail.Header.Height
	verifyProcessBlock(t, blockChain, block, err, height, tail)
	// check balance
	expectUserBalance := userBalance - param.vmValue - gasCost + param.userRecv
	expectMinerBalance := minerBalance + BaseSubsidy + gasCost
	if err != nil {
		if err == errInsufficientBalanceForGas {
			expectUserBalance, expectMinerBalance = userBalance, minerBalance
		} else {
			expectUserBalance, expectMinerBalance = userBalance, minerBalance
		}
	}
	// for userAddr
	balance := getBalance(userAddr.String(), blockChain.db)
	stateBalance, _ := blockChain.GetBalance(userAddr)
	t.Logf("user %s balance: %d, stateBalance: %d, expect balance: %d",
		userAddr, balance, stateBalance, expectUserBalance)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, expectUserBalance)
	userBalance = balance
	// for miner
	balance = getBalance(minerAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(minerAddr)
	t.Logf("miner %s balance: %d, stateBalance: %d, expect balance: %d",
		minerAddr, balance, stateBalance, expectMinerBalance)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, expectMinerBalance)
	minerBalance = balance
	// for contract address
	balance = getBalance(param.contractAddr.String(), blockChain.db)
	stateBalance, _ = blockChain.GetBalance(param.contractAddr)
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, stateBalance, param.contractBalance)
	contractBalance = stateBalance
	t.Logf("contract address %s balance: %d", param.contractAddr, contractBalance)
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
	// withdraw 9000
	testFaucetCall2 = "2e1a7d4d0000000000000000000000000000000000000000000000000000000000002328"
)

func TestFaucetContract(t *testing.T) {
	blockChain := NewTestBlockChain()
	ensure.NotNil(t, blockChain)
	ensure.True(t, blockChain.LongestChainHeight == 0)

	// contract blocks test
	b2 := genTestChain(t, blockChain)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b2 -> b3
	// make creation contract tx
	gasUsed, vmValue, gasPrice, gasLimit := uint64(56160), uint64(10000), uint64(10), uint64(200000)
	vmParam := &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, vmValue, 0, nil,
	}

	byteCode, _ := hex.DecodeString(testFaucetContract)
	nonce := uint64(1)
	contractVout, err := txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue2 := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue2))
	signTx(vmTx, privKey, pubKey)
	vmTxHash, _ := vmTx.TxHash()
	t.Logf("vmTx hash: %s", vmTxHash)
	stateDB, err := state.New(&b2.Header.RootHash, nil, blockChain.db)
	ensure.Nil(t, err)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b3 := nextBlockWithTxs(b2, vmTx)
	b3.Header.RootHash.SetString("7a999efa9fcc0d58b3a0e0959ef40cfef8df019a7945d7157033d24435690e83")
	b3.Header.UtxoRoot.SetString("23e986e2282ddfb84d003b81a32f801eac7579a7a76e66519896b27d0ae362f7")
	receipt := types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, nil).WithTxIndex(1)
	b3.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b2, b3, b3, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)

	gasRefundValue := vmParam.gasPrice * (vmParam.gasLimit - vmParam.gasUsed)
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b4
	// make call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(9912), uint64(0), uint64(6), uint64(20000)
	contractBalance := uint64(10000 - 2000) // withdraw 2000, construct contract with 10000
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, contractBalance, 2000, contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3.InternalTxs[0].TxHash()
	changeValue3 := gasRefundValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue3))
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	op := types.NewOutPoint(types.NormalizeAddressHash(contractAddr.Hash160()), 0)
	contractTx := types.NewTx(0, 0, 0).
		AppendVin(txlogic.MakeContractVin(op, 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), 2000))
	signTx(vmTx, privKey, pubKey)
	vmTxHash, _ = vmTx.TxHash()
	t.Logf("vmTx hash: %s", vmTxHash)
	b4 := nextBlockWithTxs(b3, vmTx)
	b4.Header.RootHash.SetString("af3defef7e1a7fe8866ba1e14be3a567920a04bae7a77bdbf4095f9a0ecd6f3f")
	b4.Header.UtxoRoot.SetString("e39a03ffead32d90383f7ca635b0d7850a2b8f22ea6bafa1de96d8e5f98af2a3")
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, nil).WithTxIndex(1)
	b4.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b3, b4, b4, vmParam, nil, gasRefundTx, contractTx)
	stateDB, err = state.New(&b4.Header.RootHash, &b4.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)
	t.Logf("b3 -> b4 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b4 -> b5
	// make creation contract tx with insufficient gas
	gasUsed, vmValue, gasPrice, gasLimit = uint64(20000), uint64(1000), uint64(10), uint64(20000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, contractBalance, vmValue, vmParam.contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetContract)
	nonce++
	contractVout, err = txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue4 := changeValue2 - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue4))
	signTx(vmTx, privKey, pubKey)
	// make refund tx
	contractAddrHash := types.CreateAddress(*userAddr.Hash160(), nonce)
	refundTx := types.NewTx(0, 0, 0).
		AppendVin(txlogic.MakeContractVin(
			types.NewOutPoint(types.NormalizeAddressHash(contractAddrHash), 0), 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), vmValue))
	//
	b5 := nextBlockWithTxs(b4, vmTx)
	b5.Header.RootHash.SetString("0bf64507600e191311f40c2ad178be827e6e43eafbd053d9c7210da1b7a35dd8")
	b5.Header.UtxoRoot.SetString("8909c72909a9959740e0b9710c59c1f218d2a6a3e1b4bb9096ac4803feb5fd5b")
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), true, gasUsed, nil).WithTxIndex(1)
	b5.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b4, b5, b5, vmParam, nil, refundTx)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)
	t.Logf("b4 -> b5 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make creation contract tx with insufficient balance
	gasUsed, vmValue, gasPrice, gasLimit = uint64(20000), uint64(1000), uint64(10), uint64(600000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, contractBalance, vmValue, vmParam.contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetContract)
	nonce++
	contractVout, err = txlogic.MakeContractCreationVout(vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	prevHash, _ = b5.InternalTxs[0].TxHash()
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout)
	signTx(vmTx, privKey, pubKey)
	b6 := nextBlockWithTxs(b5, vmTx)
	b6.Header.RootHash.SetString("")
	b6.Header.UtxoRoot.SetString("")
	contractBlockHandle(t, blockChain, b5, b6, b5, vmParam, errInsufficientBalanceForGas, refundTx)
	nonce--
	t.Logf("b5 -> b6 failed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make call contract tx with insufficient contract balance
	gasUsed, vmValue, gasPrice, gasLimit = uint64(9914), uint64(0), uint64(6), uint64(20000)
	contractBalance = uint64(0) // withdraw 2000+9000, construct contract with 10000
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 8000, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall2)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue6 := changeValue4 - vmValue - gasPrice*gasLimit
	logger.Warnf("utxo value: %d, gas: %d", changeValue4, gasPrice*gasLimit)
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue6))
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	signTx(vmTx, privKey, pubKey)
	vmTxHash, _ = vmTx.TxHash()
	t.Logf("vmTx hash: %s", vmTxHash)
	b6 = nextBlockWithTxs(b5, vmTx)
	b6.Header.RootHash.SetString("b955aba83fd8773d33dab64bb961c2c72cc095fc0529a41984f4b4c39c05b54b")
	b6.Header.UtxoRoot.SetBytes(b5.Header.UtxoRoot[:])
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), true, gasUsed, nil).WithTxIndex(1)
	b6.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b5, b6, b6, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b6.Header.RootHash, &b6.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)
	t.Logf("b5 -> b6 passed, now tail height: %d", blockChain.LongestChainHeight)
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
	log.Printf("mint 8000000: %s\n", mintCall)
	// sent 2000000
	input, err = abiObj.Pack("send", *receiver.Hash160(), big.NewInt(2000000))
	if err != nil {
		log.Fatal(err)
	}
	sendCall = hex.EncodeToString(input)
	log.Printf("send 2000000: %s\n", sendCall)
	// balances user addr
	input, err = abiObj.Pack("balances", *userAddr.Hash160())
	if err != nil {
		log.Fatal(err)
	}
	balancesUserCall = hex.EncodeToString(input)
	log.Printf("balancesUser: %s\n", balancesUserCall)
	// balances test Addr
	input, err = abiObj.Pack("balances", receiver.Hash160())
	if err != nil {
		log.Fatal(err)
	}
	balancesReceiverCall = hex.EncodeToString(input)
	log.Printf("balances %s: %s\n", receiver, balancesReceiverCall)
}

func _TestCoinContract(t *testing.T) {

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
	gasUsed, vmValue, gasPrice, gasLimit := uint64(246403), uint64(0), uint64(10), uint64(400000)
	vmParam := &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, vmValue, 0, nil,
	}
	byteCode, _ := hex.DecodeString(testCoinContract)
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
	// TODO
	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b3 := nextBlockWithTxs(b2, vmTx)
	b3.Header.RootHash.SetString("e64182ce966ec345c08a8f139331e7b96c3cf6069c5e2f55a080a0061fa09ae1")
	utxoRootHash := "38c155f64dcf43043ea9b0002b71d11d4fd58c0fa6921e59bd7d812e574ccf28"
	b3.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ := vmTx.TxHash()
	receipt := types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, nil).WithTxIndex(1)
	b3.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b2, b3, b3, vmParam, nil, gasRefundTx)
	stateDB, err := state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	//refundValue := vmParam.gasPrice * (vmParam.gasLimit - vmParam.gasUsed)
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b4
	// make mint 8000000 call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(23177), uint64(0), uint64(6), uint64(30000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(mintCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(contractAddr.String(),
		vmValue, gasLimit, gasPrice, nonce, byteCode)
	ensure.Nil(t, err)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue))
	signTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b4 := nextBlockWithTxs(b3, vmTx)
	b4.Header.RootHash.SetString("74094671ea59c16d215cb0797f51d8ded5a1e0c8932bddb82b2a86f7b9366fe6")
	b4.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, nil).WithTxIndex(1)
	b4.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b3, b4, b4, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b4.Header.RootHash, &b4.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b3 -> b4 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b4 -> b5
	// make send 2000000 call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(30219), uint64(0), uint64(6), uint64(40000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(sendCall)
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
	b5.Header.RootHash.SetString("aa0c32471602b461406f808a27d0f911f4e440ff62280c5c059b52a556bd5130")
	b5.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, nil).WithTxIndex(1)
	b5.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b4, b5, b5, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b5.Header.RootHash, &b5.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b4 -> b5 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make balances user call contract tx
	gasUsed, vmValue, gasPrice, gasLimit = uint64(2825), uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balancesUserCall)
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
	b6.Header.RootHash.SetString("404aa027eb404b7d3608ccc4f87f7e866916cf4ef646176088e68ad63b0e8779")
	b6.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, nil).WithTxIndex(1)
	b6.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b5, b6, b6, vmParam, nil, gasRefundTx)
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
	gasUsed, vmValue, gasPrice, gasLimit = uint64(2825), uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// gasUsed, vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		gasUsed, vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balancesReceiverCall)
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
	gasRefundTxHash, _ := gasRefundTx.TxHash()
	t.Logf("refund tx: %s", gasRefundTxHash)
	b7 := nextBlockWithTxs(b6, vmTx)
	b7.Header.RootHash.SetString("a0589aeda39379bc4a5e3c368ea43c3acbd5e5395af9c60bc890826d5a28ae5d")
	b7.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, nil).WithTxIndex(1)
	b7.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	contractBlockHandle(t, blockChain, b6, b7, b7, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b7.Header.RootHash, &b7.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b6 -> b7 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return 0x1e8480 = 2000000, check okay
}

func _TestERC20Contract(t *testing.T) {

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
	txlogic.SignTx(vmTx, privKey, pubKey)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	t.Logf("user addr: %s", hex.EncodeToString(userAddr.Hash160()[:]))
	//
	gasRefundTx := createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b3 := nextBlockWithTxs(b2, vmTx)
	b3.Header.RootHash.SetString("f7c3382e36452cf8df488d4d07b2915f43e871536f0e9ee3b2c0e090ef5407a0")
	utxoRootHash := "38c155f64dcf43043ea9b0002b71d11d4fd58c0fa6921e59bd7d812e574ccf28"
	b3.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ := vmTx.TxHash()
	logs := []*types.Log{unmarshalLogTests["ok"].want}
	logs[0].Address.SetBytes(contractAddr.Hash())
	eventid, _ := hex.DecodeString("ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
	logs[0].Topics[0].SetBytes(eventid)
	receipt := types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, nil).WithTxIndex(1)
	// receipt := types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, logs).WithTxIndex(1)
	b3.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	// b3.Header.ReceiptHash, _ = hex.DecodeString("31306230653333383136323964363137306532656330303238353335646135316539383537363331663165353763666366313761353166353837323534356465")
	b3.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	contractBlockHandle(t, blockChain, b2, b3, b3, vmParam, nil, gasRefundTx)
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
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b3A := nextBlockWithTxs(b3, vmTx)
	b3A.Header.RootHash.SetString("92185cea3cc5e92e61a93aad8d447b9bd6619a98a16c37e4e234f12251da925b")
	b3A.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, logs).WithTxIndex(1)
	b3A.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	b3A.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	contractBlockHandle(t, blockChain, b3, b3A, b3A, vmParam, nil, gasRefundTx)
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
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b4 := nextBlockWithTxs(b3A, vmTx)
	b4.Header.RootHash.SetString("d5a85c1cbc4fe6db563b2f5e375cabbcaec9fb649ac13b2a4ccf8839a82c7a0b")
	b4.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, logs).WithTxIndex(1)
	b4.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	b4.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	contractBlockHandle(t, blockChain, b3A, b4, b4, vmParam, nil, gasRefundTx)
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
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b5 := nextBlockWithTxs(b4, vmTx)
	b5.Header.RootHash.SetString("ad50bfd5f3801eb79ed0778acb3870f18bffba04140c9edb486cab657eac7c50")
	b5.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, logs).WithTxIndex(1)
	b5.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	b5.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	contractBlockHandle(t, blockChain, b4, b5, b5, vmParam, nil, gasRefundTx)
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
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b6 := nextBlockWithTxs(b5, vmTx)
	b6.Header.RootHash.SetString("431863d3250351ec4d9945f2f3affce6f34880b370b780a7b85fb6aa76fda4f1")
	b6.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, logs).WithTxIndex(1)
	b6.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	b6.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	contractBlockHandle(t, blockChain, b5, b6, b6, vmParam, nil, gasRefundTx)
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
	txlogic.SignTx(vmTx, privKeyMiner, pubKeyMiner)
	gasRefundTx = createGasRefundUtxoTx(minerAddr.Hash160(), gasPrice*(gasLimit-gasUsed), minerNonce)
	b7 := nextBlockWithTxs(b6, vmTx)
	b7.Header.RootHash.SetString("e8afb2306dcbaa6c61669220c3bad300a421082b4bcc6aaea7d481e831fce9b8")
	b7.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), true, gasUsed, logs).WithTxIndex(1)
	b7.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	b7.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	userBalance += gasUsed * gasPrice
	minerBalance -= gasUsed * gasPrice
	contractBlockHandle(t, blockChain, b6, b7, b7, vmParam, nil, gasRefundTx)
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
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b8 := nextBlockWithTxs(b7, vmTx)
	b8.Header.RootHash.SetString("4c92d920e640fe8eaacb84516001bfebd0f037d19f352597676d2910a85b6123")
	b8.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, logs).WithTxIndex(1)
	b8.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	b8.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	contractBlockHandle(t, blockChain, b7, b8, b8, vmParam, nil, gasRefundTx)
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
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b9 := nextBlockWithTxs(b8, vmTx)
	b9.Header.RootHash.SetString("7816518ce1ebd785bee0a4618597fbf300461cb627942a6ba683401788e0f913")
	b9.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, logs).WithTxIndex(1)
	b9.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	b9.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	contractBlockHandle(t, blockChain, b8, b9, b9, vmParam, nil, gasRefundTx)
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
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b10 := nextBlockWithTxs(b9, vmTx)
	b10.Header.RootHash.SetString("dfce597d21b1db7c34bda440906a240ec2e471ad34f41e8c9a9761c11fb0e621")
	b10.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, logs).WithTxIndex(1)
	b10.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	b10.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	contractBlockHandle(t, blockChain, b9, b10, b10, vmParam, nil, gasRefundTx)
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
	txlogic.SignTx(vmTx, privKeyMiner, pubKeyMiner)
	gasRefundTx = createGasRefundUtxoTx(minerAddr.Hash160(), gasPrice*(gasLimit-gasUsed), minerNonce)
	b11 := nextBlockWithTxs(b10, vmTx)
	b11.Header.RootHash.SetString("9a6d121fcacacc199c687343ddda4bb40375e4f796e7bf5ef110140dbb89ea13")
	b11.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, logs).WithTxIndex(1)
	b11.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	b11.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	userBalance += gasUsed * gasPrice
	minerBalance -= gasUsed * gasPrice
	contractBlockHandle(t, blockChain, b10, b11, b11, vmParam, nil, gasRefundTx)
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
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b12 := nextBlockWithTxs(b11, vmTx)
	b12.Header.RootHash.SetString("005f1476e3bb297d71f9b3239b914b26dc331b1d5ea313959f6afca7976ac277")
	b12.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, logs).WithTxIndex(1)
	b12.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	b12.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	contractBlockHandle(t, blockChain, b11, b12, b12, vmParam, nil, gasRefundTx)
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
	txlogic.SignTx(vmTx, privKey, pubKey)
	gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b13 := nextBlockWithTxs(b12, vmTx)
	b13.Header.RootHash.SetString("61ad6fa5ef8dacf0f512b0d20c59dc50dd9d071b2b539d02611dd719238bd84f")
	b13.Header.UtxoRoot.SetString(utxoRootHash)
	vmTxHash, _ = vmTx.TxHash()
	receipt = types.NewReceipt(vmTxHash, contractAddr.Hash160(), false, gasUsed, logs).WithTxIndex(1)
	b13.Header.ReceiptHash = *(new(types.Receipts).Append(receipt).Hash())
	b13.Header.Bloom = types.CreateReceiptsBloom([]*types.Receipt{receipt})
	contractBlockHandle(t, blockChain, b12, b13, b13, vmParam, nil, gasRefundTx)
	stateDB, err = state.New(&b13.Header.RootHash, &b13.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b12 -> b13 passed, now tail height: %d", blockChain.LongestChainHeight)
	// check balances
	// return d0471f0000000000000000000000000000000000000000000000000000000000
	// 0x1f47d0 = 2050000, check okay

	// emptyBlockNum := 10
	// tmp := blockChain.tail
	// for i := 0; i < emptyBlockNum; i++ {

	// 	state, err := state.New(&tmp.Header.RootHash, &tmp.Header.UtxoRoot, blockChain.db)
	// 	ensure.Nil(t, err)
	// 	miner := minerAddr.Hash160()
	// 	state.AddBalance(*miner, big.NewInt(50*core.DuPerBox))
	// 	root, utxoroot, err := state.Commit(false)
	// 	ensure.Nil(t, err)

	// 	tmp = nextBlockWithTxs(tmp)
	// 	tmp.Header.TxsRoot = *CalcTxsHash(tmp.Txs)
	// 	tmp.Header.RootHash = *root
	// 	tmp.Header.UtxoRoot = *utxoroot
	// 	verifyProcessBlock(t, blockChain, tmp, nil, tmp.Header.Height, tmp)
	// }

	// topicslist := [][][]byte{
	// 	[][]byte{
	// 		logs[0].Address.Bytes(),
	// 	},
	// 	[][]byte{
	// 		eventid,
	// 	},
	// 	[][]byte{
	// 		// []byte("ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"),
	// 	},
	// 	[][]byte{},
	// }

	// heights, err := blockChain.sectionMgr.GetLogs(3, 10000, topicslist)
	// fmt.Println(heights)
	// ensure.Nil(t, err)

}
