// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"github.com/BOXFoundation/boxd/core"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/log"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	proto "github.com/gogo/protobuf/proto"
)

var logger = log.NewLogger("core:types") // logger

// Block defines a block containing block and height that provides easier and more efficient
// manipulation of raw blocks.  It also memoizes hashes for the block and its
// transactions on their first access so subsequent accesses don't have to
// repeat the relatively expensive hashing operations.
type Block struct {
	Hash             *crypto.HashType
	Header           *BlockHeader
	Txs              []*Transaction
	Signature        []byte
	IrreversibleInfo *IrreversibleInfo

	Height uint32
}

var _ conv.Convertible = (*Block)(nil)
var _ conv.Serializable = (*Block)(nil)

// NewBlock new a block from parent.
func NewBlock(parent *Block) *Block {
	return &Block{
		Header: &BlockHeader{
			Magic:         parent.Header.Magic,
			PrevBlockHash: *parent.BlockHash(),
		},
		Txs:    make([]*Transaction, 0),
		Height: parent.Height + 1,
	}
}

// AppendTx appends tx to block
func (block *Block) AppendTx(txs ...*Transaction) *Block {
	block.Txs = append(block.Txs, txs...)
	return block
}

// ToProtoMessage converts block to proto message.
func (block *Block) ToProtoMessage() (proto.Message, error) {

	header, _ := block.Header.ToProtoMessage()

	if header, ok := header.(*corepb.BlockHeader); ok {
		var ii *corepb.IrreversibleInfo
		if block.IrreversibleInfo != nil {
			v, _ := block.IrreversibleInfo.ToProtoMessage()
			ii = v.(*corepb.IrreversibleInfo)
		}

		var txs []*corepb.Transaction
		for _, v := range block.Txs {
			tx, err := v.ToProtoMessage()
			if err != nil {
				return nil, err
			}
			if tx, ok := tx.(*corepb.Transaction); ok {
				txs = append(txs, tx)
			}
		}
		return &corepb.Block{
			Header:           header,
			Txs:              txs,
			Signature:        block.Signature,
			IrreversibleInfo: ii,
			Height:           block.Height,
		}, nil
	}

	return nil, core.ErrSerializeHeader
}

