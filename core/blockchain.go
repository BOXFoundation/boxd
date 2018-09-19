// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	corepb "github.com/BOXFoundation/Quicksilver/core/pb"
	"github.com/BOXFoundation/Quicksilver/core/types"
	"github.com/BOXFoundation/Quicksilver/crypto"
	"github.com/BOXFoundation/Quicksilver/log"
	"github.com/BOXFoundation/Quicksilver/p2p"
	"github.com/BOXFoundation/Quicksilver/storage"
	proto "github.com/gogo/protobuf/proto"
	"github.com/jbenet/goprocess"
)

// const defines constants
const (
	BlockMsgChBufferSize = 1024
	Tail                 = "tail_block"
)

var logger log.Logger // logger

func init() {
	logger = log.NewLogger("core")
}

// BlockChain define chain struct
type BlockChain struct {
	notifiee      p2p.Net
	newblockMsgCh chan p2p.Message
	txpool        *TransactionPool
	db            storage.Storage
	genesis       *types.MsgBlock
	tail          *types.MsgBlock
	proc          goprocess.Process

	// Actually a tree-shaped structure where any node can have
	// multiple children.  However, there can only be one active branch (longest) which does
	// indeed form a chain from the tip all the way back to the genesis block.
	hashToBlock map[crypto.HashType]*types.MsgBlock

	// longest chain
	longestChainHeight int
	longestChainTip    *types.MsgBlock

	// orphan block pool
	hashToOrphanBlockmap map[crypto.HashType]*types.MsgBlock
	// orphan block's parents; one parent can have multiple orphan children
	parentToOrphanBlock map[crypto.HashType]*types.MsgBlock
}

// NewBlockChain return a blockchain.
func NewBlockChain(parent goprocess.Process, notifiee p2p.Net, db storage.Storage) (*BlockChain, error) {

	b := &BlockChain{
		notifiee:      notifiee,
		newblockMsgCh: make(chan p2p.Message, BlockMsgChBufferSize),
		proc:          goprocess.WithParent(parent),
		txpool:        NewTransactionPool(parent, notifiee),
		db:            db,
	}

	genesis, err := b.loadGenesis()
	if err != nil {
		return nil, err
	}
	b.genesis = genesis

	tail, err := b.loadTailBlock()
	if err != nil {
		return nil, err
	}
	b.tail = tail

	return b, nil
}

func (chain *BlockChain) loadGenesis() (*types.MsgBlock, error) {

	if ok, _ := chain.db.Has(genesisHash[:]); ok {
		genesis, err := chain.LoadBlockByHashFromDb(genesisHash)
		if err != nil {
			return nil, err
		}
		return genesis, nil
	}

	genesispb, err := genesisBlock.Serialize()
	if err != nil {
		return nil, err
	}
	genesisBin, err := proto.Marshal(genesispb)
	chain.db.Put(genesisHash[:], genesisBin)

	return &genesisBlock, nil
}

func (chain *BlockChain) loadTailBlock() (*types.MsgBlock, error) {

	if ok, _ := chain.db.Has([]byte(Tail)); ok {
		tailBin, err := chain.db.Get([]byte(Tail))
		if err != nil {
			return nil, err
		}

		pbblock := new(corepb.MsgBlock)
		if err := proto.Unmarshal(tailBin, pbblock); err != nil {
			return nil, err
		}

		tail := new(types.MsgBlock)
		if err := tail.Deserialize(pbblock); err != nil {
			return nil, err
		}

		return tail, nil

	}

	tailpb, err := genesisBlock.Serialize()
	if err != nil {
		return nil, err
	}
	tailBin, err := proto.Marshal(tailpb)
	if err != nil {
		return nil, err
	}
	chain.db.Put([]byte(Tail), tailBin)

	return &genesisBlock, nil

}

