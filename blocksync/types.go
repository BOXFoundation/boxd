// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package blocksync

import (
	"errors"
	"fmt"

	"github.com/BOXFoundation/boxd/blocksync/pb"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	coreTypes "github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	"github.com/gogo/protobuf/proto"
)

var (
	errEmptyProtoMessage   = errors.New("Empty proto message")
	errInvalidProtoMessage = errors.New("Invalid proto message")
	errNilReceiver         = errors.New("Nil receiver")

	_ conv.Convertible  = (*LocateHeaders)(nil)
	_ conv.Serializable = (*LocateHeaders)(nil)
	_ conv.Convertible  = (*SyncHeaders)(nil)
	_ conv.Serializable = (*SyncHeaders)(nil)
	_ conv.Convertible  = (*CheckHash)(nil)
	_ conv.Serializable = (*CheckHash)(nil)
	_ conv.Convertible  = (*SyncCheckHash)(nil)
	_ conv.Serializable = (*SyncCheckHash)(nil)
	_ conv.Convertible  = (*FetchBlockHeaders)(nil)
	_ conv.Serializable = (*FetchBlockHeaders)(nil)
	_ conv.Convertible  = (*SyncBlocks)(nil)
	_ conv.Serializable = (*SyncBlocks)(nil)
)

// LocateHeaders includes hashes sent to a peer to locate fork point
// in the peer's chain
type LocateHeaders struct {
	Hashes []*crypto.HashType
}

// SyncHeaders includes the hashes that local node needs to sync with the
// peer's chain. SyncHeaders may contain overlapped block hashes
// with local chain
type SyncHeaders struct {
	LocateHeaders
}

// CheckHash defines information about synchronizing check hash with
// the peer's chain, it only needs to check Length headers.
// Remote peer will send a root hash from the corresponding Headers
type CheckHash struct {
	BeginHash *crypto.HashType
	Length    int32
}

// SyncCheckHash defines information about root hash for check in sync
// scenario. the RootHash is the hash for headers that CheckHash indicate
type SyncCheckHash struct {
	RootHash *crypto.HashType
}

// FetchBlockHeaders includes headers sent to a sync peer to fetch blocks
type FetchBlockHeaders struct {
	// the index to indicate which hashes chunk in sync hashes
	// it is used to find out which hashes chunk needed to re-sync when
	// error happen in the sync with some remote peer
	Idx int32
	CheckHash
}

// SyncBlocks includes blocks sent from synchronized peer to local node
type SyncBlocks struct {
	// the index to indicate which hashes chunk in sync hashes
	// it is used to find out which hashes chunk needed to re-sync when
	// error happen in the sync with some remote peer
	Idx    int32
	Blocks []*coreTypes.Block
}

func newLocateHeaders(hashes ...*crypto.HashType) *LocateHeaders {
	if hashes == nil {
		hashes = make([]*crypto.HashType, 0)
	}
	return &LocateHeaders{Hashes: hashes}
}

func newSyncHeaders(hashes ...*crypto.HashType) *SyncHeaders {
	return &SyncHeaders{LocateHeaders: *newLocateHeaders(hashes...)}
}

func newCheckHash(hash *crypto.HashType, len int) *CheckHash {
	if hash == nil {
		hash = &crypto.HashType{}
	}
	return &CheckHash{BeginHash: hash, Length: int32(len)}
}

func newSyncCheckHash(hash *crypto.HashType) *SyncCheckHash {
	if hash == nil {
		hash = &crypto.HashType{}
	}
	return &SyncCheckHash{RootHash: hash}
}

func newFetchBlockHeaders(idx int32, hash *crypto.HashType, len int) *FetchBlockHeaders {
	return &FetchBlockHeaders{Idx: idx, CheckHash: *newCheckHash(hash, len)}
}

func newSyncBlocks(idx int32, blocks ...*coreTypes.Block) *SyncBlocks {
	if blocks == nil {
		blocks = make([]*coreTypes.Block, 0)
	}
	return &SyncBlocks{Idx: idx, Blocks: blocks}
}

