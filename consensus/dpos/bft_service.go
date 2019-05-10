// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dpos

import (
	"sync"
	"time"

	chain "github.com/BOXFoundation/boxd/core/chain"
	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/p2p"
	"github.com/BOXFoundation/boxd/util"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jbenet/goprocess"
)

// Define const.
const (
	EternalBlockMsgChBufferSize        = 65536
	MaxEternalBlockMsgCacheTime        = 5
	MinConfirmMsgNumberForEternalBlock = 2 * PeriodSize / 3
)

// BftService use for quick identification of eternal block.
type BftService struct {
	blockPrepareMsgCh chan p2p.Message
	blockCommitMsgCh  chan p2p.Message
	notifiee          p2p.Net
	chain             *chain.BlockChain
	consensus         *Dpos
	// msgCache             *sync.Map
	blockPrepareMsgCache *sync.Map
	blockCommitMsgCache  *sync.Map
	blockPrepareMsgKey   *lru.Cache
	blockCommitMsgKey    *lru.Cache
	proc                 goprocess.Process
}

// NewBftService new bft service for eternalBlockMsg.
func NewBftService(consensus *Dpos) (*BftService, error) {

	bft := &BftService{
		blockPrepareMsgCh:    make(chan p2p.Message, EternalBlockMsgChBufferSize),
		blockCommitMsgCh:     make(chan p2p.Message, EternalBlockMsgChBufferSize),
		notifiee:             consensus.net,
		chain:                consensus.chain,
		consensus:            consensus,
		blockCommitMsgCache:  new(sync.Map),
		blockPrepareMsgCache: new(sync.Map),
		proc:                 goprocess.WithParent(consensus.proc),
	}

	bft.blockPrepareMsgKey, _ = lru.New(64)
	bft.blockCommitMsgKey, _ = lru.New(64)
	return bft, nil
}

// Run bft service to handle eternalBlockMsg.
func (bft *BftService) Run() {
	bft.subscribeMessageNotifiee()
	bft.proc.Go(bft.loop)
	// bft.proc.Go(bft.update)
}

func (bft *BftService) subscribeMessageNotifiee() {
	bft.notifiee.Subscribe(p2p.NewNotifiee(p2p.BlockPrepareMsg, bft.blockPrepareMsgCh))
	bft.notifiee.Subscribe(p2p.NewNotifiee(p2p.BlockCommitMsg, bft.blockCommitMsgCh))
}

func (bft *BftService) loop(p goprocess.Process) {
	logger.Info("Start BftService to quick identification of eternal block...")
	for {
		select {
		case msg := <-bft.blockPrepareMsgCh:
			if err := bft.handleBlockPrepareMsg(msg); err != nil {
				logger.Warnf("Failed to handle eternalBlockMsg. Err: %s", err.Error())
			}
		case msg := <-bft.blockCommitMsgCh:
			if err := bft.handleBlockCommitMsg(msg); err != nil {
				logger.Warnf("Failed to handle eternalBlockMsg. Err: %s", err.Error())
			}
		case <-p.Closing():
			logger.Info("Quit bftservice loop.")
			return
		}
	}
}

// FetchIrreversibleInfo fetch Irreversible block info.
func (bft *BftService) FetchIrreversibleInfo() *types.IrreversibleInfo {

	tailHeight := bft.chain.TailBlock().Header.Height
	BookkeeperRefreshIntervalInSecond := BookkeeperRefreshInterval / SecondInMs
	offset := time.Now().Unix() % BookkeeperRefreshIntervalInSecond

	if tailHeight == 0 {
		return nil
	}
	height := tailHeight
	for offset >= 0 && height > 0 {
		block, err := bft.chain.LoadBlockByHeight(height)
		if err != nil {
			continue
		}
		blockHash := *block.BlockHash()
		if value, ok := bft.blockCommitMsgCache.Load(blockHash); ok {
			signatures := value.([][]byte)
			if len(signatures) > MinConfirmMsgNumberForEternalBlock {
				// go bft.updateEternal(block)
				irreversibleInfo := new(types.IrreversibleInfo)
				irreversibleInfo.Hash = blockHash
				irreversibleInfo.Signatures = value.([][]byte)
				bft.blockCommitMsgCache.Delete(blockHash)
				return irreversibleInfo
			}
		}
		height--
		offset--
	}
	if offset == BookkeeperRefreshIntervalInSecond-1 {
		bft.blockCommitMsgCache = &sync.Map{}
	}

	return nil
}

// // update eternal block.
// func (bft *BftService) update(p goprocess.Process) {
// 	timerChan := time.NewTicker(time.Second)
// 	defer timerChan.Stop()
// 	for {
// 		select {
// 		case <-timerChan.C:
// 			bft.maybeUpdateEternalBlock()
// 		case <-p.Closing():
// 			logger.Info("Quit update eternalBlock loop.")
// 			return
// 		}
// 	}
// }

// func (bft *BftService) maybeUpdateEternalBlock() {
// 	if bft.chain.TailBlock().Header.Height-bft.chain.EternalBlock().Header.Height > MinConfirmMsgNumberForEternalBlock*BlockNumPerPeiod {
// 		block, err := bft.chain.LoadBlockByHeight(bft.chain.EternalBlock().Header.Height + 1)
// 		if err != nil {
// 			logger.Errorf("Failed to update eternal block. LoadBlockByHeight occurs error: %s", err.Error())
// 		} else {
// 			bft.updateEternal(block)
// 		}
// 	}
// }

