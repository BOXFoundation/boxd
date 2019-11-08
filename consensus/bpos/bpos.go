// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bpos

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BOXFoundation/boxd/boxd/eventbus"
	"github.com/BOXFoundation/boxd/boxd/service"
	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/txlogic"
	"github.com/BOXFoundation/boxd/core/txpool"
	"github.com/BOXFoundation/boxd/core/types"
	state "github.com/BOXFoundation/boxd/core/worldstate"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
	acc "github.com/BOXFoundation/boxd/wallet/account"
	"github.com/jbenet/goprocess"
)

var logger = log.NewLogger("bpos") // logger

const (
	free int32 = iota
	busy
)

// Define const
const (
	SecondInMs                = int64(1000)
	BookkeeperRefreshInterval = int64(5000)
	MaxPackedTxTime           = int64(100)
	BlockNumPerPeiod          = 5
)

// Config defines the configurations of bpos
type Config struct {
	Keypath    string `mapstructure:"keypath"`
	EnableMint bool   `mapstructure:"enable_mint"`
	Passphrase string `mapstructure:"passphrase"`
}

// Bpos define bpos struct
type Bpos struct {
	chain       *chain.BlockChain
	txpool      *txpool.TransactionPool
	context     *ConsensusContext
	net         p2p.Net
	proc        goprocess.Process
	cfg         *Config
	bookkeeper  *acc.Account
	canMint     bool
	disableMint bool
	bftservice  *BftService
	status      int32
}

// NewBpos new a bpos implement.
func NewBpos(parent goprocess.Process, chain *chain.BlockChain, txpool *txpool.TransactionPool, net p2p.Net, cfg *Config) (*Bpos, error) {

	bpos := &Bpos{
		chain:   chain,
		txpool:  txpool,
		net:     net,
		proc:    goprocess.WithParent(parent),
		cfg:     cfg,
		canMint: false,
		context: &ConsensusContext{},
		status:  free,
	}
	// peer is bookkeeper, start bftService.
	bftService, err := NewBftService(bpos)
	if err != nil {
		return nil, err
	}
	bpos.bftservice = bftService
	return bpos, nil
}

// EnableMint return the peer mint status
func (bpos *Bpos) EnableMint() bool {
	return bpos.cfg.EnableMint
}

// Setup setup bpos
func (bpos *Bpos) Setup() error {
	account, err := acc.NewAccountFromFile(bpos.cfg.Keypath)
	if err != nil {
		return err
	}
	bpos.bookkeeper = account

	return nil
}

// implement interface service.Server
var _ service.Server = (*Bpos)(nil)

// Run start bpos
func (bpos *Bpos) Run() error {
	logger.Info("Bpos run")
	if !bpos.IsBookkeeper() {
		logger.Warn("You have no authority to produce block")
		return ErrNoLegalPowerToProduce
	}
	bpos.subscribe()
	bpos.bftservice.Run()
	bpos.proc.Go(bpos.loop)

	return nil
}

// Proc returns the goprocess running the service
func (bpos *Bpos) Proc() goprocess.Process {
	return bpos.proc
}

// Stop bpos
func (bpos *Bpos) Stop() {
	bpos.proc.Close()
}

// StopMint stops producing blocks.
func (bpos *Bpos) StopMint() {
	bpos.disableMint = true
}

// RecoverMint resumes producing blocks.
func (bpos *Bpos) RecoverMint() {
	bpos.disableMint = false
}

// Verify check the legality of the block.
func (bpos *Bpos) Verify(block *types.Block) error {
	ok, err := bpos.verifySign(block)
	if err != nil {
		return err
	}
	if !ok {
		return ErrFailedToVerifySign
	}

	if err := bpos.verifyInternalContractTx(block); err != nil {
		return err
	}

	if err := bpos.verifyDynasty(block); err != nil {
		return err
	}

	return bpos.verifyIrreversibleInfo(block)
}

// Finalize notify consensus to finalize the parent block by bft service.
func (bpos *Bpos) Finalize(tail *types.Block) error {
	parent := bpos.chain.GetParentBlock(tail)
	if bpos.IsBookkeeper() && time.Now().Unix()-parent.Header.TimeStamp < MaxEternalBlockMsgCacheTime {
		go func() {
			if err := bpos.BroadcastBFTMsgToBookkeepers(parent, p2p.BlockPrepareMsg); err != nil {
				logger.Errorf("Failed to broadcast bft message to bookkeepers. Err: %v", err)
			}
		}()
	}
	go bpos.TryToUpdateEternalBlock(parent)
	return nil
}

