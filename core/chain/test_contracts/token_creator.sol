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
        tag = new Tag("ABC", 2);
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
