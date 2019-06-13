pragma solidity ^0.4.23;

library SafeMath {
    function add(uint a, uint b) internal pure returns (uint c) {
        c = a + b;
        require(c >= a, "unexpect error in add.");
    }
    function sub(uint a, uint b) internal pure returns (uint c) {
        require(b <= a, "unexpect error in sub.");
        c = a - b;
    }
    function mul(uint a, uint b) internal pure returns (uint c) {
        c = a * b;
        require(a == 0 || c / a == b, "unexpect error in mul.");
    }
    function div(uint a, uint b) internal pure returns (uint c) {
        require(b > 0, "unexpect error in div.");
        c = a / b;
    }
}

contract Permission {
    address public admin;
    address public newAdmin;

    event AdminTransferred(address indexed _from, address indexed _to);

    constructor() public {
        admin = msg.sender;
    }

    modifier onlyAdmin {
        require(msg.sender == admin, "only admin can do it.");
        _;
    }

    function transferAdmin(address _newAdmin) public onlyAdmin {
        newAdmin = _newAdmin;
    }
    function acceptAdmin() public {
        require(msg.sender == newAdmin, "only new admin can do it.");
        emit AdminTransferred(admin, newAdmin);
        admin = newAdmin;
        newAdmin = address(0);
    }
}