func (bpos *Bpos) loop(p goprocess.Process) {
	logger.Info("Start bpos loop")
	timeChan := time.NewTicker(time.Second)
	defer timeChan.Stop()
	for {
		select {
		case <-timeChan.C:
			if atomic.LoadInt32(&bpos.status) == free && !bpos.chain.IsBusy() {
				go func() {
					if err := bpos.run(time.Now().Unix()); err != nil {
						if err != ErrNotMyTurnToProduce {
							logger.Errorf("Bpos run err. Err: %v", err.Error())
						}
					}
				}()
			}

		case <-p.Closing():
			logger.Info("Stopped Bpos Mining.")
			return
		}
	}
}

func (bpos *Bpos) run(timestamp int64) error {

	atomic.StoreInt32(&bpos.status, busy)
	defer atomic.StoreInt32(&bpos.status, free)
	// disableMint might be set true by sync business or others
	if bpos.disableMint {
		return ErrNoLegalPowerToProduce
	}

	dynasty, err := bpos.fetchDynastyByHeight(bpos.chain.LongestChainHeight)
	if err != nil {
		return err
	}
	netParams, err := bpos.chain.FetchNetParamsByHeight(bpos.chain.LongestChainHeight)
	if err != nil {
		return err
	}
	bpos.context.dynasty = dynasty
	bpos.context.dynastySwitchThreshold = netParams.DynastySwitchThreshold
	bpos.context.bookKeeperReward = netParams.BlockReward
	bpos.context.calcScoreThreshold = netParams.CalcScoreThreshold

	current, err := bpos.fetchCurrentDelegatesByHeight(bpos.chain.LongestChainHeight)
	if err != nil {
		return err
	}
	next, err := bpos.fetchNextDelegatesByHeight(bpos.chain.LongestChainHeight)
	if err != nil {
		return err
	}
	bpos.context.currentDelegates = current
	bpos.context.nextDelegates = next
	bpos.context.candidates = bpos.filterCandidates(current, dynasty.delegates)
	go bpos.net.ReorgConns()

	if err := bpos.verifyBookkeeper(timestamp, dynasty.delegates); err != nil {
		return err
	}
	logger.Infof("My turn to produce a block, time: %d", timestamp)
	bpos.context.timestamp = timestamp
	MetricsMintTurnCounter.Inc(1)

	return bpos.produceBlock()
}

func (bpos *Bpos) filterCandidates(delegates []Delegate, miners []Delegate) []Delegate {
	if len(delegates) < len(miners) {
		return make([]Delegate, 0)
	}
	candidates := make([]Delegate, len(delegates)-len(miners))
	minerPid := make(map[string]interface{}, len(miners))
	for _, miner := range miners {
		minerPid[miner.PeerID] = struct{}{}
	}
	for _, delegate := range delegates {
		if _, ok := minerPid[delegate.PeerID]; !ok {
			candidates = append(candidates, delegate)
		}
	}
	return candidates
}

// verifyProposer check to verify if bookkeeper can mint at the timestamp
func (bpos *Bpos) verifyBookkeeper(timestamp int64, delegates []Delegate) error {
	bookkeeper, err := bpos.FindProposerWithTimeStamp(timestamp, delegates)
	if err != nil {
		return err
	}
	addr, err := types.NewAddress(bpos.bookkeeper.Addr())
	if err != nil {
		return err
	}
	if *bookkeeper != *addr.Hash160() {
		return ErrNotMyTurnToProduce
	}
	return nil
}

// IsBookkeeper verifies whether the peer has authority to produce block.
func (bpos *Bpos) IsBookkeeper() bool {

	if bpos.bookkeeper == nil {
		return false
	}

	if bpos.canMint {
		return true
	}

	if err := bpos.bookkeeper.UnlockWithPassphrase(bpos.cfg.Passphrase); err != nil {
		logger.Error(err)
		return false
	}
	bpos.canMint = true
	return true
}

func (bpos *Bpos) produceBlock() error {

	tail := bpos.chain.TailBlock()
	block := types.NewBlock(tail)
	block.Header.TimeStamp = bpos.context.timestamp
	dynastyBytes, err := bpos.context.dynasty.Marshal()
	if err != nil {
		return err
	}
	block.Header.DynastyHash = crypto.DoubleHashH(dynastyBytes)

	if err := bpos.PackTxs(block, bpos.bookkeeper.PubKeyHash()); err != nil {
		logger.Warnf("Failed to pack txs. err: %s", err.Error())
		return err
	}
	if err := bpos.signBlock(block); err != nil {
		logger.Warnf("Failed to sign block. err: %s", err.Error())
		return err
	}
	go bpos.chain.BroadcastOrRelayBlock(block, core.BroadcastMode)
	if err := bpos.chain.ProcessBlock(block, core.DefaultMode, ""); err != nil {
		logger.Warnf("Failed to process block mint by self. err: %s", err.Error())
	}

	return nil
}

