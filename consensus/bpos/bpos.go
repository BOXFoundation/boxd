// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bpos

import (
	"container/heap"
	"encoding/json"
	"errors"
	"math/big"
	"sync"
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
}

// ConsensusContext represents consensus context info.
type ConsensusContext struct {
	timestamp int64
	dynasty   *Dynasty
}

// NewBpos new a bpos implement.
func NewBpos(parent goprocess.Process, chain *chain.BlockChain, txpool *txpool.TransactionPool, net p2p.Net, cfg *Config) *Bpos {
	return &Bpos{
		chain:   chain,
		txpool:  txpool,
		net:     net,
		proc:    goprocess.WithParent(parent),
		cfg:     cfg,
		canMint: false,
		context: &ConsensusContext{},
	}
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

	// peer is bookkeeper, start bftService.
	bftService, err := NewBftService(bpos)
	if err != nil {
		return err
	}
	bpos.bftservice = bftService
	bpos.subscribe()
	bftService.Run()
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
		return errors.New("Failed to verify sign block")
	}

	if err := bpos.verifyDynasty(block); err != nil {
		return err
	}

	return bpos.verifyIrreversibleInfo(block)
}

// Finalize notify consensus to change new tail.
func (bpos *Bpos) Finalize(tail *types.Block) error {
	if bpos.IsBookkeeper() && time.Now().Unix()-tail.Header.TimeStamp < MaxEternalBlockMsgCacheTime {
		go bpos.BroadcastBFTMsgToBookkeepers(tail, p2p.BlockPrepareMsg)
	}
	go bpos.TryToUpdateEternalBlock(tail)
	return nil
}

func (bpos *Bpos) loop(p goprocess.Process) {
	logger.Info("Start bpos loop")
	timeChan := time.NewTicker(time.Second)
	defer timeChan.Stop()
	for {
		select {
		case <-timeChan.C:
			if !bpos.chain.IsBusy() {
				bpos.run(time.Now().Unix())
			}

		case <-p.Closing():
			logger.Info("Stopped Bpos Mining.")
			return
		}
	}
}