contract DelegateBonus is Permission{
    using SafeMath for uint;

    struct Delegate {
        address addr;
        uint votes;
        uint pledgeAmount;
        uint score;
        bool isExist;
    }

    struct FrozenVote {
        uint votes;
        uint height;
    }

    Delegate[15] dynasty;
    Delegate[] delegates;
    address[] pledgeAddrList;
    mapping(address => Delegate) addrToDelegates;

    mapping(address => mapping(address => uint)) votes;
    mapping(address => mapping(address => uint)) delegateVotesDetail;
    mapping(address => address[]) delegateToVoters;

    mapping(address => mapping(address => FrozenVote[])) frozenVotes;

    mapping(address => uint) voteBonus;
    mapping(address => uint) dynastyToBonus;

    mapping(address => Delegate) frozenDelegate;


    uint pledgePool;
    uint dynastyBonusPool;
    uint voteBonusPool;

    uint _pledge_threshold;
    uint _dynasty_threshold;
    uint _vote_threshold;
    uint _min_vote_bonus_limit_to_pick;
    uint _vote_frozen_block_number;

    event ExecBonus();
    event CalcBonus(address _coinbase, uint value);

    constructor() public{
        _pledge_threshold = 1800000 * 10**8;
        _dynasty_threshold = 10000;
        _min_vote_bonus_limit_to_pick = 1 * 10**8;
        _vote_frozen_block_number = 2000;
    }

    function initAdmin(address _admin) public {
        require(block.number == 0, "init admin out of genesis block");
        admin = _admin;
    }

    function  pledge() public payable {
        require(msg.value < _pledge_threshold, "pledge amount is not correct.");
        require(addrToDelegates[msg.sender].isExist == false, "can not repeat the mortgage");

        Delegate memory delegate = Delegate(msg.sender, 0, msg.value, 0, true);
        delegate.score = calcScore(delegate);
        addrToDelegates[msg.sender] = delegate;
        pledgePool = pledgePool.add(msg.value);
        pledgeAddrList.push(msg.sender);
    }

    function redeemPledgeApply() public {
        require(addrToDelegates[msg.sender].isExist == true, "not delegate node.");
        frozenDelegate[msg.sender] = addrToDelegates[msg.sender];
        delete addrToDelegates[msg.sender];
        uint idx = getIdxInPledgeAddrList(msg.sender);
        deletePledgeAddrList(idx);
    }

    function redeemPledge() public {
        require(frozenDelegate[msg.sender].isExist == true, "not frozen delegate node.");
        if (block.number > ((block.number / _dynasty_threshold) + 1) * _dynasty_threshold) {
                msg.sender.transfer(addrToDelegates[msg.sender].pledgeAmount);
                delete frozenDelegate[msg.sender];
                for (uint i = 0; i < delegateToVoters[msg.sender].length; i++) {
                    if (delegateVotesDetail[msg.sender][delegateToVoters[msg.sender][i]] > 0) {
                        delegateToVoters[msg.sender][i].transfer(delegateVotesDetail[msg.sender][delegateToVoters[msg.sender][i]]);
                        delete delegateToVoters[msg.sender];
                    }
                }
        }
    }

    function redeemVoteApply(address delegateAddr, uint count) public {
        require(count <= delegateVotesDetail[delegateAddr][msg.sender], "the vote count is not enough.");
        require(count <= votes[msg.sender][delegateAddr], "the vote count is not enough.");
        FrozenVote memory frozenVote = FrozenVote(count, block.number);
        frozenVotes[delegateAddr][msg.sender].push(frozenVote);

        delegateVotesDetail[delegateAddr][msg.sender] = delegateVotesDetail[delegateAddr][msg.sender].sub(count);
        votes[msg.sender][delegateAddr] = votes[msg.sender][delegateAddr].sub(count);
        Delegate storage delegate = addrToDelegates[delegateAddr];
        delegate.votes = delegate.votes.sub(count);
        delegate.score = calcScore(delegate);
    }

    function redeemVote(address delegateAddr) public {
        for (uint i = 0; i < frozenVotes[delegateAddr][msg.sender].length; i++) {
            if (frozenVotes[delegateAddr][msg.sender][i].votes > 0 &&
            block.number > (frozenVotes[delegateAddr][msg.sender][i].height + _vote_frozen_block_number)) {
                msg.sender.transfer(frozenVotes[delegateAddr][msg.sender][i].votes);
                deleteFrozenVote(frozenVotes[delegateAddr][msg.sender], i);
            }
        }
    }

    function calcBonus() public payable {
        require(msg.sender == block.coinbase, "only coinbase can do it.");

        dynastyBonusPool = dynastyBonusPool.add(msg.value/2);
        voteBonusPool = voteBonusPool.add(msg.value/2);
        dynastyToBonus[msg.sender] = dynastyToBonus[msg.sender].add(msg.value/2);

        for (uint i = 0; i < delegateToVoters[msg.sender].length; i++) {
            uint vote = delegateVotesDetail[msg.sender][delegateToVoters[msg.sender][i]];
            voteBonus[delegateToVoters[msg.sender][i]] = voteBonus[delegateToVoters[msg.sender][i]].
            add((msg.value/2) * vote/addrToDelegates[msg.sender].votes);
        }
        emit CalcBonus(msg.sender, msg.value);
    }

    function execBonus() public {
        require(msg.sender == block.coinbase || msg.sender == admin, "Not enough permissions.");
        if (msg.sender == block.coinbase) {
            require(block.number % _dynasty_threshold == 0, "Not the time to switch dynasties");
        }

        for(uint i = 0; i < dynasty.length; i++) {
            if (dynastyToBonus[dynasty[i].addr] > 0) {
                dynasty[i].addr.transfer(dynastyToBonus[dynasty[i].addr]);
            }
        }
        updateDynasty();
    }

    function getDynasty() public view returns (address[15] memory) {
        address[15] memory addrs;
        for (uint i = 0; i < dynasty.length; i++) {
            addrs[i] = dynasty[i].addr;
        }
        return addrs;
    }

    function calcScore(Delegate memory delegate)  internal pure returns (uint) {
        // TODO: wait correct calculation formula.
        return delegate.votes;
    }

    function updateDynasty() internal {
        require(pledgeAddrList.length > 15, "");
        delete delegates;
        delete dynasty;

        for(uint i = 0; i < pledgeAddrList.length; i++) {
            if (addrToDelegates[pledgeAddrList[i]].isExist) {
                delegates.push(addrToDelegates[pledgeAddrList[i]]);
            }
        }
        quickSort(delegates, 0, delegates.length);
        for (uint j = 0; j < dynasty.length; j++) {
            dynasty[j] = delegates[j];
        }
    }

    function deleteFrozenVote(FrozenVote[] storage array, uint index) internal{
        uint len = array.length;
        if (index >= len) return;
        for (uint i = index; i<len-1; i++) {
            array[i] = array[i+1];
        }
        delete array[len-1];
        array.length--;
    }

    function getIdxInPledgeAddrList(address addr) internal view returns (uint) {
        for (uint i = 0; i < pledgeAddrList.length; i++) {
            if (addr == pledgeAddrList[i]) {
                return i;
            }
        }
    }

    function deletePledgeAddrList(uint index) internal{
        uint len = pledgeAddrList.length;
        if (index >= len) return;
        for (uint i = index; i<len-1; i++) {
            pledgeAddrList[i] = pledgeAddrList[i+1];
        }
        delete pledgeAddrList[len-1];
        pledgeAddrList.length--;
    }

    // function deleteDelegate(Delegate[] storage array, uint index) internal{
    //     uint len = array.length;
    //     if (index >= len) return;
    //     for (uint i = index; i<len-1; i++) {
    //         array[i] = array[i+1];
    //     }
    //     delete array[len-1];
    //     array.length--;
    // }

    function quickSort(Delegate[] storage arr, uint left, uint right) internal {
        uint i = left;
        uint j = right;
        uint pivot = arr[left + (right - left) / 2].score;
        while (i <= j) {
            while (arr[i].score < pivot) i++;
            while (pivot < arr[j].score) j--;
            if (i <= j) {
                // (arr[i], arr[j]) = (arr[j], arr[i]);
                Delegate memory temp = arr[i];
                arr[i] = arr[j];
                arr[j] = temp;
                i++;
                j--;
            }
        }
        if (left < j)
            quickSort(arr, left, j);
        if (i < right)
            quickSort(arr, i, right);
    }

    function vote(address delegateAddr) public payable {
        require(msg.value > _vote_threshold, "vote amount is not correct.");

        if (votes[msg.sender][delegateAddr] > 0) {
            votes[msg.sender][delegateAddr] = votes[msg.sender][delegateAddr].add(msg.value);
        } else {
            votes[msg.sender][delegateAddr] = msg.value;
        }

        Delegate storage delegate = addrToDelegates[delegateAddr];
        delegate.votes = delegate.votes.add(msg.value);
        delegate.score = calcScore(delegate);
        if (delegateVotesDetail[delegateAddr][msg.sender] > 0) {
            delegateVotesDetail[delegateAddr][msg.sender] = delegateVotesDetail[delegateAddr][msg.sender].add(msg.value);
        } else {
            delegateVotesDetail[delegateAddr][msg.sender] = msg.value;
            delegateToVoters[delegateAddr].push(msg.sender);
        }
    }

    function pickVoteBonus() public {
        require(voteBonus[msg.sender] > _min_vote_bonus_limit_to_pick, "you don`t have enough vote bonus.");
        delete voteBonus[msg.sender];
        msg.sender.transfer(voteBonus[msg.sender]);
    }

    function myVote(address delegate) public view returns (uint) {
        return votes[msg.sender][delegate];
    }

    function setThreshold(uint threshold) public onlyAdmin {
        _pledge_threshold = threshold;
    }

    function setDynastyThreshold(uint dynasty_threshold) public onlyAdmin {
        _dynasty_threshold = dynasty_threshold;
    }

    function setMinVoteBonusLimitToPick(uint min_vote_bonus_limit_to_pick) public onlyAdmin {
        _min_vote_bonus_limit_to_pick = min_vote_bonus_limit_to_pick;
    }

    function setVoteFrozenBlockNumber(uint vote_frozen_block_number) public onlyAdmin {
        _vote_frozen_block_number = vote_frozen_block_number;
    }
}