func lessFunc(queue *util.PriorityQueue, i, j int) bool {
	txi := queue.Items(i).(*types.TxWrap)
	txj := queue.Items(j).(*types.TxWrap)
	return txi.AddedTimestamp < txj.AddedTimestamp
}

func (bpos *Bpos) nonceFunc(queue *util.PriorityQueue, i, j int) bool {
	txi := queue.Items(i).(*types.VMTransaction)
	txj := queue.Items(j).(*types.VMTransaction)
	return txi.Nonce() < txj.Nonce()
}

// sort pending transactions in mempool
func (bpos *Bpos) sortPendingTxs(pendingTxs []*types.TxWrap) ([]*types.TxWrap, error) {

	pool := util.NewPriorityQueue(lessFunc)
	hashToTx := make(map[crypto.HashType]*types.TxWrap)
	addressToTxs := make(map[types.AddressHash]*util.PriorityQueue)
	addressToNonceSortedTxs := make(map[types.AddressHash][]*types.VMTransaction)
	hashToAddress := make(map[crypto.HashType]types.AddressHash)

	tail := bpos.chain.TailBlock()
	statedb, err := state.New(&tail.Header.RootHash, &tail.Header.UtxoRoot, bpos.chain.DB())
	if err != nil {
		return nil, err
	}

	for _, pendingTx := range pendingTxs {
		txHash, _ := pendingTx.Tx.TxHash()
		// place onto heap sorted by gasPrice
		// only pack txs whose scripts have been verified
		if pendingTx.IsScriptValid {
			heap.Push(pool, pendingTx)
			hashToTx[*txHash] = pendingTx
			if pendingTx.IsContract {
				// from is in txpool if the contract tx used a vout in txpool
				op := pendingTx.Tx.Vin[0].PrevOutPoint
				ownerTx, ok := bpos.txpool.GetTxByHash(&op.Hash)
				if !ok { // no need to find owner in orphan tx pool
					ownerTx = nil
				}
				// extract contract tx
				vmTx, err := chain.ExtractVMTransaction(pendingTx.Tx, ownerTx.GetTx())
				if err != nil {
					return nil, err
				}
				from := *vmTx.From()
				if v, exists := addressToTxs[from]; exists {
					heap.Push(v, vmTx)
				} else {
					nonceQueue := util.NewPriorityQueue(bpos.nonceFunc)
					heap.Push(nonceQueue, vmTx)
					addressToTxs[from] = nonceQueue
					hashToAddress[*txHash] = from
				}
			}
		}
	}

	for from, v := range addressToTxs {
		var vmtxs []*types.VMTransaction
		currentNonce := statedb.GetNonce(from)

		for v.Len() > 0 {
			vmTx := heap.Pop(v).(*types.VMTransaction)
			hash := vmTx.OriginTxHash()
			if vmTx.Nonce() != currentNonce+1 {
				// remove from mem_pool if vmTx nonce is smaller than current nonce
				if vmTx.Nonce() < currentNonce+1 {
					logger.Warnf("vm tx %+v has a wrong nonce, expect nonce: %d, remove it from mem_pool.", vmTx, currentNonce+1)
					bpos.chain.Bus().Publish(eventbus.TopicInvalidTx, hashToTx[*hash].Tx, true)
				}
				logger.Warnf("vm tx %+v has a bigger nonce, expect nonce: %d", vmTx, currentNonce+1)
				delete(hashToTx, *hash)
				continue
			}
			currentNonce++
			vmtxs = append(vmtxs, vmTx)
		}
		addressToNonceSortedTxs[from] = vmtxs
	}

	dag := util.NewDag()
	for pool.Len() > 0 {
		txWrap := heap.Pop(pool).(*types.TxWrap)
		txHash, _ := txWrap.Tx.TxHash()
		if _, exists := hashToTx[*txHash]; !exists {
			continue
		}
		dag.AddNode(*txHash, int(txWrap.AddedTimestamp))
		if txWrap.IsContract {
			from := hashToAddress[*txHash]
			sortedNonceTxs := addressToNonceSortedTxs[from]
			handleVMTx(dag, sortedNonceTxs, hashToTx)
			delete(addressToNonceSortedTxs, from)
		}
		for _, txIn := range txWrap.Tx.Vin {
			prevTxHash := txIn.PrevOutPoint.Hash
			if wrap, exists := hashToTx[prevTxHash]; exists {
				dag.AddNode(prevTxHash, int(wrap.AddedTimestamp))
				dag.AddEdge(prevTxHash, *txHash)
			}
		}
	}
	if dag.IsCirclular() {
		return nil, ErrCircleTxExistInDag
	}
	var sortedTxs []*types.TxWrap
	nodes := dag.TopoSort()
	for _, v := range nodes {
		hash := v.Key().(crypto.HashType)
		sortedTxs = append(sortedTxs, hashToTx[hash])
	}
	return sortedTxs, nil
}

