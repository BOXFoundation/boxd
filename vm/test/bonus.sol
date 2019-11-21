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
    uint constant NET_PARAMS_LENGTH = 14;

    uint constant PLEDGE_THRESHOLD = 1;
    uint constant DYNASTY_CHANGE_THRESHOLD = 2;
    uint constant MIN_VOTE_BONUS_LIMIT_TO_PICK = 3;
    uint constant VOTE_FROZEN_BLOCK_NUMBER = 4;
    uint constant VOTE_THRESHOLD = 5;
    uint constant PROPOSAL_EXPENDITURE = 6;
    uint constant PLEDGE_OPEN_LIMIT = 7;
    uint constant PROPOSAL_EXPIRATION_TIME = 8;
    uint constant BOOK_KEEPER_REWARD = 9;
    uint constant BONUS_TO_VOTERS = 10;
    uint constant CALC_SCORE_THRESHOLD = 11;
    uint constant BLOCK_REWARD = 12;
    uint constant FREE_TO_REPORT_EVIL = 13;
    uint constant THE_MINIMUM_OUTPUT_OVER_EVIL = 14;

    uint constant BOX = 1 * 10 ** 8;

    struct Delegate {
        address addr;
        string peerID;
        uint votes;
        uint pledgeAmount;
        uint score;
        uint continualPeriod;
        uint curDynastyOutputNumber;
        uint totalOutputNumber;
    }

    struct FrozenVote {
        uint votes;
        uint timestamp;
    }

    struct Proposal {
        uint id;
        address proposer;
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

    address[] dynasty;
    address[] lastEpochAddrs;
    mapping(address => Delegate) addrToLastEpoch;

    address[] epochAddrs;
    mapping(address => Delegate) addrToEpoch;
    mapping(address => uint) OutputNumberHistory;

    mapping(address => mapping(address => uint)) votes;
    mapping(address => mapping(address => uint)) delegateVotesDetail;
    mapping(address => address[]) delegateToVoters;

    mapping(address => mapping(address => uint)) currentDelegateVotesDetail;
    mapping(address => address[]) currentDelegateToVoters;

    mapping(address => mapping(address => FrozenVote)) frozenVotes;

    mapping(address => uint) voteBonus;
    mapping(address => uint) dynastyToBonus;
    mapping(address => uint) delegateToBonus;

    mapping(address => FrozenDelegate) frozenDelegate;

    mapping(uint => Proposal) proposals;
    Proposal[] proposalHistory;
    Proposal[] currentProposals;
    mapping(uint => uint) netParams;
    uint[] changedProposalIDs;
    mapping(uint => uint) changedProposals;

    uint delegateRewardTotal;

    uint _global_open_pledge_limit;
    uint _dynasty_switch_height;

    event ExecBonus(uint bonus, uint balance);
    event CalcBonus(address _coinbase, uint value, uint bonus);

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

    modifier onlyDelegate {
        require(addressIsExist(msg.sender, epochAddrs), "only delegate can do it.");
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
        netParams[PROPOSAL_EXPENDITURE] = 100 * 10**8;
        netParams[PLEDGE_OPEN_LIMIT] = 100;
        netParams[PROPOSAL_EXPIRATION_TIME] = 100;
        netParams[BOOK_KEEPER_REWARD] = 2.852 * 10**8;
        netParams[BONUS_TO_VOTERS] = 50;
        netParams[CALC_SCORE_THRESHOLD] = 200;
        netParams[BLOCK_REWARD] = 3.17 * 10**8;
        netParams[FREE_TO_REPORT_EVIL] = 100;
        netParams[THE_MINIMUM_OUTPUT_OVER_EVIL] = 100;
    }

    function initDynasty() internal {

        addrToEpoch[0xce86056786e3415530f8cc739fb414a87435b4b6] = Delegate(0xce86056786e3415530f8cc739fb414a87435b4b6,
         "12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cza", 0, 1800000 * 10**8, 0, 0, 0, 0);
        addrToEpoch[0x50570cc73bb18a51fc4153eec68d21d1105d326e] = Delegate(0x50570cc73bb18a51fc4153eec68d21d1105d326e,
         "12D3KooWKPRAK7vBBrVv9szEin55kBnJEEuHG4gDTQEM72ByZDpA", 0, 1800000 * 10**8, 0, 0, 0, 0);
        addrToEpoch[0xae3e96d008658db64dd4f8df2d736edbc6be1c31] = Delegate(0xae3e96d008658db64dd4f8df2d736edbc6be1c31,
         "12D3KooWSdXLNeoRQQ2a7yiS6xLpTn3LdCr8B8fqPz94Bbi7itsi", 0, 1800000 * 10**8, 0, 0, 0, 0);
        addrToEpoch[0x064b377c9555b83a43d05c773cef7c3a6209154f] = Delegate(0x064b377c9555b83a43d05c773cef7c3a6209154f,
         "12D3KooWRHVAwymCVcA8jqyjpP3r3HBkCW2q5AZRTBvtaungzFSJ", 0, 1800000 * 10**8, 0, 0, 0, 0);
        addrToEpoch[0x3e8821fa1b0f9fef5aaf3e1bb5879bf36772c258] = Delegate(0x3e8821fa1b0f9fef5aaf3e1bb5879bf36772c258,
         "12D3KooWQSaxCgbWakLcU69f4gmNFMszwhyHbwx4xPAhV7erDC2P", 0, 1800000 * 10**8, 0, 0, 0, 0);
        addrToEpoch[0x7f7c5668923236d74334651f731aac5dbc69421b] = Delegate(0x7f7c5668923236d74334651f731aac5dbc69421b,
         "12D3KooWNcJQzHaNpW5vZDQbTcoLXVCyGS755hTpendGzb5Hqtcu", 0, 1800000 * 10**8, 0, 0, 0, 0);

        epochAddrs = [0xce86056786e3415530f8cc739fb414a87435b4b6, 0x50570cc73bb18a51fc4153eec68d21d1105d326e,
         0xae3e96d008658db64dd4f8df2d736edbc6be1c31, 0x064b377c9555b83a43d05c773cef7c3a6209154f,
          0x3e8821fa1b0f9fef5aaf3e1bb5879bf36772c258, 0x7f7c5668923236d74334651f731aac5dbc69421b];
        lastEpochAddrs = epochAddrs;
        dynasty = epochAddrs;

        for(uint i = 0; i < epochAddrs.length; i++) {
            addrToLastEpoch[epochAddrs[i]] = addrToEpoch[epochAddrs[i]];
        }
    }

    function  pledge(string peerID) public payable onlyPledgeIsOpen{
        require(block.number % netParams[DYNASTY_CHANGE_THRESHOLD] <= netParams[PLEDGE_OPEN_LIMIT], "pledge is not open.");
        require(msg.value >= netParams[PLEDGE_THRESHOLD], "pledge amount is not correct.");
        require(frozenDelegate[msg.sender].pledgeAmount == 0, "can not repeat the mortgage");

        Delegate memory delegate = Delegate(msg.sender, peerID, 0, msg.value, 0, 0, 0, OutputNumberHistory[msg.sender]);
        addrToEpoch[msg.sender] = delegate;
        epochAddrs.push(msg.sender);
    }

    function myPledge() public view returns (uint) {
        return addrToEpoch[msg.sender].pledgeAmount;
    }

    function myFrozenDelegate() public view returns (uint, uint) {
        if(block.number > ((frozenDelegate[msg.sender].blockNumber / netParams
        [DYNASTY_CHANGE_THRESHOLD]) + 1) * netParams[DYNASTY_CHANGE_THRESHOLD]) {
            return (0, frozenDelegate[msg.sender].pledgeAmount);
        } else {
            return (frozenDelegate[msg.sender].pledgeAmount, 0);
        }
    }

    function redeemPledgeApply() public {
        require(block.number % netParams[DYNASTY_CHANGE_THRESHOLD] <= netParams[PLEDGE_OPEN_LIMIT], "redeem pledge apply is not allowed.");
        require(addrToEpoch[msg.sender].pledgeAmount >= netParams[PLEDGE_THRESHOLD], "not delegate node.");
        FrozenDelegate memory fd = FrozenDelegate(msg.sender, block.number, addrToEpoch[msg.sender].pledgeAmount);
        frozenDelegate[msg.sender] = fd;
        delete addrToEpoch[msg.sender];
        uint idx = getIdxInAddressList(epochAddrs, msg.sender);
        deleteFromAddressList(epochAddrs, idx);

        for (uint i = 0; i < delegateToVoters[msg.sender].length; i++) {
            if (delegateVotesDetail[msg.sender][delegateToVoters[msg.sender][i]] > 0) {
                voteBonus[delegateToVoters[msg.sender][i]] = voteBonus[delegateToVoters[msg.sender][i]].
                add(delegateVotesDetail[msg.sender][delegateToVoters[msg.sender][i]]);
            }
        }
        delete delegateToVoters[msg.sender];
    }

    function myPledgeDetail() public view returns (uint, uint, uint) {
        if (block.number > ((frozenDelegate[msg.sender].blockNumber/
        netParams[DYNASTY_CHANGE_THRESHOLD]) + 1) * netParams[DYNASTY_CHANGE_THRESHOLD]) {
            return (addrToEpoch[msg.sender].pledgeAmount, 0, frozenDelegate[msg.sender].pledgeAmount);
        } else {
            return (addrToEpoch[msg.sender].pledgeAmount, frozenDelegate[msg.sender].pledgeAmount, 0);
        }
    }

    function pickRedeemPledge() public {
        require(frozenDelegate[msg.sender].blockNumber > 0, "not frozen delegate node.");
        require(block.number > ((frozenDelegate[msg.sender].blockNumber / netParams
        [DYNASTY_CHANGE_THRESHOLD]) + 1) * netParams[DYNASTY_CHANGE_THRESHOLD], "no time to pick redeem pledge.");
        FrozenDelegate memory fd = frozenDelegate[msg.sender];
        delete frozenDelegate[msg.sender];
        msg.sender.transfer(fd.pledgeAmount);
    }

    function report(address devil) public onlyDelegate{

        require(addrToEpoch[devil].pledgeAmount > 0, "the devil is not exist.");
        require(block.number > _dynasty_switch_height, "system error.");
        require(block.number - _dynasty_switch_height > netParams[FREE_TO_REPORT_EVIL], "not time to report the devil.");
        uint curDynastyOutputNumber = addrToEpoch[devil].curDynastyOutputNumber;
        uint total;
        for (uint i = 0; i < dynasty.length; i++) {
            if(dynasty[i] != devil) {
                total += addrToEpoch[dynasty[i]].curDynastyOutputNumber;
            }
        }

        require(curDynastyOutputNumber < total/(dynasty.length - 1), "the node is not the devil.");
        require(total/(dynasty.length - 1) - curDynastyOutputNumber >
         netParams[THE_MINIMUM_OUTPUT_OVER_EVIL], "the node is not the devil.");
        uint idx = getIdxInAddressList(epochAddrs, devil);
        deleteFromAddressList(epochAddrs, idx);
        for (i = 0; i < delegateToVoters[devil].length; i++) {
            if (delegateVotesDetail[devil][delegateToVoters[devil][i]] > 0) {
                voteBonus[delegateToVoters[devil][i]] = voteBonus[delegateToVoters[devil][i]].
                add(delegateVotesDetail[devil][delegateToVoters[devil][i]]);
            }
        }
        for (i = 0; i < dynasty.length; i++) {
            if (devil == dynasty[i]) {
                deleteFromAddressList(dynasty, i);
            }
        }
        for (i = DYNASTY_SIZE; i < lastEpochAddrs.length; i++) {
            if (!IsInDynasty(lastEpochAddrs[i])) {
                dynasty.push(lastEpochAddrs[i]);
            }
        }
        // reset all dynasty output number.
        for (i = 0; i < dynasty.length; i++) {
            addrToEpoch[dynasty[i]].curDynastyOutputNumber = 0;
        }
        delegateRewardTotal = delegateRewardTotal.add(addrToEpoch[devil].pledgeAmount);
        _dynasty_switch_height = block.number;
        delete addrToEpoch[devil];
    }

    function calcBonus() public payable onlyAdmin {
        // require(msg.sender == block.coinbase, "only coinbase can do it.");
        require(msg.value == netParams[BLOCK_REWARD], "block reward is error.");
        address coinbase = block.coinbase;
        uint bookKeeperReward = netParams[BOOK_KEEPER_REWARD];
        require(bookKeeperReward < netParams[BLOCK_REWARD], "bookKeeperReward is bigger than block reward.");
        addrToEpoch[coinbase].curDynastyOutputNumber++;
        addrToEpoch[coinbase].totalOutputNumber++;
        OutputNumberHistory[coinbase]++;
        dynastyToBonus[coinbase] = dynastyToBonus[coinbase].add(bookKeeperReward * (100 - netParams[BONUS_TO_VOTERS])/100);
        for (uint i = 0; i < currentDelegateToVoters[coinbase].length; i++) {
            uint vote = currentDelegateVotesDetail[coinbase][currentDelegateToVoters[coinbase][i]];
            voteBonus[currentDelegateToVoters[coinbase][i]] = voteBonus[currentDelegateToVoters[coinbase][i]].
            add((bookKeeperReward * netParams[BONUS_TO_VOTERS] / 100) * (vote/BOX)/(addrToLastEpoch[coinbase].votes/BOX));
        }
        emit CalcBonus(coinbase, bookKeeperReward, dynastyToBonus[coinbase]);
        delegateRewardTotal = delegateRewardTotal.add(msg.value-bookKeeperReward);

        delete currentProposals;
        for (i = 1; i <= NET_PARAMS_LENGTH; i++) {
            if (proposals[i].id > 0) {
                if (block.number - proposals[i].createtime > netParams[PROPOSAL_EXPIRATION_TIME]) {
                    proposalHistory.push(proposals[i]);
                    delete proposals[i];
                } else {
                    currentProposals.push(proposals[i]);
                }
            }
        }
    }

    function execBonus() public onlyAdmin {

        require((block.number+1) % netParams[DYNASTY_CHANGE_THRESHOLD] == 0, "Not the time to switch dynasties");
        uint totalBonusToBookkeeper;
        uint pledgeLength = epochAddrs.length;
        uint delegateReward = delegateRewardTotal.div(pledgeLength);
        for(uint i = 0; i < dynasty.length; i++) {
            if (dynastyToBonus[dynasty[i]] > 0) {
                dynasty[i].transfer(dynastyToBonus[dynasty[i]]);
                totalBonusToBookkeeper += dynastyToBonus[dynasty[i]];
            }
            delete dynastyToBonus[dynasty[i]];
        }

        delegateRewardTotal = 0;
        if (epochAddrs.length >= DYNASTY_SIZE) {
            for(i = 0; i < epochAddrs.length; i++) {
                address pledgeAddr = epochAddrs[i];
                delegateToBonus[pledgeAddr] = delegateToBonus[pledgeAddr].add(delegateReward);
                address[] memory voters = delegateToVoters[pledgeAddr];
                if(voters.length > 0) {
                    currentDelegateToVoters[pledgeAddr] = voters;
                    for(uint j = 0; j < voters.length; j++) {
                        currentDelegateVotesDetail[pledgeAddr][voters[j]] = delegateVotesDetail[pledgeAddr][voters[j]];
                    }
                }
            }
            updateDynasty();
            updateNetParams();
        }
        _dynasty_switch_height = block.number;
        emit ExecBonus(totalBonusToBookkeeper, address(this).balance);
    }

    function calcScore(uint[] scores) public onlyAdmin {
        for(uint i = 0; i < scores.length; i++) {
            Delegate storage delegate = addrToEpoch[epochAddrs[i]];
            delegate.score = scores[i];
        }
    }

    function getDynasty() public view returns (Delegate[] memory) {
        Delegate[] memory res = new Delegate[](dynasty.length);
        for(uint i = 0; i < dynasty.length; i++) {
            if (addrToEpoch[dynasty[i]].pledgeAmount > 0) {
                res[i] = addrToEpoch[dynasty[i]];
            }
        }
        return res;
    }

    function getLastEpoch() public view returns (Delegate[] memory) {
        Delegate[] memory res = new Delegate[](lastEpochAddrs.length);
        for(uint i = 0; i < lastEpochAddrs.length; i++) {
            if (addrToLastEpoch[lastEpochAddrs[i]].pledgeAmount > 0) {
                res[i] = addrToLastEpoch[lastEpochAddrs[i]];
            }
        }
        return res;
    }

    function getCurrentEpoch() public view returns (Delegate[] memory) {
        Delegate[] memory res = new Delegate[](epochAddrs.length);
        for(uint i = 0; i < epochAddrs.length; i++) {
            if (addrToEpoch[epochAddrs[i]].pledgeAmount > 0) {
                res[i] = addrToEpoch[epochAddrs[i]];
            }
        }
        return res;
    }

    function updateDynasty() internal {
        Delegate[] memory sortAux = new Delegate[](epochAddrs.length);
        Delegate[] memory currentEpoch = new Delegate[](epochAddrs.length);
        for(uint i = 0; i < epochAddrs.length; i++) {
            addrToEpoch[epochAddrs[i]].curDynastyOutputNumber = 0;
            if (addrToEpoch[epochAddrs[i]].pledgeAmount > 0) {
                currentEpoch[i] = addrToEpoch[epochAddrs[i]];
                addrToLastEpoch[epochAddrs[i]] = addrToEpoch[epochAddrs[i]];
            }
        }
        sort(currentEpoch, sortAux, 0, currentEpoch.length-1);
        for (uint j = 0; j < dynasty.length; j++) {
            dynasty[j] = currentEpoch[j].addr;
        }
        delete lastEpochAddrs;
        for(i = 0; i < currentEpoch.length; i++) {
            lastEpochAddrs.push(currentEpoch[i].addr);
        }
        epochAddrs = lastEpochAddrs;
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

    function getIdxInAddressList(address[] storage addressList, address addr) internal view  returns (uint) {
        for (uint i = 0; i < addressList.length; i++) {
            if (addr == addressList[i]) {
                return i;
            }
        }
    }

    function deleteDynasty(uint index) internal {
        uint len = dynasty.length;
        if (index >= len) return;
        for (uint i = index; i<len-1; i++) {
            dynasty[i] = dynasty[i+1];
        }
        delete dynasty[len-1];
        dynasty.length--;
    }

    function deleteFromAddressList(address[] storage addressList, uint index) internal{
        uint len = addressList.length;
        if (index >= len) return;
        for (uint i = index; i<len-1; i++) {
            addressList[i] = addressList[i+1];
        }
        delete addressList[len-1];
        addressList.length--;
    }

    function IsInDynasty(address addr) internal view returns (bool) {
        for (uint i = 0; i < dynasty.length; i++) {
            if (addr == dynasty[i]) {
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

    function sort(Delegate[] arr, Delegate[] aux, uint lo, uint hi) internal {
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
        require(addrToEpoch[delegateAddr].pledgeAmount >= netParams[PLEDGE_THRESHOLD], "delegate is not exist.");
        require(block.number % netParams[DYNASTY_CHANGE_THRESHOLD] > netParams[PLEDGE_OPEN_LIMIT] &&
         block.number % netParams[DYNASTY_CHANGE_THRESHOLD] < netParams[CALC_SCORE_THRESHOLD], "Out of voting hours");

        if (votes[msg.sender][delegateAddr] > 0) {
            votes[msg.sender][delegateAddr] = votes[msg.sender][delegateAddr].add(msg.value);
        } else {
            votes[msg.sender][delegateAddr] = msg.value;
        }

        Delegate storage delegate = addrToEpoch[delegateAddr];
        delegate.votes = delegate.votes.add(msg.value);
        if (delegateVotesDetail[delegateAddr][msg.sender] > 0) {
            delegateVotesDetail[delegateAddr][msg.sender] = delegateVotesDetail[delegateAddr][msg.sender].add(msg.value);
        } else {
            delegateVotesDetail[delegateAddr][msg.sender] = msg.value;
            if (addressIsExist(msg.sender, delegateToVoters[delegateAddr]) == false) {
                delegateToVoters[delegateAddr].push(msg.sender);
            }
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
        Delegate storage delegate = addrToEpoch[delegateAddr];
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
        require(voteBonus[msg.sender] >= netParams[MIN_VOTE_BONUS_LIMIT_TO_PICK], "you don`t have enough vote bonus.");
        uint bonus = voteBonus[msg.sender];
        delete voteBonus[msg.sender];
        msg.sender.transfer(bonus);
    }

    function myDelegateReward() public view returns (uint) {
        return delegateToBonus[msg.sender];
    }

    function pickDelegateReward() public {
        require(delegateToBonus[msg.sender] >= netParams[MIN_VOTE_BONUS_LIMIT_TO_PICK], "you don`t have enough delegate reward.");
        uint bonus = delegateToBonus[msg.sender];
        delete delegateToBonus[msg.sender];
        msg.sender.transfer(bonus);
    }

    function myVote(address delegate) public view returns (uint) {
        return votes[msg.sender][delegate];
    }

    function myVoteDetail(address delegate) public view returns (uint, uint, uint) {
        if (frozenVotes[delegate][msg.sender].votes > 0 &&
            block.number > (frozenVotes[delegate][msg.sender].timestamp + netParams[VOTE_FROZEN_BLOCK_NUMBER])) {
            return (votes[msg.sender][delegate], 0, frozenVotes[delegate][msg.sender].votes);
        } else {
            return (votes[msg.sender][delegate], frozenVotes[delegate][msg.sender].votes, 0);
        }
    }

    function giveProposal(uint proposalID, uint value) public onlyDynasty payable{
        require(msg.value >= netParams[PROPOSAL_EXPENDITURE] && proposalID > 0, "Insufficient minimum fee for give proposal.");
        require(proposals[proposalID].id == 0 ||
         (block.number - proposals[proposalID].createtime > netParams[PROPOSAL_EXPIRATION_TIME]), "the proposal is exist.");

        Proposal memory proposal = Proposal(proposalID, msg.sender, value, block.number, new address[](0), false);
        proposals[proposalID] = proposal;
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
            proposalHistory.push(proposal);
            delete proposals[proposalID];
        }
    }

    function getProposalHistory() public view returns (Proposal[] memory) {
        return proposalHistory;
    }

    function getCurrentProposals() public view returns (Proposal[] memory) {
        return currentProposals;
    }


    function setGlobalOpenPledgeLimit(uint value) public onlyAdmin {
        _global_open_pledge_limit = value;
    }


    function getNetParams() public view returns (uint[14]) {
        return [netParams[PLEDGE_THRESHOLD],
        netParams[DYNASTY_CHANGE_THRESHOLD],
        netParams[MIN_VOTE_BONUS_LIMIT_TO_PICK],
        netParams[VOTE_FROZEN_BLOCK_NUMBER],
        netParams[VOTE_THRESHOLD],
        netParams[PROPOSAL_EXPENDITURE],
        netParams[PLEDGE_OPEN_LIMIT],
        netParams[PROPOSAL_EXPIRATION_TIME],
        netParams[BOOK_KEEPER_REWARD],
        netParams[BONUS_TO_VOTERS],
        netParams[CALC_SCORE_THRESHOLD],
        netParams[BLOCK_REWARD],
        netParams[FREE_TO_REPORT_EVIL],
        netParams[THE_MINIMUM_OUTPUT_OVER_EVIL]];
    }
}