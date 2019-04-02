pragma solidity ^0.4.25;

contract B {
    int public x;

    event senderAddr(address);
    function inc() public {
        x++;
        emit senderAddr(msg.sender);
    }
}