func handleVMTx(
	dag *util.Dag, sortedNonceTxs []*types.VMTransaction,
	hashToTx map[crypto.HashType]*types.TxWrap,
) {
	var parentHash *crypto.HashType
	for _, vmTx := range sortedNonceTxs {
		hash := vmTx.OriginTxHash()
		originTx := hashToTx[*hash]
		dag.AddNode(*hash, int(originTx.AddedTimestamp))
		if parentHash != nil {
			dag.AddEdge(*parentHash, *hash)
		}
		parentHash = hash
	}
}

// PackTxs packed txs and add them to block.
func (bpos *Bpos) PackTxs(block *types.Block, scriptAddr []byte) error {

	// We sort txs in mempool by fees when packing while ensuring child tx is not packed before parent tx.
	// otherwise the former's utxo is missing
	pendingTxs := bpos.txpool.GetAllTxs()
	sortedTxs, err := bpos.sortPendingTxs(pendingTxs)
	if err != nil {
		return err
	}

	var packedTxs []*types.Transaction
	remainTimeInMs := bpos.context.timestamp*SecondInMs + MaxPackedTxTime - time.Now().Unix()*SecondInMs
	spendableTxs := new(sync.Map)

	// Total fees of all packed txs
	totalTransferFee := uint64(0)

	stopPack := false
	stopPackCh := make(chan bool, 1)
	continueCh := make(chan bool, 1)

	go func() {
		packedTxsRoughSize := 0
		for txIdx, txWrap := range sortedTxs {
			if stopPack {
				continueCh <- true
				logger.Debugf("stops at %d-th tx: packed %d txs out of %d", txIdx,
					len(packedTxs), len(sortedTxs))
				return
			}
			if packedTxsRoughSize+txWrap.Tx.RoughSize() > core.MaxTxsRoughSize {
				logger.Infof("txs size reach %d KB, stop packing", packedTxsRoughSize/1024)
				break
			}

			txHash, _ := txWrap.Tx.TxHash()
			if txWrap.IsContract {
				spendableTxs.Store(*txHash, txWrap)
				packedTxs = append(packedTxs, txWrap.Tx)
				continue
			}

			utxoSet, err := chain.GetExtendedTxUtxoSet(txWrap.Tx, bpos.chain.DB(), spendableTxs)
			if err != nil {
				logger.Warnf("Could not get extended utxo set for tx %v", txHash)
				continue
			}

			totalInputAmount := utxoSet.TxInputAmount(txWrap.Tx)
			if totalInputAmount == 0 {
				// This can only occur when a tx's parent is removed from mempool but not written to utxo db yet
				logger.Errorf("This can not occur totalInputAmount == 0, tx hash: %v", txHash)
				continue
			}
			totalOutputAmount := txWrap.Tx.OutputAmount()
			if totalInputAmount != totalOutputAmount+core.TransferFee+txWrap.Tx.ExtraFee() {
				// This must not happen since the tx already passed the check when admitted into mempool
				logger.Warnf("total value of all transaction outputs for "+
					"transaction %v is %v, which exceeds the input amount "+
					"of %v", txHash, totalOutputAmount, totalInputAmount)
				// TODO: abandon the error tx from pool.
				continue
			}
			totalTransferFee += core.TransferFee + txWrap.Tx.ExtraFee()

			spendableTxs.Store(*txHash, txWrap)
			packedTxs = append(packedTxs, txWrap.Tx)
		}
		continueCh <- true
		stopPackCh <- true
	}()

	select {
	case <-time.After(time.Duration(remainTimeInMs) * time.Millisecond):
		logger.Debug("Packing timeout")
		stopPack = true
	case <-stopPackCh:
		logger.Debug("Packing completed")
	}

	// Important: wait for packing complete and exit
	<-continueCh

	block.Header.BookKeeper = *bpos.bookkeeper.Address.Hash160()
	parent, err := chain.LoadBlockByHash(block.Header.PrevBlockHash, bpos.chain.DB())
	if err != nil {
		return err
	}
	statedb, err := state.New(&parent.Header.RootHash, &parent.Header.UtxoRoot, bpos.chain.DB())
	if err != nil {
		return err
	}
	adminAddr, err := types.NewAddress(chain.Admin)
	if err != nil {
		return err
	}
	from := *adminAddr.Hash160()
	adminNonce := statedb.GetNonce(from) + 1
	statedb.AddBalance(from, new(big.Int).SetUint64(bpos.context.bookKeeperReward.Uint64()))
	if totalTransferFee > 0 {
		statedb.AddBalance(block.Header.BookKeeper, big.NewInt(int64(totalTransferFee)))
	}

	// coinbaseTx, err := bpos.makeCoinbaseTx(from, block, statedb, totalTransferFee, adminNonce)
	coinbaseTx, err := chain.MakeCoinBaseContractTx(block.Header.BookKeeper,
		bpos.context.bookKeeperReward.Uint64(), totalTransferFee, adminNonce, block.Header.Height)
	if err != nil {
		return err
	}
	block.Txs = append(block.Txs, coinbaseTx)

	if uint64((block.Header.Height+1))%bpos.context.dynastySwitchThreshold.Uint64() == 0 { // dynasty switch
		adminNonce++
		dynastySwitchTx, err := bpos.chain.MakeDynastySwitchTx(*adminAddr.Hash160(), adminNonce, block.Header.Height)
		if err != nil {
			return err
		}
		block.Txs = append(block.Txs, dynastySwitchTx)
	} else if uint64(block.Header.Height)%bpos.context.dynastySwitchThreshold.Uint64() == bpos.context.calcScoreThreshold.Uint64() { // calc score
		adminNonce++
		scores, err := bpos.calcScores()
		if err != nil {
			return err
		}
		calcScoreTx, err := bpos.chain.MakeCalcScoreTx(*adminAddr.Hash160(), adminNonce, block.Header.Height, scores)
		if err != nil {
			return err
		}
		block.Txs = append(block.Txs, calcScoreTx)
	}

	block.Txs = append(block.Txs, packedTxs...)

	if err := bpos.executeBlock(block, statedb); err != nil {
		return err
	}
	block.IrreversibleInfo = bpos.bftservice.FetchIrreversibleInfo()
	logger.Infof("Finish packing txs. Hash: %v, Height: %d, TxsNum: %d, internal"+
		" TxsNum: %d, Mempool TxsNum: %d", block.BlockHash(), block.Header.Height,
		len(block.Txs), len(block.InternalTxs), len(sortedTxs))
	return nil
}