func (bpos *Bpos) run(timestamp int64) error {

	// disableMint might be set true by sync business or others
	if bpos.disableMint {
		return ErrNoLegalPowerToProduce
	}

	dynasty, err := bpos.fetchDynastyByHeight(bpos.chain.LongestChainHeight)
	if err != nil {
		return err
	}
	bpos.context.dynasty = dynasty

	if err := bpos.verifyBookkeeper(timestamp, dynasty.delegates); err != nil {
		return err
	}
	bpos.context.timestamp = timestamp
	MetricsMintTurnCounter.Inc(1)

	logger.Infof("My turn to produce a block, time: %d", timestamp)
	return bpos.produceBlock()
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

	addr, err := types.NewAddress(bpos.bookkeeper.Addr())
	if err != nil {
		return false
	}
	dynasty, err := bpos.fetchDynastyByHeight(bpos.chain.LongestChainHeight)
	if err != nil {
		return false
	}
	if !util.InArray(*addr.Hash160(), dynasty.addrs) {
		return false
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
	dynastyBytes, err := json.Marshal(bpos.context.dynasty)
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

	go func() {
		bpos.chain.BroadcastOrRelayBlock(block, core.BroadcastMode)
		if err := bpos.chain.ProcessBlock(block, core.DefaultMode, ""); err != nil {
			logger.Warnf("Failed to process block mint by self. err: %s", err.Error())
		}
	}()

	return nil
}

func lessFunc(queue *util.PriorityQueue, i, j int) bool {
	txi := queue.Items(i).(*types.TxWrap)
	txj := queue.Items(j).(*types.TxWrap)
	if txi.GasPrice == txj.GasPrice {
		return txi.AddedTimestamp < txj.AddedTimestamp
	}
	return txi.GasPrice > txj.GasPrice
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
			if txlogic.HasContractVout(pendingTx.Tx) { // smart contract tx
				// from is in txpool if the contract tx used a vout in txpool
				op := pendingTx.Tx.Vin[0].PrevOutPoint
				ownerTx, ok := bpos.txpool.GetTxByHash(&op.Hash)
				if !ok { // no need to find owner in orphan tx pool
					ownerTx = nil
				}
				// extract contract tx
				vmTx, err := bpos.chain.ExtractVMTransactions(pendingTx.Tx, ownerTx.GetTx())
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
				logger.Warnf("vm tx %+v has a wrong nonce(now %d), remove it", vmTx, currentNonce)
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
		dag.AddNode(*txHash, int(txWrap.GasPrice))
		if txlogic.HasContractVout(txWrap.Tx) { // smart contract tx
			from := hashToAddress[*txHash]
			sortedNonceTxs := addressToNonceSortedTxs[from]
			handleVMTx(dag, sortedNonceTxs, hashToTx)
			delete(addressToNonceSortedTxs, from)
		}
		for _, txIn := range txWrap.Tx.Vin {
			prevTxHash := txIn.PrevOutPoint.Hash
			if wrap, exists := hashToTx[prevTxHash]; exists {
				dag.AddNode(prevTxHash, int(wrap.GasPrice))
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

func handleVMTx(dag *util.Dag, sortedNonceTxs []*types.VMTransaction, hashToTx map[crypto.HashType]*types.TxWrap) {
	var parentHash *crypto.HashType
	for _, vmTx := range sortedNonceTxs {
		hash := vmTx.OriginTxHash()
		originTx := hashToTx[*hash]
		dag.AddNode(*hash, int(originTx.GasPrice))
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
	totalTxFee := uint64(0)
	stopPack := false
	stopPackCh := make(chan bool, 1)
	continueCh := make(chan bool, 1)

	go func() {
		for txIdx, txWrap := range sortedTxs {
			if stopPack {
				continueCh <- true
				logger.Debugf("stops at %d-th tx: packed %d txs out of %d", txIdx, len(packedTxs), len(sortedTxs))
				return
			}

			txHash, _ := txWrap.Tx.TxHash()
			if txlogic.HasContractVout(txWrap.Tx) {
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
			if totalInputAmount < totalOutputAmount {
				// This must not happen since the tx already passed the check when admitted into mempool
				logger.Warnf("total value of all transaction outputs for "+
					"transaction %v is %v, which exceeds the input amount "+
					"of %v", txHash, totalOutputAmount, totalInputAmount)
				// TODO: abandon the error tx from pool.
				continue
			}
			txFee := totalInputAmount - totalOutputAmount
			totalTxFee += txFee

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
	coinbaseTx, err := bpos.makeCoinbaseTx(block, statedb, totalTxFee)
	if err != nil {
		return err
	}
	block.Txs = append(block.Txs, coinbaseTx)
	block.Txs = append(block.Txs, packedTxs...)

	if err := bpos.executeBlock(block, statedb); err != nil {
		return err
	}
	block.IrreversibleInfo = bpos.bftservice.FetchIrreversibleInfo()
	logger.Infof("Finish packing txs. Hash: %v, Height: %d, Block TxsNum: %d, "+
		"internal TxsNum: %d, Mempool TxsNum: %d", block.BlockHash(),
		block.Header.Height, len(block.Txs), len(block.InternalTxs), len(sortedTxs))
	return nil
}

func (bpos *Bpos) makeCoinbaseTx(block *types.Block, statedb *state.StateDB, txFee uint64) (*types.Transaction, error) {

	amount := chain.CalcBlockSubsidy(block.Header.Height) + txFee
	nonce := statedb.GetNonce(block.Header.BookKeeper)
	statedb.AddBalance(block.Header.BookKeeper, new(big.Int).SetUint64(amount))
	return bpos.chain.MakeCoinbaseTx(block.Header.BookKeeper, amount, nonce+1, block.Header.Height)
}

func (bpos *Bpos) executeBlock(block *types.Block, statedb *state.StateDB) error {

	genesisContractBalanceOld := statedb.GetBalance(chain.ContractAddr).Uint64()
	logger.Infof("Before execute block.statedb root: %s utxo root: %s genesis contract balance: %d block height: %d",
		statedb.RootHash(), statedb.UtxoRoot(), genesisContractBalanceOld, block.Header.Height)

	utxoSet := chain.NewUtxoSet()
	if err := utxoSet.LoadBlockUtxos(block, true, bpos.chain.DB()); err != nil {
		return err
	}
	blockCopy := block.Copy()
	bpos.chain.SplitBlockOutputs(blockCopy)
	if err := utxoSet.ApplyBlock(blockCopy); err != nil {
		return err
	}
	receipts, gasUsed, _, utxoTxs, err :=
		bpos.chain.StateProcessor().Process(block, statedb, utxoSet)
	if err != nil {
		return err
	}

	// block.Txs[0].Vout[0].Value -= gasRemainingFee
	bpos.chain.UpdateNormalTxBalanceState(blockCopy, utxoSet, statedb)

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
	logger.Infof("After execute block.statedb root: %s utxo root: %s genesis contract balance: %d block height: %d",
		statedb.RootHash(), statedb.UtxoRoot(), statedb.GetBalance(chain.ContractAddr).Uint64(), block.Header.Height)
	if genesisContractBalanceOld+block.Txs[0].Vout[0].Value != statedb.GetBalance(chain.ContractAddr).Uint64() {
		return errors.New("genesis contract state is error")
	}

	bpos.chain.UtxoSetCache()[block.Header.Height] = utxoSet

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
	if len(receipts) > 0 {
		block.Header.ReceiptHash = *receipts.Hash()
		bpos.chain.ReceiptsCache()[block.Header.Height] = receipts
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
	bookkeepers := bpos.context.dynasty.peers

	return bpos.net.BroadcastToBookkeepers(messageID, prepareBlockMsg, bookkeepers)
}

// verifyCandidates vefiry if the block candidates hash is right.
func (bpos *Bpos) verifyDynasty(block *types.Block) error {

	dynasty, err := bpos.fetchDynastyByHeight(block.Header.Height)
	dynastyBytes, err := json.Marshal(dynasty)
	if err != nil {
		return err
	}
	dynastyHash := crypto.DoubleHashH(dynastyBytes)
	if (&block.Header.DynastyHash).IsEqual(&dynastyHash) {
		return ErrInvalidDynastyHash
	}

	return nil
}

// verifyIrreversibleInfo vefiry if the block irreversibleInfo is right.
func (bpos *Bpos) verifyIrreversibleInfo(block *types.Block) error {

	irreversibleInfo := block.IrreversibleInfo
	if irreversibleInfo != nil {
		dynasty, err := bpos.fetchDynastyByHeight(block.Header.Height)
		if err != nil {
			return err
		}
		if len(irreversibleInfo.Signatures) <= 2*len(dynasty.delegates)/3 {
			return errors.New("the number of irreversibleInfo signatures is not enough")
		}
		// check hash is exist
		// block, _ := bpos.chain.LoadBlockByHash(irreversibleInfo.Hash)
		// if block == nil {
		// 	logger.Warnf("Invalid irreversible info. The block hash %s is not exist.", irreversibleInfo.Hash.String())
		// 	return ErrInvalidHashInIrreversibleInfo
		// }
		//TODO: period switching requires extra processing

		addrs := dynasty.addrs
		remains := []types.AddressHash{}
		for _, v := range irreversibleInfo.Signatures {
			if pubkey, ok := crypto.RecoverCompact(irreversibleInfo.Hash[:], v); ok {
				addrPubKeyHash, err := types.NewAddressFromPubKey(pubkey)
				if err != nil {
					return err
				}
				addr := *addrPubKeyHash.Hash160()
				if util.InArray(addr, addrs) {
					if !util.InArray(addr, remains) {
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
		if len(remains) <= 2*len(dynasty.delegates)/3 {
			logger.Errorf("Invalid irreversible info in block. Hash: %s, Height: %d, remains: %d", block.BlockHash().String(), block.Header.Height, len(remains))
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
	dynasty, err := bpos.fetchDynastyByHeight(block.Header.Height)
	if err != nil {
		return false, err
	}
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

// TryToUpdateEternalBlock try to update eternal block.
func (bpos *Bpos) TryToUpdateEternalBlock(src *types.Block) {
	irreversibleInfo := src.IrreversibleInfo
	dynasty, err := bpos.fetchDynastyByHeight(src.Header.Height)
	if err != nil {
		return
	}
	if irreversibleInfo != nil && len(irreversibleInfo.Signatures) > 2*len(dynasty.delegates)/3 {
		block, err := chain.LoadBlockByHash(irreversibleInfo.Hash, bpos.chain.DB())
		if err != nil {
			logger.Warnf("Failed to update eternal block. Err: %s", err.Error())
			return
		}
		bpos.bftservice.updateEternal(block)
	}
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
