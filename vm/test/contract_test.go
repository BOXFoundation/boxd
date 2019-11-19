// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/BOXFoundation/boxd/core/abi"
	"github.com/BOXFoundation/boxd/core/chain"
	coretypes "github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/vm"
	"github.com/BOXFoundation/boxd/vm/common/hexutil"
	"github.com/facebookgo/ensure"
)

var (
	userLiubei   = coretypes.BytesToAddressHash([]byte("liubei"))
	userZhangfei = coretypes.BytesToAddressHash([]byte("zhangfei"))
	userLikui    = coretypes.BytesToAddressHash([]byte("likui"))

	caller = vm.AccountRef(userLiubei)
)

/*
	pragma solidity ^0.4.25;
	contract selfdestructDemo{

		constructor() payable {

		}

		function kill(address add) public {
			selfdestruct(add);
		}
	}
*/
func TestSelfdestruct(t *testing.T) {

	bindata := "608060405260c9806100126000396000f300608060405260043610603f576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063cbf0b0c0146044575b600080fd5b348015604f57600080fd5b506082600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506084565b005b8073ffffffffffffffffffffffffffffffffffffffff16ff00a165627a7a723058204e48d91662dd473b6efc759814dcf1c227a4899b525a21da68e7241c68c881bf0029"
	jsondata := "[{\"constant\":false,\"inputs\":[{\"name\":\"add\",\"type\":\"address\"}],\"name\":\"kill\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"constructor\"}]"

	value := int64(1e10)

	contractAddr, abiObj, evm, stateDb := deploy(t, bindata, jsondata, value)

	input, err := abiObj.Pack("kill", userZhangfei)
	ensure.Nil(t, err)

	ret, _, err := evm.Call(caller, contractAddr, input, stateDb.GetBalance(userLiubei).Uint64(), big.NewInt(0), false)

	ensure.Nil(t, err)
	ensure.SameElements(t, ret, []byte{})
	ensure.DeepEqual(t, stateDb.GetBalance(userZhangfei).Int64(), value)
	ensure.DeepEqual(t, stateDb.GetBalance(contractAddr).Int64(), int64(0))
}

/*
	pragma solidity ^0.4.25;

	contract precompiledContractDemo {
		function testSha256(string input) public returns (bytes32) {
			return sha256(input);
		}
	}
*/
func TestPrecompiledContract(t *testing.T) {

	bindata := "608060405234801561001057600080fd5b506101a2806100206000396000f300608060405260043610610041576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680631422def914610046575b600080fd5b34801561005257600080fd5b506100ad600480360381019080803590602001908201803590602001908080601f01602080910402602001604051908101604052809392919081815260200183838082843782019150505050505091929192905050506100cb565b60405180826000191660001916815260200191505060405180910390f35b60006002826040518082805190602001908083835b60208310151561010557805182526020820191506020810190506020830392506100e0565b6001836020036101000a0380198251168184511680821785525050505050509050019150506020604051808303816000865af1158015610149573d6000803e3d6000fd5b5050506040513d602081101561015e57600080fd5b810190808051906020019092919050505090509190505600a165627a7a723058203ccfe047933f77fe034e068c3b49cdf098adfd27efbff499146b66732aa97d420029"
	jsondata := "[{\"constant\":false,\"inputs\":[{\"name\":\"input\",\"type\":\"string\"}],\"name\":\"testSha256\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

	contractAddr, abiObj, evm, stateDb := deploy(t, bindata, jsondata, 0)

	data := "The battle of red cliff."
	input, err := abiObj.Pack("testSha256", data)
	ensure.Nil(t, err)

	ret, _, err := evm.Call(caller, contractAddr, input, stateDb.GetBalance(userLiubei).Uint64(), big.NewInt(0), false)

	ensure.Nil(t, err)
	ensure.SameElements(t, ret, crypto.Sha256([]byte(data)))
}