func (bpos *Bpos) makeCoinbaseTx(from types.AddressHash, block *types.Block,
	statedb *state.StateDB, txFee uint64, nonce uint64) (*types.Transaction, error) {
	amount := bpos.context.bookKeeperReward.Uint64() + txFee
	logger.Infof("make coinbaseTx %s:%d amount: %d txFee: %d",
		block.BlockHash(), block.Header.Height, amount, txFee)
	statedb.AddBalance(from, new(big.Int).SetUint64(amount))
	return chain.MakeCoinBaseContractTx(from,
		bpos.context.bookKeeperReward.Uint64(), txFee, nonce, block.Header.Height)
}

func (bpos *Bpos) executeBlock(block *types.Block, statedb *state.StateDB) error {

	genesisContractBalanceOld := statedb.GetBalance(chain.ContractAddr).Uint64()
	logger.Infof("Before execute block %d statedb root: %s utxo root: %s genesis contract balance: %d",
		block.Header.Height, statedb.RootHash(), statedb.UtxoRoot(), genesisContractBalanceOld)

	utxoSet := chain.NewUtxoSet()
	if err := utxoSet.LoadBlockUtxos(block, true, bpos.chain.DB()); err != nil {
		return err
	}
	blockCopy := block.Copy()
	bpos.chain.SplitBlockOutputs(blockCopy)
	if err := utxoSet.ApplyBlock(blockCopy, bpos.chain.IsContractAddr2); err != nil {
		return err
	}
	bpos.chain.UpdateNormalTxBalanceState(blockCopy, utxoSet, statedb)
	receipts, gasUsed, utxoTxs, err :=
		bpos.chain.StateProcessor().Process(block, statedb, utxoSet)
	if err != nil {
		return err
	}
	coinbaseHash, err := block.Txs[0].TxHash()
	if err != nil {
		return err
	}
	opCoinbase := types.NewOutPoint(coinbaseHash, 1)

	if gasUsed > 0 {
		bookkeeper := bpos.bookkeeper.AddressHash()
		statedb.AddBalance(*bookkeeper, big.NewInt(int64(gasUsed)))
		if len(block.Txs[0].Vout) == 2 {
			block.Txs[0].Vout[1].Value += gasUsed
		} else {
			block.Txs[0].AppendVout(txlogic.MakeVout(bookkeeper, gasUsed))
		}
		reward := block.Txs[0].Vout[0].Value + block.Txs[0].Vout[1].Value
		logger.Infof("gas used for block %d: %d, bookkeeper %s reward: %d",
			block.Header.Height, gasUsed, bookkeeper, reward)
		utxoSet.SpendUtxo(*opCoinbase)
		block.Txs[0].ResetTxHash()
		utxoSet.AddUtxo(block.Txs[0], 1, block.Header.Height)
		newHash, _ := block.Txs[0].TxHash()
		receipts[0].WithTxHash(newHash)
	}

	// apply internal txs.
	block.InternalTxs = utxoTxs
	if len(utxoTxs) > 0 {
		if err := utxoSet.ApplyInternalTxs(block); err != nil {
			return err
		}
	}
	if err := bpos.chain.UpdateContractUtxoState(statedb, utxoSet); err != nil {
		return err
	}

	root, utxoRoot, err := statedb.Commit(false)
	if err != nil {
		return err
	}

	logger.Infof("After execute block %d statedb root: %s utxo root: %s genesis "+
		"contract balance: %d", block.Header.Height, statedb.RootHash(),
		statedb.UtxoRoot(), statedb.GetBalance(chain.ContractAddr))
	logger.Infof("genesis contract balance change, previous %d, coinbase value: "+
		"%d, gas used %d, now %d in statedb", genesisContractBalanceOld,
		block.Txs[0].Vout[0].Value, gasUsed, statedb.GetBalance(chain.ContractAddr))

	block.Header.GasUsed = gasUsed
	block.Header.RootHash = *root

	txsRoot := chain.CalcTxsHash(block.Txs)
	block.Header.TxsRoot = *txsRoot
	if len(utxoTxs) > 0 {
		internalTxsRoot := chain.CalcTxsHash(utxoTxs)
		block.Header.InternalTxsRoot = *internalTxsRoot
	}
	if utxoRoot != nil {
		block.Header.UtxoRoot = *utxoRoot
	}
	block.Header.Bloom = types.CreateReceiptsBloom(receipts)
	if len(receipts) > 0 {
		block.Header.ReceiptHash = *receipts.Hash()
	}
	// check whether contract balance is identical in utxo and statedb
	for o, u := range utxoSet.ContractUtxos() {
		contractAddr := types.NewAddressHash(o.Hash[:])
		if u.Value() != statedb.GetBalance(*contractAddr).Uint64() {
			address, _ := types.NewContractAddressFromHash(contractAddr[:])
			return fmt.Errorf("contract %s have ambiguous balance(%d in utxo and %d"+
				" in statedb)", address, u.Value(), statedb.GetBalance(*contractAddr))
		}
	}
	bpos.chain.BlockExecuteResults()[block.Header.Height] = &chain.BlockExecuteResult{
		StateDB:  statedb,
		Receipts: receipts,
		UtxoSet:  utxoSet,
	}
	block.Hash = nil
	logger.Infof("block %s height: %d have state root %s utxo root %s",
		block.BlockHash(), block.Header.Height, root, utxoRoot)
	return nil
}

