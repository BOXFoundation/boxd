pragma solidity ^0.4.25;

contract A {
    int public x;
    
    function inc_call(address _contractAddress) public {
        _contractAddress.call(bytes4(keccak256("inc()")));
    }
    function inc_callcode(address _contractAddress) public {
        _contractAddress.callcode(bytes4(keccak256("inc()")));
    }
    function inc_delegatecall(address _contractAddress) public {
        _contractAddress.delegatecall(bytes4(keccak256("inc()")));
    }
}
