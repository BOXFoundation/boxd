// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package chain
package chain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/abi"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/script"
	"github.com/BOXFoundation/boxd/vm"
	"github.com/BOXFoundation/boxd/vm/common/hexutil"
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
		price, limit   uint64
		nonce          uint64
		version        int32
		err            error
	}{
		{100, "b1VAnrX665aeExMaPeW6pk3FZKCLuywUaHw", "b5WYphc4yBPH18gyFthS1bHyRcEvM6xANuT",
			testVMScriptCode, 100, 20000, 1, 0, nil},
		{0, "b1VAnrX665aeExMaPeW6pk3FZKCLuywUaHw", "", testVMScriptCode, 100, 20000, 2, 0, nil},
	}
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
			tc.price, tc.limit, tc.nonce, tc.version)
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
		btx, err := ExtractVMTransaction(tx)
		if err != nil {
			t.Fatal(err)
		}
		// check
		hashWith, _ := tx.TxHash()
		if *btx.OriginTxHash() != *hashWith ||
			*btx.From() != *from.Hash160() ||
			(btx.To() != nil && *btx.To() != *toAddressHash) ||
			btx.Value().Cmp(big.NewInt(int64(tc.value))) != 0 ||
			btx.GasPrice().Cmp(big.NewInt(int64(tc.price))) != 0 ||
			btx.Gas() != tc.limit || btx.Nonce() != tc.nonce ||
			btx.Version() != tc.version {
			t.Fatalf("want: %+v originTxHash: %s, got BoxTransaction: %+v", tc, hashWith, btx)
		}
	}
}

// generate a child block with contract tx
func nextBlockWithTxs(
	parent *types.Block, chain *BlockChain, txs ...*types.Transaction,
) *types.Block {
	newBlock := nextBlock(parent, chain)
	newBlock.Txs = append(newBlock.Txs, txs...)
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
	return newBlock
}

// generate a child block with contract tx
func nextBlockWithTxsV2(
	parent *types.Block, chain *BlockChain, txs ...*types.Transaction,
) *types.Block {
	newBlock := nextBlockV2(parent, chain)
	newBlock.Txs = append(newBlock.Txs, txs...)
	newBlock.Header.TxsRoot = *CalcTxsHash(newBlock.Txs)
	return newBlock
}

var (
	userBalance, minerBalance, contractBalance, coinbaseGasUsed uint64
)

type testContractParam struct {
	vmValue, gasPrice, gasLimit, contractBalance, userRecv uint64

	contractAddr *types.AddressContract
}