// BroadcastBFTMsgToBookkeepers broadcast block BFT message to bookkeepers
func (bpos *Bpos) BroadcastBFTMsgToBookkeepers(block *types.Block, messageID uint32) error {

	prepareBlockMsg := &EternalBlockMsg{}
	hash := block.BlockHash()
	signature, err := crypto.SignCompact(bpos.bookkeeper.PrivateKey(), hash[:])
	if err != nil {
		return err
	}
	prepareBlockMsg.Hash = *hash
	prepareBlockMsg.Signature = signature
	prepareBlockMsg.Timestamp = block.Header.TimeStamp
	bookkeepers := bpos.context.verifyDynasty.peers
	logger.Debugf("BroadcastBFTMsgToBookkeepers peers: %v", bookkeepers)

	return bpos.net.BroadcastToBookkeepers(messageID, prepareBlockMsg, bookkeepers)
}

// verifyCandidates vefiry if the block candidates hash is right.
func (bpos *Bpos) verifyDynasty(block *types.Block) error {
	if block.Header.Height > 0 {
		dynastyBytes, err := bpos.context.verifyDynasty.Marshal()
		if err != nil {
			return err
		}
		dynastyHash := crypto.DoubleHashH(dynastyBytes)
		if !(&block.Header.DynastyHash).IsEqual(&dynastyHash) {
			return ErrInvalidDynastyHash
		}
	}

	return nil
}