func (bft *BftService) updateEternal(block *types.Block) {

	if block.Header.Height <= bft.chain.EternalBlock().Header.Height {
		//logger.Warnf("No need to update eternal block because the height is lower than current eternal block height")
		return
	}
	if err := bft.chain.SetEternal(block); err != nil {
		logger.Info("Failed to update eternal block.Hash: %s, Height: %d, Err: %s",
			block.BlockHash().String(), block.Header.Height, err.Error())
		return
	}
	logger.Infof("Eternal block has changed! Hash: %s Height: %d", block.BlockHash(), block.Header.Height)

}

func (bft *BftService) handleBlockPrepareMsg(msg p2p.Message) error {

	// preCheck
	eternalBlockMsg, block, err := bft.preCheck(msg)
	if err != nil || eternalBlockMsg == nil {
		logger.Warnf("Failed to handle block prepare msg. Err: %v", err)
		return err
	}

	key := eternalBlockMsg.Hash
	signature := eternalBlockMsg.Signature

	if pubkey, ok := crypto.RecoverCompact(eternalBlockMsg.Hash[:], signature); ok {
		addrPubKeyHash, err := types.NewAddressFromPubKey(pubkey)
		if err != nil {
			return err
		}
		addr := *addrPubKeyHash.Hash160()
		var period *Period
		for _, v := range bft.consensus.context.periodContext.period {
			if v.addr == addr && msg.From().Pretty() == v.peerID {
				period = v
			}
		}
		if period == nil {
			return ErrIllegalMsg
		}

		if msg, ok := bft.blockPrepareMsgCache.Load(key); ok {
			value := msg.([][]byte)
			if util.InArray(signature, value) {
				return nil
			}
			value = append(value, signature)
			bft.blockPrepareMsgCache.Store(key, value)
			if len(value) > MinConfirmMsgNumberForEternalBlock {
				bft.blockCommitMsgKey.Add(key, key)
				bft.consensus.BroadcastBFTMsgToBookkeepers(block, p2p.BlockCommitMsg)
				bft.blockPrepareMsgCache.Delete(key)
			}
		} else {
			bft.blockPrepareMsgCache.Store(key, [][]byte{signature})
		}
	}

	return nil
}

func (bft *BftService) handleBlockCommitMsg(msg p2p.Message) error {
	// preCheck
	eternalBlockMsg, block, err := bft.preCheck(msg)
	if err != nil || eternalBlockMsg == nil {
		logger.Warnf("Failed to handle block commit msg. Err: %v", err)
		return err
	}

	key := eternalBlockMsg.Hash
	signature := eternalBlockMsg.Signature
	if pubkey, ok := crypto.RecoverCompact(key[:], signature); ok {
		addrPubKeyHash, err := types.NewAddressFromPubKey(pubkey)
		if err != nil {
			return err
		}
		addr := *addrPubKeyHash.Hash160()
		var period *Period
		for _, v := range bft.consensus.context.periodContext.period {
			if v.addr == addr && msg.From().Pretty() == v.peerID {
				period = v
			}
		}
		if period == nil {
			return ErrIllegalMsg
		}

		if msg, ok := bft.blockCommitMsgCache.Load(key); ok {
			value := msg.([][]byte)
			if util.InArray(signature, value) {
				return nil
			}
			value = append(value, signature)
			bft.blockCommitMsgCache.Store(key, value)
			if len(value) > MinConfirmMsgNumberForEternalBlock {
				bft.blockCommitMsgKey.Add(key, key)
				// receive more than 2/3 block commit msg. update local eternal block.
				bft.updateEternal(block)
			}
		} else {
			bft.blockCommitMsgCache.Store(key, [][]byte{signature})
		}
	}

	return nil
}

func (bft *BftService) preCheck(msg p2p.Message) (*EternalBlockMsg, *types.Block, error) {
	// quick check
	peerID := msg.From().Pretty()
	if !util.InArray(peerID, bft.consensus.context.periodContext.periodPeers) {
		return nil, nil, ErrNotBookkeeperPeer
	}

	eternalBlockMsg := new(EternalBlockMsg)
	if err := eternalBlockMsg.Unmarshal(msg.Body()); err != nil {
		return nil, nil, err
	}

	key := eternalBlockMsg.Hash
	if bft.blockCommitMsgKey.Contains(key) {
		logger.Debugf("Enough eternalBlockMsgs has been received.")
		return nil, nil, nil
	}

	block, err := bft.chain.LoadBlockByHash(key)
	if err != nil {
		return nil, nil, err
	}
	// block height is lower than current eternal block.
	if block.Header.Height < bft.chain.EternalBlock().Header.Height {
		return nil, nil, nil
	}

	now := time.Now().Unix()
	if eternalBlockMsg.Timestamp > now || now-eternalBlockMsg.Timestamp > MaxEternalBlockMsgCacheTime {
		return nil, nil, ErrIllegalMsg
	}

	return eternalBlockMsg, block, nil
}
