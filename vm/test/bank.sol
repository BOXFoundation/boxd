pragma solidity ^0.4.23;

library SafeMath {
    function add(uint a, uint b) internal pure returns (uint c) {
        c = a + b;
        require(c >= a, "");
    }
    function sub(uint a, uint b) internal pure returns (uint c) {
        require(b <= a, "");
        c = a - b;
    }
    function mul(uint a, uint b) internal pure returns (uint c) {
        c = a * b;
        require(a == 0 || c / a == b, "");
    }
    function div(uint a, uint b) internal pure returns (uint c) {
        require(b > 0, "");
        c = a / b;
    }
}

contract Token {
    using SafeMath for uint;

    string public symbol;
    string public name;
    uint8 public decimals;
    uint _totalSupply;

    mapping(address => uint) balances;
    mapping(address => mapping(address => uint)) allowed;

    event Transfer(address indexed from, address indexed to, uint tokens);

    constructor() public{
        symbol = "dudu";
        name = "ContentBox";
        decimals = 6;
        _totalSupply = 10000000000 * 10**uint(decimals);
        balances[msg.sender] = _totalSupply;
        emit Transfer(address(0), msg.sender, _totalSupply);
    }

    function totalSupply() public view returns (uint) {
        return _totalSupply.sub(balances[address(0)]);
    }

    function balanceOf(address tokenOwner) public view returns (uint balance) {
        return balances[tokenOwner];
    }

    function transfer(address to, uint tokens) public returns (bool success) {
        balances[msg.sender] = balances[msg.sender].sub(tokens);
        balances[to] = balances[to].add(tokens);
        emit Transfer(msg.sender, to, tokens);
        return true;
    }

    function approve(address spender, uint tokens) public returns (bool success) {
        allowed[msg.sender][spender] = tokens;
        return true;
    }

    function transferFrom(address from, address to, uint tokens) public returns (bool success) {
        balances[from] = balances[from].sub(tokens);
        allowed[from][msg.sender] = allowed[from][msg.sender].sub(tokens);
        balances[to] = balances[to].add(tokens);
        emit Transfer(from, to, tokens);
        return true;
    }

    function allowance(address tokenOwner, address spender) public view returns (uint remaining) {
        return allowed[tokenOwner][spender];
    }
}

contract Bank {
    using SafeMath for uint;

    Token token;

    constructor(address _tokenAddress) public {
        token = Token(_tokenAddress);
    }

    function recharge() public payable {
        require(msg.value > 0, "");
        token.transfer(msg.sender, msg.value);
    }
}