// verifyIrreversibleInfo vefiry if the block irreversibleInfo is right.
func (bpos *Bpos) verifyIrreversibleInfo(block *types.Block) error {

	irreversibleInfo := block.IrreversibleInfo
	if irreversibleInfo != nil {
		dynasty := bpos.context.verifyDynasty
		if len(irreversibleInfo.Signatures) < 2*len(dynasty.delegates)/3 {
			logger.Errorf("the number of irreversibleInfo signatures is not enough. "+
				"signatures len: %d dynasty len: %d", len(irreversibleInfo.Signatures),
				len(dynasty.delegates))
			return errors.New("the number of irreversibleInfo signatures is not enough")
		}

		remains := []types.AddressHash{}
		for _, v := range irreversibleInfo.Signatures {
			if pubkey, ok := crypto.RecoverCompact(irreversibleInfo.Hash[:], v); ok {
				addrPubKeyHash, err := types.NewAddressFromPubKey(pubkey)
				if err != nil {
					return err
				}
				addr := *addrPubKeyHash.Hash160()
				if types.InAddresses(addr, dynasty.addrs) {
					if !types.InAddresses(addr, remains) {
						remains = append(remains, addr)
					} else {
						logger.Errorf("Duplicated irreversible signature %v in block. Hash: %s, Height: %d",
							v, block.BlockHash().String(), block.Header.Height)
						return errors.New("Duplicated irreversible signature in block")
					}
				} else {
					logger.Errorf("Invalid irreversible signature %v in block. Hash: %s, Height: %d",
						v, block.BlockHash().String(), block.Header.Height)
					return errors.New("Invalid irreversible signature in block")
				}
			} else {
				return errors.New("Invalid irreversible signature in block")
			}
		}
		if len(remains) < 2*len(dynasty.delegates)/3 {
			logger.Errorf("Invalid irreversible info in block. Hash: %s, Height: %d,"+
				" remains: %d", block.BlockHash().String(), block.Header.Height, len(remains))
			return errors.New("Invalid irreversible info in block")
		}
	}
	return nil
}

func (bpos *Bpos) signBlock(block *types.Block) error {

	hash := block.BlockHash()
	signature, err := crypto.SignCompact(bpos.bookkeeper.PrivateKey(), hash[:])
	if err != nil {
		return err
	}
	block.Signature = signature
	return nil
}

// verifySign consensus verifies signature info.
func (bpos *Bpos) verifySign(block *types.Block) (bool, error) {
	var height uint32
	if block.Header.Height > 0 {
		height = block.Header.Height - 1
	}
	dynasty, err := bpos.fetchDynastyByHeight(height)
	if err != nil {
		return false, err
	}
	netParams, err := bpos.chain.FetchNetParamsByHeight(height)
	if err != nil {
		return false, err
	}

	bpos.context.verifyDynasty = dynasty
	bpos.context.verifyDynastySwitchThreshold = netParams.DynastySwitchThreshold
	bpos.context.verifyCalcScoreThreshold = netParams.CalcScoreThreshold
	bookkeeper, err := bpos.FindProposerWithTimeStamp(block.Header.TimeStamp, dynasty.delegates)
	if err != nil {
		return false, err
	}
	if bookkeeper == nil {
		return false, ErrNotFoundBookkeeper
	}

	if pubkey, ok := crypto.RecoverCompact(block.BlockHash()[:], block.Signature); ok {
		addr, err := types.NewAddressFromPubKey(pubkey)
		if err != nil {
			return false, err
		}
		if *addr.Hash160() == *bookkeeper {
			return true, nil
		}
	}

	return false, nil
}

func (bpos *Bpos) verifyImmutableTx(block *types.Block, ty string) error {

	parent := bpos.chain.GetParentBlock(block)
	statedb, err := state.New(&parent.Header.RootHash, &parent.Header.UtxoRoot, bpos.chain.DB())
	if err != nil {
		return err
	}
	adminAddr, err := types.NewAddress(chain.Admin)
	if err != nil {
		return err
	}
	// adminNonce := statedb.GetNonce(*adminAddr.Hash160()) + 2
	from := *adminAddr.Hash160()
	var internalContractTx *types.Transaction
	switch ty {
	case chain.CalcBonus:
		adminNonce := statedb.GetNonce(from) + 1
		var totalTransferFee uint64
		if len(block.Txs[0].Vout) > 1 {
			totalTransferFee = block.Txs[0].Vout[1].Value
		}
		internalContractTx, err = chain.MakeCoinBaseContractTx(block.Header.BookKeeper,
			block.Txs[0].Vout[0].Value, totalTransferFee, adminNonce, block.Header.Height)
		if err != nil {
			return err
		}
		expect, err := internalContractTx.Marshal()
		if err != nil {
			return err
		}
		current, err := block.Txs[0].Marshal()
		if err != nil {
			return err
		}
		if !bytes.Equal(expect, current) {
			return ErrInvalidCoinbaseTx
		}
	case chain.ExecBonus:
		adminNonce := statedb.GetNonce(from) + 2
		internalContractTx, err = bpos.chain.MakeDynastySwitchTx(*adminAddr.Hash160(), adminNonce, block.Header.Height)
		if err != nil {
			return err
		}
		expect, err := internalContractTx.Marshal()
		if err != nil {
			return err
		}
		current, err := block.Txs[1].Marshal()
		if err != nil {
			return err
		}
		if !bytes.Equal(expect, current) {
			return ErrInvalidDynastySwitchTx
		}
	case chain.CalcScore:
		adminNonce := statedb.GetNonce(from) + 2
		scores, err := bpos.calcScores()
		if err != nil {
			return err
		}
		internalContractTx, err = bpos.chain.MakeCalcScoreTx(*adminAddr.Hash160(), adminNonce, block.Header.Height, scores)
		if err != nil {
			return err
		}
		expect, err := internalContractTx.Marshal()
		if err != nil {
			return err
		}
		current, err := block.Txs[1].Marshal()
		if err != nil {
			return err
		}
		if !bytes.Equal(expect, current) {
			return ErrInvalidCalcScoreTx
		}
	}

	return nil
}