// FromProtoMessage converts proto message to block.
func (block *Block) FromProtoMessage(message proto.Message) error {

	if message, ok := message.(*corepb.Block); ok {
		if message != nil {
			header := new(BlockHeader)
			if err := header.FromProtoMessage(message.Header); err != nil {
				return err
			}
			var ii *IrreversibleInfo
			if message.IrreversibleInfo != nil {
				ii = new(IrreversibleInfo)
				if err := ii.FromProtoMessage(message.IrreversibleInfo); err != nil {
					return err
				}
			}

			var txs []*Transaction
			for _, v := range message.Txs {
				tx := new(Transaction)
				if err := tx.FromProtoMessage(v); err != nil {
					return err
				}
				txs = append(txs, tx)
			}
			block.Header = header
			// Fill in hash after header is set
			block.Hash = block.BlockHash()
			block.Txs = txs
			block.Height = message.Height
			block.Signature = message.Signature
			block.IrreversibleInfo = ii
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return core.ErrInvalidBlockProtoMessage
}

// Copy returns a deep copy: used only for splitBlockOutputs()
// Only copy needed fields to save efforts: height & vin & vout
func (block *Block) Copy() *Block {
	newBlock := &Block{
		Height: block.Height,
	}

	txs := make([]*Transaction, len(block.Txs))
	for k, tx := range block.Txs {
		vin := make([]*TxIn, len(tx.Vin))
		for idx, txIn := range tx.Vin {
			txInCopy := &TxIn{
				PrevOutPoint: txIn.PrevOutPoint,
				ScriptSig:    txIn.ScriptSig,
				Sequence:     txIn.Sequence,
			}
			vin[idx] = txInCopy
		}

		vout := make([]*corepb.TxOut, len(tx.Vout))
		for idx, txOut := range tx.Vout {
			txOutCopy := &corepb.TxOut{
				Value:        txOut.Value,
				ScriptPubKey: txOut.ScriptPubKey,
			}
			vout[idx] = txOutCopy
		}

		txHash, _ := tx.TxHash()
		txCopy := &Transaction{
			hash:     txHash,
			Vin:      vin,
			Vout:     vout,
			Data:     tx.Data,
			Magic:    tx.Magic,
			LockTime: tx.LockTime,
			Version:  tx.Version,
		}
		txs[k] = txCopy
	}

	newBlock.Txs = txs
	return newBlock
}

// Marshal method marshal Block object to binary
func (block *Block) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(block)
}

// Unmarshal method unmarshal binary data to Block object
func (block *Block) Unmarshal(data []byte) error {
	msg := &corepb.Block{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return block.FromProtoMessage(msg)
}

// BlockHash returns the block identifier hash for the Block.
func (block *Block) BlockHash() *crypto.HashType {
	if block.Hash != nil {
		return block.Hash
	}

	// Cache the block hash and return it.
	hash, err := block.calcBlockHash()
	if err != nil {
		logger.Errorf("Failed to get block hash, err = %s", err.Error())
		return nil
	}
	block.Hash = hash
	return hash
}

// BlockHash calculates the block identifier hash for the Block.
func (block *Block) calcBlockHash() (*crypto.HashType, error) {
	headerBuf, err := block.Header.Marshal()
	if err != nil {
		return nil, err
	}
	hash := crypto.DoubleHashH(headerBuf) // dhash of header
	return &hash, nil
}

// BlockHeader defines information about a block and is used in the
// block (Block) and headers (MsgHeaders) messages.
type BlockHeader struct {
	// Version of the block.  This is not the same as the protocol version.
	Version int32

	// Hash of the previous block header in the block chain.
	PrevBlockHash crypto.HashType

	// Merkle tree reference to hash of all transactions for the block.
	TxsRoot crypto.HashType

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	TimeStamp int64

	// Distinguish between mainnet and testnet.
	Magic uint32

	PeriodHash crypto.HashType

	CandidatesHash crypto.HashType
}

var _ conv.Convertible = (*BlockHeader)(nil)
var _ conv.Serializable = (*BlockHeader)(nil)

// ToProtoMessage converts block header to proto message.
func (header *BlockHeader) ToProtoMessage() (proto.Message, error) {

	// todo check error if necessary
	return &corepb.BlockHeader{
		Version:        header.Version,
		PrevBlockHash:  header.PrevBlockHash[:],
		TxsRoot:        header.TxsRoot[:],
		TimeStamp:      header.TimeStamp,
		Magic:          header.Magic,
		PeriodHash:     header.PeriodHash[:],
		CandidatesHash: header.CandidatesHash[:],
	}, nil
}

// FromProtoMessage converts proto message to block header.
func (header *BlockHeader) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*corepb.BlockHeader); ok {
		if message != nil {
			header.Version = message.Version
			copy(header.PrevBlockHash[:], message.PrevBlockHash)
			copy(header.TxsRoot[:], message.TxsRoot)
			header.TimeStamp = message.TimeStamp
			header.Magic = message.Magic
			copy(header.PeriodHash[:], message.PeriodHash)
			copy(header.CandidatesHash[:], message.CandidatesHash)
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return core.ErrInvalidBlockHeaderProtoMessage
}

// Marshal method marshal BlockHeader object to binary
func (header *BlockHeader) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(header)
}

// Unmarshal method unmarshal binary data to BlockHeader object
func (header *BlockHeader) Unmarshal(data []byte) error {
	msg := &corepb.BlockHeader{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return header.FromProtoMessage(msg)
}

var _ conv.Convertible = (*IrreversibleInfo)(nil)
var _ conv.Serializable = (*IrreversibleInfo)(nil)

// IrreversibleInfo defines information about irreversible blocks
type IrreversibleInfo struct {
	Hash       crypto.HashType
	Signatures [][]byte
}

// ToProtoMessage converts IrreversibleInfo to proto message.
func (ii *IrreversibleInfo) ToProtoMessage() (proto.Message, error) {
	return &corepb.IrreversibleInfo{
		Hash:       ii.Hash[:],
		Signatures: ii.Signatures,
	}, nil
}

// FromProtoMessage converts proto message to IrreversibleInfo.
func (ii *IrreversibleInfo) FromProtoMessage(message proto.Message) error {
	if message, ok := message.(*corepb.IrreversibleInfo); ok {
		if message != nil {
			copy(ii.Hash[:], message.Hash[:])
			ii.Signatures = message.Signatures
			return nil
		}
		return core.ErrEmptyProtoMessage
	}

	return core.ErrInvalidOutPointProtoMessage
}

// Marshal method marshal IrreversibleInfo object to binary
func (ii *IrreversibleInfo) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(ii)
}

// Unmarshal method unmarshal binary data to IrreversibleInfo object
func (ii *IrreversibleInfo) Unmarshal(data []byte) error {
	msg := &corepb.IrreversibleInfo{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return ii.FromProtoMessage(msg)
}