/*
	pragma solidity ^0.4.25;

	contract FallbackDemo{

		event FallbackCalled(bytes data);

		function() public{
			emit FallbackCalled(msg.data);
		}

		function callNonExistFunc() public returns(bool){
			bytes4 funcIdentifier = bytes4(keccak256("functionNotExist()"));
			return address(this).call(funcIdentifier);
		}
	}
*/
func TestFallback(t *testing.T) {

	bindata := "608060405234801561001057600080fd5b506101b6806100206000396000f300608060405260043610610041576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806366b0bae0146100a2575b34801561004d57600080fd5b507f17c1956f6e992470102c5fc953bf560fda31fabee8737cf8e77bdde00eb5698d600036604051808060200182810382528484828181526020019250808284378201915050935050505060405180910390a1005b3480156100ae57600080fd5b506100b76100d1565b604051808215151515815260200191505060405180910390f35b60008060405180807f66756e6374696f6e4e6f744578697374282900000000000000000000000000008152506012019050604051809103902090503073ffffffffffffffffffffffffffffffffffffffff16817c010000000000000000000000000000000000000000000000000000000090046040518163ffffffff167c01000000000000000000000000000000000000000000000000000000000281526004016000604051808303816000875af192505050915050905600a165627a7a7230582015d37b8fcace69f559186bc765cf1c7b94c7dac371b50a84e709c876594790240029"
	jsondata := "[{\"constant\":false,\"inputs\":[],\"name\":\"callNonExistFunc\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"fallback\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"FallbackCalled\",\"type\":\"event\"}]"

	contractAddr, abiObj, evm, stateDb := deploy(t, bindata, jsondata, 0)

	input, err := abiObj.Pack("callNonExistFunc")
	ensure.Nil(t, err)

	ret, _, err := evm.Call(caller, contractAddr, input, stateDb.GetBalance(userLiubei).Uint64(), big.NewInt(0), false)
	ensure.Nil(t, err)

	rett, err := abi.UnpackByTypeName("bool", ret)
	ensure.Nil(t, err)
	ensure.True(t, rett.(bool))
	ensure.DeepEqual(t, len(stateDb.Logs()), 1)
}

