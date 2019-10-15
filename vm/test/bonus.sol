pragma solidity ^0.4.23;
pragma experimental ABIEncoderV2;

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

contract Bonus is Permission{
    using SafeMath for uint;

    uint constant DYNASTY_SIZE = 6;

    uint constant PLEDGE_THRESHOLD = 1;
    uint constant DYNASTY_CHANGE_THRESHOLD = 2;
    uint constant MIN_VOTE_BONUS_LIMIT_TO_PICK = 3;
    uint constant VOTE_FROZEN_BLOCK_NUMBER = 4;
    uint constant VOTE_THRESHOLD = 5;
    uint constant MIN_PROPOSAL_THRESHOLD = 6;
    uint constant PLEDGE_OPEN_LIMIT = 7;
    uint constant PROPOSAL_EXPIRATION_TIME = 8;
    uint constant BOOK_KEEPER_REWARD = 9;
    uint constant BONUS_TO_VOTERS = 10;
    uint constant CALC_SCORE_THRESHOLD = 11;

    struct Delegate {
        address addr;
        string peerID;
        uint votes;
        uint pledgeAmount;
        uint score;
        uint continualPeriod;
        bool isExist;
    }

    struct FrozenVote {
        uint votes;
        uint timestamp;
    }

    struct Proposal {
        uint id;
        uint value;
        uint createtime;
        address[] voters;
        bool result;
    }

    struct FrozenDelegate {
        address addr;
        uint blockNumber;
        uint pledgeAmount;
    }

    Delegate[] dynasty;
    // Delegate[] delegates;
    address[] pledgeAddrList;
    mapping(address => Delegate) addrToDelegates;
    mapping(address => Delegate) addrToDynasty;

    mapping(address => mapping(address => uint)) votes;
    mapping(address => mapping(address => uint)) delegateVotesDetail;
    mapping(address => address[]) delegateToVoters;

    mapping(address => mapping(address => uint)) currentDelegateVotesDetail;
    mapping(address => address[]) currentDelegateToVoters;

    mapping(address => mapping(address => FrozenVote)) frozenVotes;

    mapping(address => uint) voteBonus;
    mapping(address => uint) dynastyToBonus;

    mapping(address => FrozenDelegate) frozenDelegate;

    mapping(uint => Proposal) proposals;
    Proposal[] proposalList;
    mapping(uint => uint) netParams;
    uint[] changedProposalIDs;
    mapping(uint => uint) changedProposals;

    Delegate[] current;
    Delegate[] next;


    uint pledgePool;
    // uint dynastyBonusPool;
    // uint voteBonusPool;

    uint _global_open_pledge_limit;

    event ExecBonus();
    event CalcBonus(address _coinbase, uint value);

    constructor() public {
        _global_open_pledge_limit = 100;
        initNetParams();
        initDynasty();
        admin = msg.sender;
    }

     modifier onlyPledgeIsOpen {
        require(block.number > _global_open_pledge_limit, "you can not do it util the pledge is open.");
        _;
    }

    modifier onlyDynasty {
        require(IsInDynasty(msg.sender), "only dynasty can do it.");
        _;
    }

    function initAdmin(address _admin) public {
        require(block.number == 0, "init admin out of genesis block");
        admin = _admin;
    }

    function initNetParams() internal {
        netParams[PLEDGE_THRESHOLD] = 1800000 * 10**8;
        netParams[DYNASTY_CHANGE_THRESHOLD] = 250;
        netParams[MIN_VOTE_BONUS_LIMIT_TO_PICK] = 1 * 10**8;
        netParams[VOTE_FROZEN_BLOCK_NUMBER] = 50;
        netParams[VOTE_THRESHOLD] = 1 * 10**8;
        netParams[MIN_PROPOSAL_THRESHOLD] = 100 * 10**8;
        netParams[PLEDGE_OPEN_LIMIT] = 100;
        netParams[PROPOSAL_EXPIRATION_TIME] = 3 * 24 * 3600;
        netParams[BOOK_KEEPER_REWARD] = 2.85 * 10**8;
        netParams[BONUS_TO_VOTERS] = 50;
        netParams[CALC_SCORE_THRESHOLD] = 200;
    }

    function initDynasty() internal {
        dynasty.push(Delegate(0xce86056786e3415530f8cc739fb414a87435b4b6, "12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cza",
         0, 1800000 * 10**8, 0, 0, true));
        current.push(Delegate(0xce86056786e3415530f8cc739fb414a87435b4b6, "12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cza",
         0, 1800000 * 10**8, 0, 0, true));
        dynasty.push(Delegate(0x50570cc73bb18a51fc4153eec68d21d1105d326e, "12D3KooWKPRAK7vBBrVv9szEin55kBnJEEuHG4gDTQEM72ByZDpA",
         0, 1800000 * 10**8, 0, 0, true));
        current.push(Delegate(0x50570cc73bb18a51fc4153eec68d21d1105d326e, "12D3KooWKPRAK7vBBrVv9szEin55kBnJEEuHG4gDTQEM72ByZDpA",
         0, 1800000 * 10**8, 0, 0, true));
        dynasty.push(Delegate(0xae3e96d008658db64dd4f8df2d736edbc6be1c31, "12D3KooWSdXLNeoRQQ2a7yiS6xLpTn3LdCr8B8fqPz94Bbi7itsi",
         0, 1800000 * 10**8, 0, 0, true));
        current.push(Delegate(0xae3e96d008658db64dd4f8df2d736edbc6be1c31, "12D3KooWSdXLNeoRQQ2a7yiS6xLpTn3LdCr8B8fqPz94Bbi7itsi",
         0, 1800000 * 10**8, 0, 0, true));
        dynasty.push(Delegate(0x064b377c9555b83a43d05c773cef7c3a6209154f, "12D3KooWRHVAwymCVcA8jqyjpP3r3HBkCW2q5AZRTBvtaungzFSJ",
         0, 1800000 * 10**8, 0, 0, true));
        current.push(Delegate(0x064b377c9555b83a43d05c773cef7c3a6209154f, "12D3KooWRHVAwymCVcA8jqyjpP3r3HBkCW2q5AZRTBvtaungzFSJ",
         0, 1800000 * 10**8, 0, 0, true));
        dynasty.push(Delegate(0x3e8821fa1b0f9fef5aaf3e1bb5879bf36772c258, "12D3KooWQSaxCgbWakLcU69f4gmNFMszwhyHbwx4xPAhV7erDC2P",
         0, 1800000 * 10**8, 0, 0, true));
        current.push(Delegate(0x3e8821fa1b0f9fef5aaf3e1bb5879bf36772c258, "12D3KooWQSaxCgbWakLcU69f4gmNFMszwhyHbwx4xPAhV7erDC2P",
         0, 1800000 * 10**8, 0, 0, true));
        dynasty.push(Delegate(0x7f7c5668923236d74334651f731aac5dbc69421b, "12D3KooWNcJQzHaNpW5vZDQbTcoLXVCyGS755hTpendGzb5Hqtcu",
         0, 1800000 * 10**8, 0, 0, true));
        current.push(Delegate(0x7f7c5668923236d74334651f731aac5dbc69421b, "12D3KooWNcJQzHaNpW5vZDQbTcoLXVCyGS755hTpendGzb5Hqtcu",
         0, 1800000 * 10**8, 0, 0, true));

        for(uint i = 0; i < current.length; i++) {
            pledgeAddrList.push(current[i].addr);
            addrToDelegates[current[i].addr] = current[i];
            addrToDynasty[dynasty[i].addr] = dynasty[i];
        }
    }

    function  pledge(string peerID) public payable onlyPledgeIsOpen{
        require(block.number % netParams[DYNASTY_CHANGE_THRESHOLD] <= netParams[PLEDGE_OPEN_LIMIT], "pledge is not open.");
        require(msg.value >= netParams[PLEDGE_THRESHOLD], "pledge amount is not correct.");
        require(addrToDelegates[msg.sender].isExist == false, "can not repeat the mortgage");
        require(frozenDelegate[msg.sender].pledgeAmount == 0, "can not repeat the mortgage");

        Delegate memory delegate = Delegate(msg.sender, peerID, 0, msg.value, 0, 0, true);
        addrToDelegates[msg.sender] = delegate;
        pledgeAddrList.push(msg.sender);
    }

    function myPledge() public view returns (uint) {
        return addrToDelegates[msg.sender].pledgeAmount;
    }

    function redeemPledgeApply() public {
        require(block.number % netParams[DYNASTY_CHANGE_THRESHOLD] <= netParams[PLEDGE_OPEN_LIMIT], "redeem pledge apply is not allowed.");
        require(addrToDelegates[msg.sender].isExist == true, "not delegate node.");
        FrozenDelegate memory fd = FrozenDelegate(msg.sender, block.number, addrToDelegates[msg.sender].pledgeAmount);
        frozenDelegate[msg.sender] = fd;
        delete addrToDelegates[msg.sender];
        uint idx = getIdxInPledgeAddrList(msg.sender);
        deletePledgeAddrList(idx);

        for (uint i = 0; i < delegateToVoters[msg.sender].length; i++) {
            if (delegateVotesDetail[msg.sender][delegateToVoters[msg.sender][i]] > 0) {
                voteBonus[delegateToVoters[msg.sender][i]] = voteBonus[delegateToVoters[msg.sender][i]].
                add(delegateVotesDetail[msg.sender][delegateToVoters[msg.sender][i]]);
            }
        }
        delete delegateToVoters[msg.sender];
    }

    function pickRedeemPledge() public {
        require(frozenDelegate[msg.sender].blockNumber > 0, "not frozen delegate node.");
        require(block.number > ((frozenDelegate[msg.sender].blockNumber / netParams
        [DYNASTY_CHANGE_THRESHOLD]) + 1) * netParams[DYNASTY_CHANGE_THRESHOLD], "no time to pick redeem pledge.");
        FrozenDelegate memory fd = frozenDelegate[msg.sender];
        delete frozenDelegate[msg.sender];
        msg.sender.transfer(fd.pledgeAmount);
    }

    function calcBonus() public payable {
        require(msg.sender == block.coinbase, "only coinbase can do it.");

        // dynastyBonusPool = dynastyBonusPool.add(msg.value/2);
        // voteBonusPool = voteBonusPool.add(msg.value/2);
        dynastyToBonus[msg.sender] = dynastyToBonus[msg.sender].add(msg.value * (100 - netParams[BONUS_TO_VOTERS])/100);

        for (uint i = 0; i < currentDelegateToVoters[msg.sender].length; i++) {
            uint vote = currentDelegateVotesDetail[msg.sender][currentDelegateToVoters[msg.sender][i]];
            voteBonus[currentDelegateToVoters[msg.sender][i]] = voteBonus[currentDelegateToVoters[msg.sender][i]].
            add((msg.value * netParams[BONUS_TO_VOTERS] / 100) * vote/addrToDynasty[msg.sender].votes);
        }
        emit CalcBonus(msg.sender, msg.value);
    }

    function execBonus() public onlyAdmin {

        require((block.number+1) % netParams[DYNASTY_CHANGE_THRESHOLD] == 0, "Not the time to switch dynasties");
        for(uint i = 0; i < dynasty.length; i++) {
            if (dynastyToBonus[dynasty[i].addr] > 0) {
                dynasty[i].addr.transfer(dynastyToBonus[dynasty[i].addr]);
            }
            delete dynastyToBonus[dynasty[i].addr];
        }
        current = next;

        for(i = 0; i < pledgeAddrList.length; i++) {
            address pledgeAddr = pledgeAddrList[i];
            address[] memory voters = delegateToVoters[pledgeAddr];
            if(voters.length > 0) {
                currentDelegateToVoters[pledgeAddr] = voters;
                for(uint j = 0; j < voters.length; j++) {
                    currentDelegateVotesDetail[pledgeAddr][voters[j]] = delegateVotesDetail[pledgeAddr][voters[j]];
                }
            }
        }
        if (pledgeAddrList.length >= DYNASTY_SIZE) {
            updateDynasty();
            updateNetParams();
        }
    }

    function calcScore(uint[] scores) public  {
        for(uint i = 0; i < scores.length; i++) {
            Delegate storage delegate = addrToDelegates[pledgeAddrList[i]];
            delegate.score = scores[i];
        }
        delete next;
        Delegate[] memory sortAux = new Delegate[](pledgeAddrList.length);
        for(i = 0; i < pledgeAddrList.length; i++) {
            if (addrToDelegates[pledgeAddrList[i]].isExist) {
                next.push(addrToDelegates[pledgeAddrList[i]]);
            }
        }
        sort(next, sortAux, 0, next.length-1);
    }

    function getDynasty() public view returns (Delegate[] memory) {
        return dynasty;
    }

    function getNext() public view returns (Delegate[] memory) {
        return next;
    }

    function getCurrentDelegates() public view returns (Delegate[] memory) {
        return current;
    }

    function getNextDelegates() public view returns (Delegate[] memory) {
        Delegate[] memory res = new Delegate[](pledgeAddrList.length);
        for(uint i = 0; i < pledgeAddrList.length; i++) {
            if (addrToDelegates[pledgeAddrList[i]].isExist) {
                res[i] = addrToDelegates[pledgeAddrList[i]];
            }
        }
        return res;
    }

    function updateDynasty() internal {
        for (uint j = 0; j < dynasty.length; j++) {
            dynasty[j] = next[j];
            addrToDynasty[next[j].addr] = next[j];
        }
    }

    function updateNetParams() internal {
        for(uint i = 0; i < changedProposalIDs.length; i++) {
            netParams[changedProposalIDs[i]] = changedProposals[changedProposalIDs[i]];
            delete changedProposals[changedProposalIDs[i]];
        }
        delete changedProposalIDs;
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

    function IsInDynasty(address addr) internal view returns (bool) {
        for (uint i = 0; i < dynasty.length; i++) {
            if (addr == dynasty[i].addr) {
                return true;
            }
        }
        return false;
    }

    function addressIsExist(address addr, address[] addrs) internal pure returns (bool) {
        for (uint i = 0; i < addrs.length; i++) {
            if (addr == addrs[i]) {
                return true;
            }
        }
        return false;
    }

    function sort(Delegate[] storage arr, Delegate[] aux, uint lo, uint hi) internal {
        if (lo == hi || lo == hi+1)
          return;
        // top-down sort
        uint mid = lo + (hi - lo) / 2;
        sort(arr, aux, lo, mid);
        sort(arr, aux, mid+1, hi);
        // merge
        for (uint k = lo; k <= hi; k++) {
          aux[k] = arr[k];
        }
        uint i = lo;
        uint j = mid + 1;
        for (k = lo; k <= hi; k++) {
            if      (i > mid)                     arr[k] = aux[j++];
            else if (j > hi)                      arr[k] = aux[i++];
            else if (aux[j].score > aux[i].score) arr[k] = aux[j++];
            else                                  arr[k] = aux[i++];
        }
    }

    function vote(address delegateAddr) public payable {
        require(msg.value >= netParams[MIN_VOTE_BONUS_LIMIT_TO_PICK], "vote amount is not correct.");
        require(addrToDelegates[delegateAddr].pledgeAmount >= netParams[PLEDGE_THRESHOLD], "delegate is not exist.");
        require(block.number % netParams[DYNASTY_CHANGE_THRESHOLD] > netParams[PLEDGE_OPEN_LIMIT] &&
         block.number % netParams[DYNASTY_CHANGE_THRESHOLD] < netParams[CALC_SCORE_THRESHOLD], "Out of voting hours");

        if (votes[msg.sender][delegateAddr] > 0) {
            votes[msg.sender][delegateAddr] = votes[msg.sender][delegateAddr].add(msg.value);
        } else {
            votes[msg.sender][delegateAddr] = msg.value;
        }

        Delegate storage delegate = addrToDelegates[delegateAddr];
        delegate.votes = delegate.votes.add(msg.value);
        // delegate.score = calcScore(delegate);
        if (delegateVotesDetail[delegateAddr][msg.sender] > 0) {
            delegateVotesDetail[delegateAddr][msg.sender] = delegateVotesDetail[delegateAddr][msg.sender].add(msg.value);
        } else {
            delegateVotesDetail[delegateAddr][msg.sender] = msg.value;
            delegateToVoters[delegateAddr].push(msg.sender);
        }
    }

    function redeemVoteApply(address delegateAddr, uint count) public {
        require(block.number % netParams[DYNASTY_CHANGE_THRESHOLD] > netParams[PLEDGE_OPEN_LIMIT] &&
         block.number % netParams[DYNASTY_CHANGE_THRESHOLD] < netParams[CALC_SCORE_THRESHOLD], "Out of voting hours");

        require(count <= delegateVotesDetail[delegateAddr][msg.sender], "the vote count is not enough.");
        require(count <= votes[msg.sender][delegateAddr], "the vote count is not enough.");
        require(count >= netParams[MIN_VOTE_BONUS_LIMIT_TO_PICK], "the redeem vote is too small.");
        frozenVotes[delegateAddr][msg.sender].votes = frozenVotes[delegateAddr][msg.sender].votes.add(count);
        frozenVotes[delegateAddr][msg.sender].timestamp = block.number;

        delegateVotesDetail[delegateAddr][msg.sender] = delegateVotesDetail[delegateAddr][msg.sender].sub(count);
        votes[msg.sender][delegateAddr] = votes[msg.sender][delegateAddr].sub(count);
        Delegate storage delegate = addrToDelegates[delegateAddr];
        delegate.votes = delegate.votes.sub(count);
        // delegate.score = calcScore(delegate);
    }

    function myRedeemVote(address delegateAddr) public view returns (uint, uint) {
        if (frozenVotes[delegateAddr][msg.sender].votes > 0 &&
            block.number > (frozenVotes[delegateAddr][msg.sender].timestamp + netParams[VOTE_FROZEN_BLOCK_NUMBER])) {
            return (0, frozenVotes[delegateAddr][msg.sender].votes);
        } else {
            return (frozenVotes[delegateAddr][msg.sender].votes, 0);
        }
    }

    function pickRedeemVote(address delegateAddr) public {
        require(frozenVotes[delegateAddr][msg.sender].votes > netParams[MIN_VOTE_BONUS_LIMIT_TO_PICK] &&
            block.number > (frozenVotes[delegateAddr][msg.sender].timestamp + netParams[VOTE_FROZEN_BLOCK_NUMBER]), "");
        uint voteNumber = frozenVotes[delegateAddr][msg.sender].votes;
        delete frozenVotes[delegateAddr][msg.sender];
        msg.sender.transfer(voteNumber);
    }

    function myVoteBonus() public view returns (uint) {
        return voteBonus[msg.sender];
    }

    function pickVoteBonus() public {
        require(voteBonus[msg.sender] > netParams[MIN_VOTE_BONUS_LIMIT_TO_PICK], "you don`t have enough vote bonus.");
        uint bonus = voteBonus[msg.sender];
        delete voteBonus[msg.sender];
        msg.sender.transfer(bonus);
    }

    function myVote(address delegate) public view returns (uint) {
        return votes[msg.sender][delegate];
    }

    function giveProposal(uint proposalID, uint value) public onlyDynasty payable{
        require(msg.value >= netParams[MIN_PROPOSAL_THRESHOLD] && proposalID > 0, "Insufficient minimum fee for give proposal.");
        require(proposals[proposalID].id == 0 ||
         (block.number - proposals[proposalID].createtime > netParams[PROPOSAL_EXPIRATION_TIME]), "the proposal is exist.");
        Proposal memory proposal = Proposal(proposalID, value, block.number, new address[](0), false);
        proposals[proposalID] = proposal;
        proposalList.push(proposal);
    }

    function voteProposal(uint proposalID) public onlyDynasty{
        require(proposalID > 0, "proposalID is not legal.");
        Proposal storage proposal = proposals[proposalID];
        require(proposal.id == proposalID &&
         (block.number - proposal.createtime <= netParams[PROPOSAL_EXPIRATION_TIME]) && proposal.result == false, "the proposal is not exist.");
        require(addressIsExist(msg.sender, proposal.voters) == false, "Repeated voting is forbidden.");
        proposal.voters.push(msg.sender);
        if (proposal.voters.length > 2 * DYNASTY_SIZE / 3) {
            proposal.result = true;
            changedProposals[proposalID] = proposal.value;
            changedProposalIDs.push(proposalID);
        }
    }

    function getProposal() public view returns (Proposal[] memory) {
        return proposalList;
    }

    function setGlobalOpenPledgeLimit(uint value) public onlyAdmin {
        _global_open_pledge_limit = value;
    }

    function getNetParams() public view returns (uint,uint,uint) {
        return (netParams[DYNASTY_CHANGE_THRESHOLD], netParams[BOOK_KEEPER_REWARD], netParams[CALC_SCORE_THRESHOLD]);
    }
}