func genTestChain(t *testing.T, blockChain *BlockChain) *types.Block {
	var err error
	b0 := blockChain.tail

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b0 -> b1
	statedb, err := state.New(&b0.Header.RootHash, &b0.Header.UtxoRoot, blockChain.DB())
	if err != nil {
		t.Fatal(err)
	}
	timestamp = startTime
	b1 := nextBlockV2(b0, blockChain)
	if err := calcRootHash(b0, b1, blockChain, statedb); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b1, nil, 1, b1)

	statedb, err = state.New(&b1.Header.RootHash, &b1.Header.UtxoRoot, blockChain.DB())
	if err != nil {
		t.Fatal(err)
	}
	// add some box to miner
	mCoinbaseTx, _ := CreateCoinbaseTx(minerAddr.Hash(), b1.Header.Height)
	utxoSet := NewUtxoSet()
	utxoSet.AddUtxo(mCoinbaseTx, uint32(0), b1.Header.Height)
	utxoSet.WriteUtxoSetToDB(blockChain.db)
	statedb.AddBalance(*minerAddr.Hash160(), new(big.Int).SetUint64(BaseSubsidy))
	root, _, err := statedb.Commit(false)
	if err != nil {
		t.Fatal(err)
	}
	b1.Header.RootHash = *root

	contractKey, _ := types.NewContractAddressFromHash(ContractAddr.Bytes())
	balance := getBalance(contractKey.String(), blockChain.db)
	addr, _ := types.NewAddress(ContractAddr.String())
	stateBalance := blockChain.tailState.GetBalance(*addr.Hash160()).Uint64()
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, BaseSubsidy)
	t.Logf("b0 -> b1 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b1 -> b2
	// transfer some box to userAddr

	// calc coinbaseGasUsed
	statedb, _ = state.New(&b1.Header.RootHash, &b1.Header.UtxoRoot, blockChain.DB())
	newBlock := types.NewBlock(b1)
	newBlock.Header.BookKeeper = *minerAddr.Hash160()
	coinbaseVMTX, _ := makeCoinbaseTx(b1, newBlock, blockChain, statedb)
	coinbaseVMTX.Vin[0].Sequence = uint32(time.Now().UnixNano())
	vmtx, _ := ExtractVMTransaction(coinbaseVMTX)
	// new evm
	context := NewEVMContext(vmtx, newBlock.Header, blockChain)
	evm := vm.NewEVM(context, statedb, blockChain.vmConfig)
	_, coinbaseGasUsed, _, _, _, _ = ApplyMessage(evm, vmtx)
	t.Logf("coinbase gas used: %d", coinbaseGasUsed)

	// generate b2
	userBalance = uint64(600000000)
	prevHash, _ := mCoinbaseTx.TxHash()
	tx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(txlogic.MakeVout(userAddr.String(), userBalance)).
		AppendVout(txlogic.MakeVout(minerAddr.String(), BaseSubsidy-userBalance))
	err = txlogic.SignTx(tx, privKeyMiner, pubKeyMiner)
	ensure.DeepEqual(t, err, nil)
	b2 := nextBlockWithTxs(b1, blockChain, tx)
	if err := calcRootHash(b1, b2, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b2, nil, 2, b2)
	// check balance
	// for userAddr
	balance = getBalance(userAddr.String(), blockChain.db)
	stateBalance = blockChain.tailState.GetBalance(*userAddr.Hash160()).Uint64()
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, userBalance)
	t.Logf("user balance: %d", userBalance)
	// for miner
	balance = getBalance(minerAddr.String(), blockChain.db)
	stateBalance = blockChain.tailState.GetBalance(*minerAddr.Hash160()).Uint64()
	ensure.DeepEqual(t, balance, stateBalance)
	ensure.DeepEqual(t, balance, BaseSubsidy-userBalance)
	minerBalance = balance
	t.Logf("b1 -> b2 passed, now tail height: %d", blockChain.LongestChainHeight)
	return b2
}