/*
	pragma solidity ^0.4.25;
	contract Child {
		uint public x;
		uint public amount;

		constructor(uint _a) public payable {
			x = _a;
			amount = msg.value;
		}
	}

	contract Parent {
		event e(uint x, uint amount, address addr);

		Child d = new Child(4);

		constructor(uint _u) public payable {
			emit e(d.x(), d.amount(), address(d));
			Child d1 = new Child(_u);
			emit e(d1.x(), d1.amount(), address(d1));
		}

		function createChild(uint _x, uint _amount) public {
			Child d2 = (new Child).value(_amount)(_x);
			emit e(d2.x(), d2.amount(), address(d2));
		}
	}
*/
func TestInternalDeploy(t *testing.T) {

	bindata := "6080604052600461000e61032e565b90815260405190819003602001906000f080158015610031573d6000803e3d6000fd5b5060008054600160a060020a031916600160a060020a03929092169190911790556040516020806107848339810160408181529151600080547f0c55699c00000000000000000000000000000000000000000000000000000000845293519193909260008051602061076483398151915292600160a060020a0390921691630c55699c9160048082019260209290919082900301818887803b1580156100d657600080fd5b505af11580156100ea573d6000803e3d6000fd5b505050506040513d602081101561010057600080fd5b505160008054604080517faa8c217c0000000000000000000000000000000000000000000000000000000081529051600160a060020a039092169263aa8c217c926004808401936020939083900390910190829087803b15801561016357600080fd5b505af1158015610177573d6000803e3d6000fd5b505050506040513d602081101561018d57600080fd5b5051600054604080519384526020840192909252600160a060020a031682820152519081900360600190a1816101c161032e565b90815260405190819003602001906000f0801580156101e4573d6000803e3d6000fd5b50905060008051602061076483398151915281600160a060020a0316630c55699c6040518163ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401602060405180830381600087803b15801561024d57600080fd5b505af1158015610261573d6000803e3d6000fd5b505050506040513d602081101561027757600080fd5b5051604080517faa8c217c0000000000000000000000000000000000000000000000000000000081529051600160a060020a0385169163aa8c217c9160048083019260209291908290030181600087803b1580156102d457600080fd5b505af11580156102e8573d6000803e3d6000fd5b505050506040513d60208110156102fe57600080fd5b5051604080519283526020830191909152600160a060020a03841682820152519081900360600190a1505061033d565b60405160e58061067f83390190565b6103338061034c6000396000f3006080604052600436106100405763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416633db8abc78114610045575b600080fd5b34801561005157600080fd5b50610060600435602435610062565b005b6000818361006e610213565b908152604051908190036020019082f080158015610090573d6000803e3d6000fd5b50905090507f6cfadb96206f0b0853da8744bda159289c0f7bcd0c1528363878a96f60ee42be8173ffffffffffffffffffffffffffffffffffffffff16630c55699c6040518163ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401602060405180830381600087803b15801561011a57600080fd5b505af115801561012e573d6000803e3d6000fd5b505050506040513d602081101561014457600080fd5b5051604080517faa8c217c000000000000000000000000000000000000000000000000000000008152905173ffffffffffffffffffffffffffffffffffffffff85169163aa8c217c9160048083019260209291908290030181600087803b1580156101ae57600080fd5b505af11580156101c2573d6000803e3d6000fd5b505050506040513d60208110156101d857600080fd5b505160408051928352602083019190915273ffffffffffffffffffffffffffffffffffffffff841682820152519081900360600190a1505050565b60405160e58061022383390190560060806040526040516020806100e583398101604052516000553460015560bb8061002a6000396000f30060806040526004361060485763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416630c55699c8114604d578063aa8c217c146071575b600080fd5b348015605857600080fd5b50605f6083565b60408051918252519081900360200190f35b348015607c57600080fd5b50605f6089565b60005481565b600154815600a165627a7a72305820bbdc41b5bdb0bd7d583ec850bbaa716a14ebd20d89f3fc8ac87268e50aea74ad0029a165627a7a7230582034bffdfc13dda4259cd8b138424d3baddb42fd34fca2a4054f39ac494d273614002960806040526040516020806100e583398101604052516000553460015560bb8061002a6000396000f30060806040526004361060485763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416630c55699c8114604d578063aa8c217c146071575b600080fd5b348015605857600080fd5b50605f6083565b60408051918252519081900360200190f35b348015607c57600080fd5b50605f6089565b60005481565b600154815600a165627a7a72305820bbdc41b5bdb0bd7d583ec850bbaa716a14ebd20d89f3fc8ac87268e50aea74ad00296cfadb96206f0b0853da8744bda159289c0f7bcd0c1528363878a96f60ee42be0000000000000000000000000000000000000000000000000000000000000001"
	jsondata := "[{\"constant\":false,\"inputs\":[{\"name\":\"_x\",\"type\":\"uint256\"},{\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"createChild\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_u\",\"type\":\"uint256\"}],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"x\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"e\",\"type\":\"event\"}]"

	contractAddr, abiObj, evm, stateDb := deploy(t, bindata, jsondata, int64(1e16))
	ensure.DeepEqual(t, len(stateDb.Logs()), 2)

	input, err := abiObj.Pack("createChild", big.NewInt(2), big.NewInt(int64(1e9)))
	ensure.Nil(t, err)

	ret, _, err := evm.Call(caller, contractAddr, input, stateDb.GetBalance(userLiubei).Uint64(), big.NewInt(0), false)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, len(stateDb.Logs()), 3)
	ensure.SameElements(t, ret, []byte{})
}