// LoadBlockByHashFromDb load block by hash from db.
func (chain *BlockChain) LoadBlockByHashFromDb(hash crypto.HashType) (*types.MsgBlock, error) {

	blockBin, err := chain.db.Get(hash[:])
	if err != nil {
		return nil, err
	}

	pbblock := new(corepb.MsgBlock)
	if err := proto.Unmarshal(blockBin, pbblock); err != nil {
		return nil, err
	}

	block := new(types.MsgBlock)
	if err := block.Deserialize(pbblock); err != nil {
		return nil, err
	}

	return block, nil
}

// Run launch blockchain.
func (chain *BlockChain) Run() {

	chain.subscribeMessageNotifiee(chain.notifiee)
	go chain.loop()
	chain.txpool.Run()
}

func (chain *BlockChain) subscribeMessageNotifiee(notifiee p2p.Net) {
	notifiee.Subscribe(p2p.NewNotifiee(p2p.NewBlockMsg, chain.newblockMsgCh))
}

func (chain *BlockChain) loop() {
	for {
		select {
		case msg := <-chain.newblockMsgCh:
			chain.processBlockMsg(msg)
		case <-chain.proc.Closing():
			logger.Info("Quit blockchain loop.")
			return
		}
	}
}

func (chain *BlockChain) processBlockMsg(msg p2p.Message) error {

	body := msg.Body()
	pbblock := new(corepb.MsgBlock)
	if err := proto.Unmarshal(body, pbblock); err != nil {
		return err
	}
	block := new(types.MsgBlock)
	if err := block.Deserialize(pbblock); err != nil {
		return err
	}

	// process block
	chain.processBlock(block)

	return nil
}

// ProcessBlock is the main workhorse for handling insertion of new blocks into
// the block chain.  It includes functionality such as rejecting duplicate
// blocks, ensuring blocks follow all rules, orphan handling, and insertion into
// the block chain along with best chain selection and reorganization.
//
// The first return value indicates if the block is on the main chain.
// The second indicates if the block is an orphan.
func (chain *BlockChain) processBlock(block *types.MsgBlock) (bool, bool, error) {
	blockHash := block.BlockHash()
	logger.Infof("Processing block %v", blockHash)
	return true, true, nil
	// // The block must not already exist in the main chain or side chains.
	// exists, err := b.blockExists(blockHash)
	// if err != nil {
	// 	return false, false, err
	// }
	// if exists {
	// 	str := fmt.Sprintf("already have block %v", blockHash)
	// 	return false, false, ruleError(ErrDuplicateBlock, str)
	// }

	// // The block must not already exist as an orphan.
	// if _, exists := b.orphans[*blockHash]; exists {
	// 	str := fmt.Sprintf("already have block (orphan) %v", blockHash)
	// 	return false, false, ruleError(ErrDuplicateBlock, str)
	// }

	// // Perform preliminary sanity checks on the block and its transactions.
	// err = checkBlockSanity(block, b.chainParams.PowLimit, b.timeSource, flags)
	// if err != nil {
	// 	return false, false, err
	// }
	// // Handle orphan blocks.
	// prevHash := &blockHeader.PrevBlock
	// prevHashExists, err := b.blockExists(prevHash)
	// if err != nil {
	// 	return false, false, err
	// }
	// if !prevHashExists {
	// 	log.Infof("Adding orphan block %v with parent %v", blockHash, prevHash)
	// 	b.addOrphanBlock(block)

	// 	return false, true, nil
	// }

	// // The block has passed all context independent checks and appears sane
	// // enough to potentially accept it into the block chain.
	// isMainChain, err := b.maybeAcceptBlock(block, flags)
	// if err != nil {
	// 	return false, false, err
	// }

	// // Accept any orphan blocks that depend on this block (they are
	// // no longer orphans) and repeat for those accepted blocks until
	// // there are no more.
	// err = b.processOrphans(blockHash, flags)
	// if err != nil {
	// 	return false, false, err
	// }

	// log.Debugf("Accepted block %v", blockHash)

	// return isMainChain, false, nil
}