func contractBlockHandle(
	t *testing.T, blockChain *BlockChain, block, tail *types.Block,
	param *testContractParam, err error,
) {

	verifyProcessBlock(t, blockChain, block, err, tail.Header.Height, tail)
	gasCost := (block.Header.GasUsed - coinbaseGasUsed) * param.gasPrice
	// check balance
	expectUserBalance := userBalance - param.vmValue - gasCost + param.userRecv
	expectMinerBalance := minerBalance
	if err != nil {
		if err == errInsufficientBalanceForGas {
			expectUserBalance, expectMinerBalance = userBalance, minerBalance
		} else {
			expectUserBalance, expectMinerBalance = userBalance, minerBalance
		}
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
	utxoBalance := getBalance(addr.String(), bc.db)
	stateBalance := bc.tailState.GetBalance(*addr.Hash160()).Uint64()
	t.Logf("%s utxo balance: %d state balance: %d", addr, utxoBalance, stateBalance)
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
	vmValue, gasPrice, gasLimit := uint64(10000), uint64(10), uint64(200000)
	vmParam := &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, vmValue, 0, nil,
	}

	byteCode, _ := hex.DecodeString(testFaucetContract)
	nonce := uint64(1)
	contractVout, err := txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue2 := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue2)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	stateDB, err := state.New(&b2.Header.RootHash, nil, blockChain.db)
	ensure.Nil(t, err)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr

	b3 := nextBlockWithTxs(b2, blockChain, vmTx)
	if err := calcRootHash(b2, b3, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b3, b3, vmParam, nil)
	stateDB, err = state.New(&b3.Header.RootHash, &b3.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, stateDB.GetNonce(*userAddr.Hash160()), nonce)

	gasUsed := b3.Header.GasUsed - coinbaseGasUsed
	gasRefundValue := vmParam.gasPrice * (vmParam.gasLimit - gasUsed)
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b3 -> b4
	// make call contract tx
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(20000)
	contractBalance := uint64(10000 - 2000) // withdraw 2000, construct contract with 10000
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, contractBalance, 2000, contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3.InternalTxs[0].TxHash()
	changeValue3 := gasRefundValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue3)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)

	b4 := nextBlockWithTxs(b3, blockChain, vmTx)
	if err := calcRootHash(b3, b4, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(1000), uint64(10), uint64(20000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, contractBalance, vmValue, vmParam.contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetContract)
	nonce++
	contractVout, err = txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue4 := changeValue2 - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue4)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)

	b5 := nextBlockWithTxs(b4, blockChain, vmTx)
	if err := calcRootHash(b4, b5, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(1000), uint64(10), uint64(600000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, contractBalance, vmValue, vmParam.contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetContract)
	nonce++
	contractVout, err = txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	prevHash, _ = b5.InternalTxs[0].TxHash()
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b6 := nextBlockWithTxs(b5, blockChain, vmTx)
	b6.Header.RootHash.SetString("")
	b6.Header.UtxoRoot.SetString("")
	contractBlockHandle(t, blockChain, b6, b5, vmParam, core.ErrInvalidFee)
	nonce--
	t.Logf("b5 -> b6 failed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b5 -> b6
	// make call contract tx with insufficient contract balance
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(20000)
	contractBalance = uint64(0) // withdraw 2000+9000, construct contract with 10000
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 8000, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(testFaucetCall2)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue6 := changeValue4 - vmValue - gasPrice*gasLimit
	logger.Warnf("utxo value: %d, gas: %d", changeValue4, gasPrice*gasLimit)
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue6)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b6 = nextBlockWithTxs(b5, blockChain, vmTx)

	if err := calcRootHash(b5, b6, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b6, b6, vmParam, nil)
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
	vmValue, gasPrice, gasLimit := uint64(0), uint64(10), uint64(400000)
	vmParam := &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, vmValue, 0, nil,
	}
	byteCode, _ := hex.DecodeString(testCoinContract)
	nonce := uint64(1)
	contractVout, err := txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	b3 := nextBlockWithTxs(b2, blockChain, vmTx)

	if err := calcRootHash(b2, b3, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(30000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(mintCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b4 := nextBlockWithTxs(b3, blockChain, vmTx)

	if err := calcRootHash(b3, b4, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(sendCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b4.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b5 := nextBlockWithTxs(b4, blockChain, vmTx)

	if err := calcRootHash(b4, b5, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balancesUserCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	// gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b6 := nextBlockWithTxs(b5, blockChain, vmTx)

	if err := calcRootHash(b5, b6, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balancesReceiverCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b6.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b7 := nextBlockWithTxs(b6, blockChain, vmTx)

	if err := calcRootHash(b6, b7, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit := uint64(0), uint64(2), uint64(2000000)
	vmParam := &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, vmValue, 0, nil,
	}
	byteCode, _ := hex.DecodeString(testERC20Contract)
	nonce := uint64(1)
	contractVout, err := txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	contractAddr, _ := types.MakeContractAddress(userAddr, nonce)
	vmParam.contractAddr = contractAddr
	t.Logf("contract address: %s", contractAddr)
	t.Logf("user addr: %s", hex.EncodeToString(userAddr.Hash160()[:]))
	b3 := nextBlockWithTxs(b2, blockChain, vmTx)

	if err := calcRootHash(b2, b3, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfUserCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b3A := nextBlockWithTxs(b3, blockChain, vmTx)

	if err := calcRootHash(b3, b3A, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(transferCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b3A.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b4 := nextBlockWithTxs(b3A, blockChain, vmTx)

	if err := calcRootHash(b3A, b4, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfUserCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b4.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b5 := nextBlockWithTxs(b4, blockChain, vmTx)

	if err := calcRootHash(b4, b5, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfReceiverCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b5.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b6 := nextBlockWithTxs(b5, blockChain, vmTx)

	if err := calcRootHash(b5, b6, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(2), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(transferFromCall)
	// minerNonce := uint64(9)
	minerNonce := stateDB.GetNonce(*minerAddr.Hash160()) + 2 // note: coinbase tx has already add 1.
	contractVout, err = txlogic.MakeContractCallVout(minerAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, minerNonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b2.Txs[1].TxHash()
	minerChangeValue := b2.Txs[1].Vout[1].Value - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(minerAddr.String(), minerChangeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKeyMiner, pubKeyMiner)
	b7 := nextBlockWithTxs(b6, blockChain, vmTx)

	if err := calcRootHash(b6, b7, blockChain); err != nil {
		t.Fatal(err)
	}
	gasUsed := b7.Header.GasUsed - coinbaseGasUsed
	userBalance += gasUsed * gasPrice
	minerBalance -= gasUsed * gasPrice
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(2), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(approveCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b6.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	// gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b8 := nextBlockWithTxs(b7, blockChain, vmTx)

	if err := calcRootHash(b7, b8, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(2), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(increaseAllowanceCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b8.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	// gasRefundTx = createGasRefundUtxoTx(userAddr.Hash160(), gasPrice*(gasLimit-gasUsed), nonce)
	b9 := nextBlockWithTxs(b8, blockChain, vmTx)

	if err := calcRootHash(b8, b9, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(2), uint64(40000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(allowanceCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b9.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b10 := nextBlockWithTxs(b9, blockChain, vmTx)

	if err := calcRootHash(b9, b10, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(2), uint64(40000)
	vmParam = &testContractParam{
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(transferFromCall)
	minerNonce = stateDB.GetNonce(*minerAddr.Hash160()) + 2 // note: coinbase tx has already add 1.
	contractVout, err = txlogic.MakeContractCallVout(minerAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, minerNonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b7.Txs[1].TxHash()
	minerChangeValue = b7.Txs[1].Vout[1].Value - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(minerAddr.String(), minerChangeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKeyMiner, pubKeyMiner)
	b11 := nextBlockWithTxs(b10, blockChain, vmTx)

	if err := calcRootHash(b10, b11, blockChain); err != nil {
		t.Fatal(err)
	}
	gasUsed = b11.Header.GasUsed - coinbaseGasUsed
	userBalance += gasUsed * gasPrice
	minerBalance -= gasUsed * gasPrice
	contractBlockHandle(t, blockChain, b11, b11, vmParam, nil)
	stateDB, err = state.New(&b11.Header.RootHash, &b11.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("miner nonce: %d", stateDB.GetNonce(*minerAddr.Hash160()))
	t.Logf("b10 -> b11 passed, now tail height: %d", blockChain.LongestChainHeight)

	// >>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	// extend main chain
	// b11 -> b12
	// make balances user call contract tx
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfUserCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b10.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b12 := nextBlockWithTxs(b11, blockChain, vmTx)

	if err := calcRootHash(b11, b12, blockChain); err != nil {
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
	vmValue, gasPrice, gasLimit = uint64(0), uint64(6), uint64(4000)
	vmParam = &testContractParam{
		// vmValue, gasPrice, gasLimit, contractBalance, userRecv, contractAddr
		vmValue, gasPrice, gasLimit, 0, 0, contractAddr,
	}
	byteCode, _ = hex.DecodeString(balanceOfReceiverCall)
	nonce++
	contractVout, err = txlogic.MakeContractCallVout(userAddr.Hash160(),
		contractAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	ensure.Nil(t, err)
	// use internal tx vout
	prevHash, _ = b12.Txs[1].TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx = types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, byteCode)
	txlogic.SignTx(vmTx, privKey, pubKey)
	b13 := nextBlockWithTxs(b12, blockChain, vmTx)

	if err := calcRootHash(b12, b13, blockChain); err != nil {
		t.Fatal(err)
	}
	contractBlockHandle(t, blockChain, b13, b13, vmParam, nil)
	stateDB, err = state.New(&b13.Header.RootHash, &b13.Header.UtxoRoot, blockChain.db)
	ensure.Nil(t, err)
	t.Logf("user nonce: %d", stateDB.GetNonce(*userAddr.Hash160()))
	t.Logf("b12 -> b13 passed, now tail height: %d", blockChain.LongestChainHeight)
}

func TestCallBetweenContracts(t *testing.T) {
	path := "./contracts/"
	// token
	tokenBinFile, tokenAbiFile := path+"token.bin", path+"token.abi"
	code, err := ioutil.ReadFile(tokenBinFile)
	if err != nil {
		t.Fatal(err)
	}
	tokenCode, _ := hex.DecodeString(string(bytes.TrimSpace(code)))
	tokenAbi, err := ReadAbi(tokenAbiFile)
	if err != nil {
		t.Fatal(err)
	}
	transferCall := func(addr *types.AddressHash, amount uint64) []byte {
		input, _ := tokenAbi.Pack("transfer", *addr, big.NewInt(int64(amount)))
		return input
	}
	// bank
	bankBinFile, bankAbiFile := path+"bank.bin", path+"bank.abi"
	code, err = ioutil.ReadFile(bankBinFile)
	if err != nil {
		t.Fatal(err)
	}
	bankCode, _ := hex.DecodeString(string(bytes.TrimSpace(code)))
	//t.Logf("bank code: %s", string(bankCode))
	bankAbi, err := ReadAbi(bankAbiFile)
	if err != nil {
		t.Fatal(err)
	}
	rechargeCall, err := bankAbi.Pack("recharge")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("user address: %x", userAddr.Hash160()[:])

	// deploy token contract
	blockChain := NewTestBlockChain()
	b2 := genTestChain(t, blockChain)
	vmValue, gasPrice, gasLimit := uint64(0), uint64(1), uint64(800000)
	nonce := uint64(1)
	contractVout, _ := txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, gasPrice, nonce)
	prevHash, _ := b2.Txs[1].TxHash()
	changeValue := userBalance - vmValue - gasPrice*gasLimit
	vmTx1 := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 0), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, tokenCode)
	txlogic.SignTx(vmTx1, privKey, pubKey)
	tokenAddr, _ := types.MakeContractAddress(userAddr, nonce)
	t.Logf("token contract address: %x", tokenAddr.Hash160()[:])
	// deploy bank contract
	vmValue, gasPrice, gasLimit = 80000, 1, 200000
	bankCode = append(bankCode, types.NormalizeAddressHash(tokenAddr.Hash160())[:]...)
	nonce++
	contractVout, _ = txlogic.MakeContractCreationVout(userAddr.Hash160(),
		vmValue, gasLimit, gasPrice, nonce)
	prevHash, _ = vmTx1.TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx2 := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, bankCode)
	txlogic.SignTx(vmTx2, privKey, pubKey)
	bankAddr, _ := types.MakeContractAddress(userAddr, nonce)
	bankBalance := vmValue
	userBalance -= vmValue
	t.Logf("bank contract address: %x", bankAddr.Hash160()[:])
	// call token.transfer api
	vmValue, gasPrice, gasLimit = 0, 1, 40000
	nonce++
	prevHash, _ = vmTx2.TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	input := transferCall(bankAddr.Hash160(), changeValue)
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(),
		tokenAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	vmTx3 := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, input)
	txlogic.SignTx(vmTx3, privKey, pubKey)
	// call bank.recharge api
	vmValue, gasPrice, gasLimit = 20000, 1, 30000
	nonce++
	contractVout, _ = txlogic.MakeContractCallVout(userAddr.Hash160(),
		bankAddr.Hash160(), vmValue, gasLimit, gasPrice, nonce)
	prevHash, _ = vmTx3.TxHash()
	changeValue = changeValue - vmValue - gasPrice*gasLimit
	vmTx4 := types.NewTx(0, 4455, 0).
		AppendVin(txlogic.MakeVin(types.NewOutPoint(prevHash, 1), 0)).
		AppendVout(contractVout).
		AppendVout(txlogic.MakeVout(userAddr.String(), changeValue)).
		WithData(types.ContractDataType, rechargeCall)
	txlogic.SignTx(vmTx4, privKey, pubKey)
	tokenBalance := vmValue
	userBalance -= vmValue

	// bring them on chain
	b3 := nextBlockWithTxs(b2, blockChain, vmTx1, vmTx2, vmTx3, vmTx4)
	if err := calcRootHash(b2, b3, blockChain); err != nil {
		t.Fatal(err)
	}
	verifyProcessBlock(t, blockChain, b3, nil, 3, b3)
	t.Logf("b3 block hash: %s", b3.BlockHash())
	t.Logf("b2 -> b3 passed, now tail height: %d", blockChain.LongestChainHeight)

	// check balance
	checkTestAddrBalance(t, blockChain, tokenAddr, tokenBalance)
	checkTestAddrBalance(t, blockChain, bankAddr, bankBalance)
	userBalance -= (b3.Header.GasUsed - coinbaseGasUsed) * gasPrice
	checkTestAddrBalance(t, blockChain, userAddr, userBalance)
}