/*
	pragma solidity >=0.4.22 <0.7.0;

	contract Tag {
		string sym;
		uint32 decimal;
		constructor(string memory _sym, uint32 _decimal) public {
			sym = _sym;
			decimal = _decimal;
		}

		function getSym() public view returns (string memory) {
			return sym;
		}
	}

	contract OwnedToken {
		TokenCreator creator;
		Tag tag;
		address owner;
		string name;

		constructor(string memory _name) payable public {
			owner = msg.sender;

			// We do an explicit type conversion from `address`
			// to `TokenCreator` and assume that the type of
			// the calling contract is `TokenCreator`, there is
			// no real way to check that.
			creator = TokenCreator(msg.sender);
			tag = new Tag("ABC", 2) ;
			name = _name;
		}

		function getTag() public view returns (address) {
			return address(tag);
		}

		function getBalance() public view returns (uint256) {
			return address(this).balance;
		}

		function changeName(string memory newName) public {
			if (msg.sender == address(creator))
				name = newName;
		}

		function transfer(address newOwner) public {
			if (msg.sender != owner) return;

			if (creator.isTokenTransferOK(owner, newOwner))
				owner = newOwner;
		}
	}

	contract TokenCreator {
		OwnedToken public myToken;

		constructor() payable public {}

		function createToken(string memory name)
			public
			returns (OwnedToken tokenAddress)
		{
			myToken = (new OwnedToken).value(1000)(name);
			return myToken;
		}

		function changeName(OwnedToken tokenAddress, string memory name) public {
			// Again, the external type of `tokenAddress` is
			// simply `address`.
			tokenAddress.changeName(name);
		}

		// Perform checks to determine if transferring a token to the
		// `OwnedToken` contract should proceed
		function isTokenTransferOK(address currentOwner, address newOwner)
			public
			pure
			returns (bool ok)
		{
			// Check an arbitrary condition to see if transfer should proceed
			return keccak256(abi.encodePacked(currentOwner, newOwner))[0] == 0x7f;
		}
	}
*/
func TestInternalDeploy2(t *testing.T) {

	// TokenCreator
	bindata := "6080604052610c4b806100136000396000f3006080604052600436106100615763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166345576f94811461006657806345ca25ed146100db578063b8fcf93714610144578063c3cee9c114610159575b600080fd5b34801561007257600080fd5b506040805160206004803580820135601f81018490048402850184019095528484526100bf9436949293602493928401919081908401838280828437509497506101949650505050505050565b60408051600160a060020a039092168252519081900360200190f35b3480156100e757600080fd5b5060408051602060046024803582810135601f8101859004850286018501909652858552610142958335600160a060020a03169536956044949193909101919081908401838280828437509497506102599650505050505050565b005b34801561015057600080fd5b506100bf610334565b34801561016557600080fd5b50610180600160a060020a0360043581169060243516610343565b604080519115158252519081900360200190f35b60006103e8826101a2610486565b60208082528251818301528251829160408301919085019080838360005b838110156101d85781810151838201526020016101c0565b50505050905090810190601f1680156102055780820380516001836020036101000a031916815260200191505b50925050506040518091039082f080158015610225573d6000803e3d6000fd5b506000805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a039283161790819055169392505050565b6040517f5353a2d8000000000000000000000000000000000000000000000000000000008152602060048201818152835160248401528351600160a060020a03861693635353a2d89386939283926044019185019080838360005b838110156102cc5781810151838201526020016102b4565b50505050905090810190601f1680156102f95780820380516001836020036101000a031916815260200191505b5092505050600060405180830381600087803b15801561031857600080fd5b505af115801561032c573d6000803e3d6000fd5b505050505050565b600054600160a060020a031681565b600082826040516020018083600160a060020a0316600160a060020a03166c0100000000000000000000000002815260140182600160a060020a0316600160a060020a03166c01000000000000000000000000028152601401925050506040516020818303038152906040526040518082805190602001908083835b602083106103de5780518252601f1990920191602091820191016103bf565b5181516020939093036101000a600019018019909116921691909117905260405192018290039091209250600091506104149050565b1a7f0100000000000000000000000000000000000000000000000000000000000000027effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916607f7f01000000000000000000000000000000000000000000000000000000000000000214905092915050565b6040516107898061049783390190560060806040526040516107893803806107898339810160405280516002805433600160a060020a031991821681178355600080549092161790559101906100436100da565b63ffffffff909116602082015260408082526003818301527f414243000000000000000000000000000000000000000000000000000000000060608301525190819003608001906000f08015801561009f573d6000803e3d6000fd5b5060018054600160a060020a031916600160a060020a039290921691909117905580516100d39060039060208401906100ea565b5050610185565b60405161029f806104ea83390190565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061012b57805160ff1916838001178555610158565b82800160010185558215610158579182015b8281111561015857825182559160200191906001019061013d565b50610164929150610168565b5090565b61018291905b80821115610164576000815560010161016e565b90565b610356806101946000396000f30060806040526004361061006c5763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166312065fe081146100715780631a695230146100985780635353a2d8146100bb578063893d20e814610114578063a7b72a5a14610145575b600080fd5b34801561007d57600080fd5b5061008661015a565b60408051918252519081900360200190f35b3480156100a457600080fd5b506100b9600160a060020a0360043516610160565b005b3480156100c757600080fd5b506040805160206004803580820135601f81018490048402850184019095528484526100b994369492936024939284019190819084018382808284375094975061024a9650505050505050565b34801561012057600080fd5b50610129610274565b60408051600160a060020a039092168252519081900360200190f35b34801561015157600080fd5b50610129610283565b30315b90565b600254600160a060020a0316331461017757610247565b60008054600254604080517fc3cee9c1000000000000000000000000000000000000000000000000000000008152600160a060020a03928316600482015285831660248201529051919092169263c3cee9c192604480820193602093909283900390910190829087803b1580156101ed57600080fd5b505af1158015610201573d6000803e3d6000fd5b505050506040513d602081101561021757600080fd5b505115610247576002805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a0383161790555b50565b600054600160a060020a0316331415610247578051610270906003906020840190610292565b5050565b600254600160a060020a031690565b600154600160a060020a031690565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106102d357805160ff1916838001178555610300565b82800160010185558215610300579182015b828111156103005782518255916020019190600101906102e5565b5061030c929150610310565b5090565b61015d91905b8082111561030c57600081556001016103165600a165627a7a7230582063a32c7391215b23afa9cae698ca5081f50bb5f64cc5531cbf4d896dacdeafc40029608060405234801561001057600080fd5b5060405161029f38038061029f833981016040528051602080830151919092018051909261004391600091850190610064565b506001805463ffffffff191663ffffffff92909216919091179055506100ff565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106100a557805160ff19168380011785556100d2565b828001600101855582156100d2579182015b828111156100d25782518255916020019190600101906100b7565b506100de9291506100e2565b5090565b6100fc91905b808211156100de57600081556001016100e8565b90565b6101918061010e6000396000f3006080604052600436106100405763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166312e2dbd88114610045575b600080fd5b34801561005157600080fd5b5061005a6100cf565b6040805160208082528351818301528351919283929083019185019080838360005b8381101561009457818101518382015260200161007c565b50505050905090810190601f1680156100c15780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b60008054604080516020601f600260001961010060018816150201909516949094049384018190048102820181019092528281526060939092909183018282801561015b5780601f106101305761010080835404028352916020019161015b565b820191906000526020600020905b81548152906001019060200180831161013e57829003601f168201915b50505050509050905600a165627a7a723058205b434262bbc1a2d48e2bd0bf5fbb9f736e2c147c2671f63ec1f210723f90128f0029a165627a7a723058205e1ffc0953f3ae7b5d19843038882e8a23f84b44454ec6ee94bd3246ec7357ca0029"
	jsondataTokenCreator := "[{\"constant\":false,\"inputs\":[{\"name\":\"name\",\"type\":\"string\"}],\"name\":\"createToken\",\"outputs\":[{\"name\":\"tokenAddress\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"tokenAddress\",\"type\":\"address\"},{\"name\":\"name\",\"type\":\"string\"}],\"name\":\"changeName\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"myToken\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"currentOwner\",\"type\":\"address\"},{\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"isTokenTransferOK\",\"outputs\":[{\"name\":\"ok\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"constructor\"}]"

	addrTokenCreator, abiObj, evm, stateDb := deploy(t, bindata, jsondataTokenCreator, int64(1e16))

	input, err := abiObj.Pack("createToken", "BOX")
	ensure.Nil(t, err)

	ret, _, err := evm.Call(caller, addrTokenCreator, input, stateDb.GetBalance(userLiubei).Uint64(), big.NewInt(0), false)
	ensure.Nil(t, err)

	// OwnedToken
	addrOwnedToken := coretypes.NewAddressHash(ret)
	balanceOwnedToken := stateDb.GetBalance(*addrOwnedToken)
	ensure.DeepEqual(t, balanceOwnedToken.Int64(), int64(1000))

	jsondataOwnedToken := "[{\"constant\":true,\"inputs\":[],\"name\":\"getBalance\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transfer\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newName\",\"type\":\"string\"}],\"name\":\"changeName\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getOwner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getTag\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_name\",\"type\":\"string\"}],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"constructor\"}]"
	reader := strings.NewReader(jsondataOwnedToken)
	abiObj, err = abi.JSON(reader)
	ensure.Nil(t, err)

	input, err = abiObj.Pack("getTag")
	ensure.Nil(t, err)

	ret, _, err = evm.Call(caller, *addrOwnedToken, input, stateDb.GetBalance(userLiubei).Uint64(), big.NewInt(0), false)
	ensure.Nil(t, err)
	addrTag := coretypes.NewAddressHash(ret)

	input, err = abiObj.Pack("getOwner")
	ensure.Nil(t, err)

	ret, _, err = evm.Call(caller, *addrOwnedToken, input, stateDb.GetBalance(userLiubei).Uint64(), big.NewInt(0), false)
	ensure.SameElements(t, coretypes.NewAddressHash(ret).Bytes(), addrTokenCreator.Bytes())

	// Tag
	jsondataTag := "[{\"constant\":true,\"inputs\":[],\"name\":\"getSym\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"_sym\",\"type\":\"string\"},{\"name\":\"_decimal\",\"type\":\"uint32\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]"
	reader = strings.NewReader(jsondataTag)
	abiObj, err = abi.JSON(reader)
	ensure.Nil(t, err)

	input, err = abiObj.Pack("getSym")
	ensure.Nil(t, err)

	ret, _, err = evm.Call(caller, *addrTag, input, stateDb.GetBalance(userLiubei).Uint64(), big.NewInt(0), false)
	ensure.Nil(t, err)
	rett, err := abi.UnpackByTypeName("string", ret)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, rett, "ABC")
}

