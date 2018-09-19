// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"bytes"
	"errors"
	"io"

	corepb "github.com/BOXFoundation/Quicksilver/core/pb"
	"github.com/BOXFoundation/Quicksilver/crypto"
	"github.com/BOXFoundation/Quicksilver/util"
	proto "github.com/gogo/protobuf/proto"
)

// Define error
var (
	ErrSerializeHeader                = errors.New("Serialize block header error")
	ErrEmptyProtoMessage              = errors.New("Empty proto message")
	ErrInvalidBlockHeaderProtoMessage = errors.New("Invalid block header proto message")
	ErrInvalidBlockProtoMessage       = errors.New("Invalid block proto message")
)

// MaxBlockHeaderPayload is the maximum number of bytes a block header can be.
// version 4 bytes + timestamp 4 bytes + bits 4 bytes + nonce 4 bytes +
// prevBlock and merkleRoot hashes + dposContextRoot.
// TODO: fill in dposContextRoot length
const MaxBlockHeaderPayload = 16 + (crypto.HashSize * 2) + 0

// BlockHeader defines information about a block and is used in the
// block (MsgBlock) and headers (MsgHeaders) messages.
type BlockHeader struct {
	// Version of the block.  This is not the same as the protocol version.
	Version int32

	// Hash of the previous block header in the block chain.
	PrevBlockHash crypto.HashType

	// Merkle tree reference to hash of all transactions for the block.
	TxsRoot crypto.HashType

	DposContextRoot DposContext

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	TimeStamp int64

	// Distinguish between mainnet and testnet.
	Magic uint32
}

func (h *BlockHeader) blockHash() crypto.HashType {
	// Encode the header and double sha256 everything prior to the number of
	// transactions.  Ignore the error returns since there is no way the
	// encode could fail except being out of memory which would cause a
	// run-time panic.
	buf := bytes.NewBuffer(make([]byte, 0, MaxBlockHeaderPayload))
	_ = writeBlockHeader(buf, 0, h)

	return crypto.DoubleSha256(buf.Bytes())
}

// writeBlockHeader writes a block header to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func writeBlockHeader(w io.Writer, pver uint32, bh *BlockHeader) error {
	sec := uint32(bh.timestamp.Unix())
	return util.WriteElements(w, bh.version, &bh.prevBlock, &bh.dposContextRoot, &bh.merkleRoot,
		sec, bh.bits, bh.nonce)
}

// DposContext define struct
type DposContext struct {
	// TODO: fill in here or some other package
}

// MsgBlock defines a block containing header and transactions within it.
type MsgBlock struct {
	Header *BlockHeader
	Txs    []*MsgTx
}

// Serialize block header to proto message.
func (header *BlockHeader) Serialize() (proto.Message, error) {

	return &corepb.BlockHeader{
		Version:       header.Version,
		PrevBlockHash: header.PrevBlockHash[:],
		TxsRoot:       header.TxsRoot[:],
		TimeStamp:     header.TimeStamp,
		Magic:         header.Magic,
	}, nil
}

// Deserialize convert proto message to block header.
func (header *BlockHeader) Deserialize(message proto.Message) error {

	if message, ok := message.(*corepb.BlockHeader); ok {
		if message != nil {
			header.Version = message.Version
			copy(header.PrevBlockHash[:], message.PrevBlockHash[:])
			copy(header.TxsRoot[:], message.TxsRoot[:])
			header.TimeStamp = message.TimeStamp
			header.Magic = message.Magic
			return nil
		}
		return ErrEmptyProtoMessage
	}

	return ErrInvalidBlockHeaderProtoMessage
}

// Serialize block to proto message.
func (msgBlock *MsgBlock) Serialize() (proto.Message, error) {

	header, _ := msgBlock.Header.Serialize()
	if header, ok := header.(*corepb.BlockHeader); ok {
		var txs []*corepb.MsgTx
		for _, v := range msgBlock.Txs {
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
			msgBlock.Header = header
			msgBlock.Txs = txs
			return nil
		}
		return ErrEmptyProtoMessage
	}

	return ErrInvalidBlockProtoMessage
}

// BlockHash calculates hash of the block
func (b *Block) BlockHash() crypto.HashType {
	return b.header.blockHash()
}