func (bpos *Bpos) verifyInternalContractTx(block *types.Block) error {

	if err := bpos.verifyImmutableTx(block, chain.CalcBonus); err != nil { // verify coinbase tx
		return err
	}

	height := block.Header.Height
	switchHeight := bpos.context.verifyDynastySwitchThreshold.Uint64()
	scoreHeight := bpos.context.verifyCalcScoreThreshold.Uint64()
	if uint64(height+1)%switchHeight == 0 { // dynasty switch
		if err := bpos.verifyImmutableTx(block, chain.ExecBonus); err != nil {
			return err
		}
	}
	// else if uint64(height)%switchHeight == scoreHeight { // calc score
	// 	if err := bpos.verifyImmutableTx(block, chain.CalcScore); err != nil {
	// 		return err
	// 	}
	// }
	if uint64(height+1)%switchHeight == 0 || uint64(height)%switchHeight == scoreHeight {
		for i, tx := range block.Txs[2:] {
			if chain.IsInternalContract(tx) {
				logger.Errorf("block contains second internal contract tx at index %d", i+2)
				return ErrMultipleDynastySwitchTx
			}
		}
	} else {
		for _, tx := range block.Txs {
			if chain.IsInternalContract(tx) {
				return ErrDynastySwitchIsNotAllowed
			}
		}
	}
	return nil
}

// TryToUpdateEternalBlock try to update eternal block.
func (bpos *Bpos) TryToUpdateEternalBlock(src *types.Block) {
	irreversibleInfo := src.IrreversibleInfo
	dynasty := bpos.context.verifyDynasty
	if irreversibleInfo != nil {
		logger.Debugf("TryToUpdateEternalBlock received number of signatures: %d",
			len(irreversibleInfo.Signatures))
	}
	if irreversibleInfo != nil && len(irreversibleInfo.Signatures) >= 2*len(dynasty.delegates)/3 {
		block, err := chain.LoadBlockByHash(irreversibleInfo.Hash, bpos.chain.DB())
		if err != nil {
			logger.Warnf("Failed to update eternal block. Err: %s", err.Error())
			return
		}
		bpos.bftservice.updateEternal(block)
	}
}

// Miners return miners.
func (bpos *Bpos) Miners() ([]string, bool) {
	if bpos.context.dynasty == nil {
		return []string{}, bpos.cfg.EnableMint
	}
	return bpos.context.dynasty.peers, bpos.cfg.EnableMint
}

// Candidates return miners.
func (bpos *Bpos) Candidates() ([]string, bool) {
	candidates := []string{}
	for _, c := range bpos.context.candidates {
		candidates = append(candidates, c.PeerID)
	}
	for _, c := range bpos.context.nextDelegates {
		candidates = append(candidates, c.PeerID)
	}
	return candidates, bpos.cfg.EnableMint
}

func (bpos *Bpos) subscribe() {
	bpos.chain.Bus().Reply(eventbus.TopicMiners, func(out chan<- []string) {
		out <- bpos.context.dynasty.peers
	}, false)
	bpos.chain.Bus().Reply(eventbus.TopicCheckMiner, func(timestamp int64, out chan<- error) {
		dynasty, err := bpos.fetchDynastyByHeight(bpos.chain.LongestChainHeight)
		if err != nil {
			out <- err
		} else {
			out <- bpos.verifyBookkeeper(timestamp, dynasty.delegates)
		}
	}, false)
}