// ToProtoMessage converts LocateHeaders to proto message.
func (lh *LocateHeaders) ToProtoMessage() (proto.Message, error) {
	if lh == nil {
		lh = newLocateHeaders()
	}
	return &pb.LocateHeaders{
		Hashes: ConvHashesToBytesArray(lh.Hashes),
	}, nil
}

// ToProtoMessage converts SyncHeaders to proto message.
func (sh *SyncHeaders) ToProtoMessage() (proto.Message, error) {
	if sh == nil {
		sh = newSyncHeaders()
	}
	return &pb.SyncHeaders{
		Hashes: ConvHashesToBytesArray(sh.Hashes),
	}, nil
}

// ToProtoMessage converts FetchBlockHeaders to proto message.
func (fbh *FetchBlockHeaders) ToProtoMessage() (proto.Message, error) {
	if fbh == nil {
		fbh = newFetchBlockHeaders(0, nil, 0)
	}
	pbFbh := new(pb.FetchBlockHeaders)
	pbFbh.BeginHash = make([]byte, crypto.HashSize)
	copy(pbFbh.BeginHash[:], (*fbh.BeginHash)[:])
	pbFbh.Length = fbh.Length
	pbFbh.Idx = fbh.Idx
	return pbFbh, nil
}

// FromProtoMessage converts proto message to LocateHeaders
func (lh *LocateHeaders) FromProtoMessage(message proto.Message) error {
	if lh == nil {
		lh = newLocateHeaders()
	}
	if m, ok := message.(*pb.LocateHeaders); ok {
		if m != nil {
			var err error
			lh.Hashes, err = ConvBytesArrayToHashes(m.Hashes)
			if err != nil {
				logger.Error(err.Error())
				return errInvalidProtoMessage
			}
			return nil
		}
		return errEmptyProtoMessage
	}
	return errInvalidProtoMessage
}

// FromProtoMessage converts proto message to SyncHeaders
func (sh *SyncHeaders) FromProtoMessage(message proto.Message) error {
	if sh == nil {
		sh = newSyncHeaders()
	}
	if m, ok := message.(*pb.SyncHeaders); ok {
		if m != nil {
			var err error
			sh.Hashes, err = ConvBytesArrayToHashes(m.Hashes)
			if err != nil {
				logger.Error(err.Error())
				return errInvalidProtoMessage
			}
			return nil
		}
		return errEmptyProtoMessage
	}
	return errInvalidProtoMessage
}

// FromProtoMessage converts proto message to FetchBlockHeaders
func (fbh *FetchBlockHeaders) FromProtoMessage(message proto.Message) error {
	if fbh == nil {
		fbh = newFetchBlockHeaders(0, nil, 0)
	}
	if fbh.BeginHash == nil {
		fbh.BeginHash = &crypto.HashType{}
	}
	if m, ok := message.(*pb.FetchBlockHeaders); ok {
		if m != nil {
			fbh.Idx = m.Idx
			copy(fbh.BeginHash[:], m.BeginHash[:])
			fbh.Length = m.Length
			return nil
		}
		return errEmptyProtoMessage
	}
	return errInvalidProtoMessage
}

// Marshal method marshal LocateHeaders object to binary
func (lh *LocateHeaders) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(lh)
}

// Marshal method marshal LocateHeaders object to binary
func (sh *SyncHeaders) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(sh)
}

// Marshal method marshal FetchBlockHeaders object to binary
func (fbh *FetchBlockHeaders) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(fbh)
}

