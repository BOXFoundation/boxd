// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"errors"

	corepb "github.com/BOXFoundation/Quicksilver/core/pb"
	"github.com/BOXFoundation/Quicksilver/crypto"
	proto "github.com/gogo/protobuf/proto"
)

// Define error
var (
	ErrSerializeHeader                = errors.New("Serialize block header error")
	ErrEmptyProtoMessage              = errors.New("Empty proto message")
	ErrInvalidBlockHeaderProtoMessage = errors.New("Invalid block header proto message")
	ErrInvalidBlockProtoMessage       = errors.New("Invalid block proto message")
)

// BlockHeader defines information about a block and is used in the
// block (MsgBlock) and headers (MsgHeaders) messages.
type BlockHeader struct {
	// Version of the block.  This is not the same as the protocol version.
	version int32

	// Hash of the previous block header in the block chain.
	prevBlockHash crypto.HashType

	// Merkle tree reference to hash of all transactions for the block.
	txsRoot crypto.HashType

	dposContextRoot DposContext

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	timestamp int64

	coinbase []byte

	// Distinguish between mainnet and testnet.
	magic uint32

	// Nonce used to generate the block.
	nonce uint32
}

// DposContext define struct
type DposContext struct {
	// TODO: fill in here or some other package
}

// MsgBlock defines a block containing header and transactions within it.
type MsgBlock struct {
	header *BlockHeader
	txs    []*MsgTx
}

// Serialize block header to proto message.
func (header *BlockHeader) Serialize() (proto.Message, error) {

	return &corepb.BlockHeader{
		Version:       header.version,
		PrevBlockHash: header.prevBlockHash[:],
		TxsRoot:       header.txsRoot[:],
		Timestamp:     header.timestamp,
		Coinbase:      header.coinbase,
		Magic:         header.magic,
	}, nil
}

// Deserialize convert proto message to block header.
func (header *BlockHeader) Deserialize(message proto.Message) error {

	if message, ok := message.(*corepb.BlockHeader); ok {
		if message != nil {
			header.version = message.Version
			copy(header.prevBlockHash[:], message.PrevBlockHash[:])
			copy(header.txsRoot[:], message.TxsRoot[:])
			header.timestamp = message.Timestamp
			header.coinbase = message.Coinbase
			header.magic = message.Magic
			return nil
		}
		return ErrEmptyProtoMessage
	}

	return ErrInvalidBlockHeaderProtoMessage
}

// Serialize block to proto message.
func (msgBlock *MsgBlock) Serialize() (proto.Message, error) {

	header, _ := msgBlock.header.Serialize()
	if header, ok := header.(*corepb.BlockHeader); ok {
		var txs []*corepb.MsgTx
		for _, v := range msgBlock.txs {
			tx, err := v.Serialize()
			if err != nil {
				return nil, err
			}
			if tx, ok := tx.(*corepb.MsgTx); ok {
				txs = append(txs, tx)
			}
		}
		return &corepb.MsgBlock{
			Header: header,
			Txs:    txs,
		}, nil
	}

	return nil, ErrSerializeHeader
}

// Deserialize convert proto message to block.
func (msgBlock *MsgBlock) Deserialize(message proto.Message) error {

	if message, ok := message.(*corepb.MsgBlock); ok {
		if message != nil {
			header := new(BlockHeader)
			if err := header.Deserialize(message.Header); err != nil {
				return err
			}
			var txs []*MsgTx
			for _, v := range message.Txs {
				tx := new(MsgTx)
				if err := tx.Deserialize(v); err != nil {
					return err
				}
				txs = append(txs, tx)
			}
			msgBlock.header = header
			msgBlock.txs = txs
			return nil
		}
		return ErrEmptyProtoMessage
	}

	return ErrInvalidBlockProtoMessage
}