func deploy(t *testing.T, bindata, jsondata string, value int64) (
	coretypes.AddressHash, abi.ABI, *vm.EVM, *state.StateDB) {
	data := hexutil.MustDecode("0x" + strings.TrimSpace(bindata))
	stateDb, err := state.New(nil, nil, initDB())
	ensure.Nil(t, err)

	msg := NewMessage(&userLiubei, big.NewInt(0))
	cc := ChainContext{}
	ctx := chain.NewEVMContext(msg, cc.GetHeader(0), blockChain)

	logConfig := vm.LogConfig{}
	structLogger := vm.NewStructLogger(&logConfig)
	vmConfig := vm.Config{Debug: true, Tracer: structLogger}

	senderbalance := int64(1e18)
	stateDb.SetBalance(userLiubei, big.NewInt(senderbalance))
	evm := vm.NewEVM(ctx, stateDb, vmConfig)
	contractRef := vm.AccountRef(userLiubei)
	_, contractAddr, balance, vmerr := evm.Create(contractRef, data, stateDb.GetBalance(userLiubei).Uint64(), big.NewInt(value), false)

	stateDb.SetBalance(userLiubei, big.NewInt(int64(balance)))
	ensure.Nil(t, vmerr)

	reader := strings.NewReader(jsondata)
	abiObj, err := abi.JSON(reader)
	ensure.Nil(t, err)
	return contractAddr, abiObj, evm, stateDb
}