// Unmarshal method unmarshal binary data to LocateHeaders object
func (lh *LocateHeaders) Unmarshal(data []byte) error {
	msg := &pb.LocateHeaders{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return lh.FromProtoMessage(msg)
}

// Unmarshal method unmarshal binary data to SyncHeaders object
func (sh *SyncHeaders) Unmarshal(data []byte) error {
	msg := &pb.SyncHeaders{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return sh.FromProtoMessage(msg)
}

// Unmarshal method unmarshal binary data to FetchBlockHeaders object
func (fbh *FetchBlockHeaders) Unmarshal(data []byte) error {
	msg := &pb.FetchBlockHeaders{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return fbh.FromProtoMessage(msg)
}

// ToProtoMessage converts CheckHash to proto message.
func (ch *CheckHash) ToProtoMessage() (proto.Message, error) {
	if ch == nil {
		ch = newCheckHash(nil, 0)
	}
	pbCh := new(pb.CheckHash)
	pbCh.BeginHash = make([]byte, crypto.HashSize)
	copy(pbCh.BeginHash[:], (*ch.BeginHash)[:])
	pbCh.Length = ch.Length
	return pbCh, nil
}

// FromProtoMessage converts proto message to CheckHash
func (ch *CheckHash) FromProtoMessage(message proto.Message) error {
	if ch == nil {
		ch = newCheckHash(nil, 0)
	}
	if ch.BeginHash == nil {
		ch.BeginHash = &crypto.HashType{}
	}
	if m, ok := message.(*pb.CheckHash); ok {
		if m != nil {
			copy(ch.BeginHash[:], m.BeginHash[:])
			ch.Length = m.Length
			return nil
		}
		return errEmptyProtoMessage
	}
	return errInvalidProtoMessage
}

// Marshal method marshal CheckHash object to binary
func (ch *CheckHash) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(ch)
}

// Unmarshal method unmarshal binary data to CheckHash object
func (ch *CheckHash) Unmarshal(data []byte) error {
	msg := &pb.CheckHash{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return ch.FromProtoMessage(msg)
}

// ToProtoMessage converts SyncCheckHash to proto message.
func (sch *SyncCheckHash) ToProtoMessage() (proto.Message, error) {
	if sch == nil {
		sch = newSyncCheckHash(nil)
	}
	if sch.RootHash == nil {
		sch.RootHash = &crypto.HashType{}
	}
	pbSch := new(pb.SyncCheckHash)
	pbSch.RootHash = make([]byte, crypto.HashSize)
	copy(pbSch.RootHash[:], (*sch.RootHash)[:])
	return pbSch, nil
}

// FromProtoMessage converts proto message to SyncCheckHash
func (sch *SyncCheckHash) FromProtoMessage(message proto.Message) error {
	if sch == nil {
		sch = newSyncCheckHash(nil)
	}
	if sch.RootHash == nil {
		sch.RootHash = &crypto.HashType{}
	}
	if m, ok := message.(*pb.SyncCheckHash); ok {
		if m != nil {
			copy((sch.RootHash)[:], m.RootHash[:])
			return nil
		}
		return errEmptyProtoMessage
	}
	return errInvalidProtoMessage
}

// Marshal method marshal SyncCheckHash object to binary
func (sch *SyncCheckHash) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(sch)
}

// Unmarshal method unmarshal binary data to SyncCheckHash object
func (sch *SyncCheckHash) Unmarshal(data []byte) error {
	msg := &pb.SyncCheckHash{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return sch.FromProtoMessage(msg)
}

// ToProtoMessage converts SyncBlocks to proto message.
func (sb *SyncBlocks) ToProtoMessage() (proto.Message, error) {
	if sb == nil {
		sb = newSyncBlocks(0)
	}
	if sb.Blocks == nil {
		sb.Blocks = make([]*coreTypes.Block, 0)
	}
	blocks, err := ConvBlocksToPbBlocks(sb.Blocks)
	if err != nil {
		return nil, err
	}
	return &pb.SyncBlocks{Idx: sb.Idx, Blocks: blocks}, nil
}

// FromProtoMessage converts proto message to SyncBlocks
func (sb *SyncBlocks) FromProtoMessage(message proto.Message) error {
	if sb == nil {
		sb = newSyncBlocks(0)
	}
	if m, ok := message.(*pb.SyncBlocks); ok {
		if m != nil {
			var err error
			sb.Blocks, err = ConvPbBlocksToBlocks(m.Blocks)
			if err != nil {
				logger.Error(err.Error())
				return errInvalidProtoMessage
			}
			sb.Idx = m.Idx
			return nil
		}
		return errEmptyProtoMessage
	}
	return errInvalidProtoMessage
}

// Marshal method marshal SyncBlocks object to binary
func (sb *SyncBlocks) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(sb)
}

// Unmarshal method unmarshal binary data to SyncBlocks object
func (sb *SyncBlocks) Unmarshal(data []byte) error {
	msg := &pb.SyncBlocks{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return sb.FromProtoMessage(msg)
}

// ConvHashesToBytesArray convert []*crypto.HashType to [][]byte
func ConvHashesToBytesArray(hashes []*crypto.HashType) [][]byte {
	bytesArray := make([][]byte, len(hashes))
	for i, v := range hashes {
		bytesArray[i] = make([]byte, crypto.HashSize)
		copy(bytesArray[i][:], (*v)[:])
	}
	return bytesArray
}

// ConvBytesArrayToHashes convert [][]byte to []*crypto.HashType
func ConvBytesArrayToHashes(bytesArray [][]byte) ([]*crypto.HashType, error) {
	hashes := make([]*crypto.HashType, len(bytesArray))
	for i, v := range bytesArray {
		if len(v) != crypto.HashSize {
			return nil, fmt.Errorf("FromProtoMessage] LocateHeaders contains " +
				"hash whoes length is not HashSize(32) bytes")
		}
		hashes[i] = &crypto.HashType{}
		copy(hashes[i][:], v[:])
	}
	return hashes, nil
}

// ConvHeadersToPbHeaders convert []*coreTypes.BlockHeader to
// []*corepb.BlockHeader
func ConvHeadersToPbHeaders(headers []*coreTypes.BlockHeader) (
	[]*corepb.BlockHeader, error) {
	pbHeaders := make([]*corepb.BlockHeader, 0, len(headers))
	for _, v := range headers {
		msg, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		hmsg, ok := msg.(*corepb.BlockHeader)
		if !ok {
			return nil, fmt.Errorf("asserted failed for proto.Message to *corepb.BlockHeader")
		}
		pbHeaders = append(pbHeaders, hmsg)
	}
	return pbHeaders, nil
}

// ConvPbHeadersToHeaders convert []*corepb.BlockHeader to
// []*coreTypes.BlockHeader
func ConvPbHeadersToHeaders(pbHeaders []*corepb.BlockHeader) (
	[]*coreTypes.BlockHeader, error) {
	headers := make([]*coreTypes.BlockHeader, 0, len(pbHeaders))
	for _, v := range pbHeaders {
		h := new(coreTypes.BlockHeader)
		if err := h.FromProtoMessage(v); err != nil {
			return nil, err
		}
		headers = append(headers, h)
	}
	return headers, nil
}

// ConvBlocksToPbBlocks convert []*coreTypes.Block to []*corepb.Block
func ConvBlocksToPbBlocks(blocks []*coreTypes.Block) ([]*corepb.Block, error) {
	pbBlocks := make([]*corepb.Block, 0, len(blocks))
	for _, v := range blocks {
		msg, err := v.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		hmsg, ok := msg.(*corepb.Block)
		if !ok {
			return nil, fmt.Errorf("asserted failed for proto.Message to *corepb.Block")
		}
		pbBlocks = append(pbBlocks, hmsg)
	}
	return pbBlocks, nil
}

// ConvPbBlocksToBlocks convert []*corepb.Block to []*coreTypes.Block
func ConvPbBlocksToBlocks(pbBlocks []*corepb.Block) ([]*coreTypes.Block, error) {
	blocks := make([]*coreTypes.Block, 0, len(pbBlocks))
	for _, v := range pbBlocks {
		h := new(coreTypes.Block)
		if err := h.FromProtoMessage(v); err != nil {
			return nil, err
		}
		blocks = append(blocks, h)
	}
	return blocks, nil
}
