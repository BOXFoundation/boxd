package netsync

import (
	"errors"
	"fmt"

	corepb "github.com/BOXFoundation/boxd/core/pb"
	coreTypes "github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	netsyncpb "github.com/BOXFoundation/boxd/netsync/pb"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	"github.com/gogo/protobuf/proto"
)

var (
	ErrEmptyProtoMessage   = errors.New("Empty proto message")
	ErrInvalidProtoMessage = errors.New("Invalid proto message")

	_ conv.Convertible  = (*LocateHeaders)(nil)
	_ conv.Serializable = (*LocateHeaders)(nil)
	_ conv.Convertible  = (*SyncHeaders)(nil)
	_ conv.Serializable = (*SyncHeaders)(nil)
	_ conv.Convertible  = (*CheckHash)(nil)
	_ conv.Serializable = (*CheckHash)(nil)
	_ conv.Convertible  = (*SyncCheckHash)(nil)
	_ conv.Serializable = (*SyncCheckHash)(nil)
	_ conv.Convertible  = (*FetchBlocksHeaders)(nil)
	_ conv.Serializable = (*FetchBlocksHeaders)(nil)
	_ conv.Convertible  = (*SyncBlocks)(nil)
	_ conv.Serializable = (*SyncBlocks)(nil)
)

// LocateHeaders includes headers sended to a peer to locate checkpoint
// in the peer's chain
type LocateHeaders struct {
	Headers []*crypto.HashType
}

// SyncHeaders includes the headers that local node need to sync with the
// peer's chain. SyncHeaders may contains overlapped block headers
// with local chain
type SyncHeaders struct {
	Headers []*coreTypes.BlockHeader
}

// CheckHash defines information about synchronizing check hash with
// the peer's chain, it only need to check Length headers.
// remote peer will send a root hash from the corresponding Headers
type CheckHash struct {
	BeginHash *crypto.HashType
	Length    uint32
}

// SyncCheckHash defines information about root hash for check in sync
// scenario. the RootHash is the hash for headers that CheckHash indicate
type SyncCheckHash struct {
	RootHash *crypto.HashType
}

// FetchBlocksHeaders includes headers sended to a sync peer to
// fetch blocks
type FetchBlocksHeaders struct {
	LocateHeaders
}

// SyncBlocks includes blocks sended from synchronized peer to local node
type SyncBlocks struct {
	Blocks []*coreTypes.Block
}

// ToProtoMessage converts LocateHeaders to proto message.
func (lh *LocateHeaders) ToProtoMessage() (proto.Message, error) {
	return &netsyncpb.LocateHeaders{
		Headers: ConvHashesToBytesArray(lh.Headers),
	}, nil
}

// FromProtoMessage converts proto message to LocateHeaders
func (lh *LocateHeaders) FromProtoMessage(message proto.Message) error {
	if m, ok := message.(*netsyncpb.LocateHeaders); ok {
		if m != nil {
			var err error
			lh.Headers, err = ConvBytesArrayToHashes(m.Headers)
			if err != nil {
				logger.Info(err.Error())
				return ErrInvalidProtoMessage
			}
			return nil
		}
		return ErrEmptyProtoMessage
	}
	return ErrInvalidProtoMessage
}

// Marshal method marshal LocateHeaders object to binary
func (lh *LocateHeaders) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(lh)
}

// Unmarshal method unmarshal binary data to LocateHeaders object
func (lh *LocateHeaders) Unmarshal(data []byte) error {
	msg := &netsyncpb.LocateHeaders{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return lh.FromProtoMessage(msg)
}

// ToProtoMessage converts SyncHeaders to proto message.
func (sh *SyncHeaders) ToProtoMessage() (proto.Message, error) {
	headers, err := ConvHeadersToPbHeaders(sh.Headers)
	if err != nil {
		return nil, err
	}
	return &netsyncpb.SyncHeaders{
		Headers: headers,
	}, nil
}

// FromProtoMessage converts proto message to SyncHeaders
func (sh *SyncHeaders) FromProtoMessage(message proto.Message) error {
	if m, ok := message.(*netsyncpb.SyncHeaders); ok {
		if m != nil {
			var err error
			sh.Headers, err = ConvPbHeadersToHeaders(m.Headers)
			if err != nil {
				logger.Info(err.Error())
				return ErrInvalidProtoMessage
			}
			return nil
		}
		return ErrEmptyProtoMessage
	}
	return ErrInvalidProtoMessage
}

// Marshal method marshal SyncHeaders object to binary
func (sh *SyncHeaders) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(sh)
}

// Unmarshal method unmarshal binary data to SyncHeaders object
func (sh *SyncHeaders) Unmarshal(data []byte) error {
	msg := &netsyncpb.SyncHeaders{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return sh.FromProtoMessage(msg)
}

// ToProtoMessage converts CheckHash to proto message.
func (ch *CheckHash) ToProtoMessage() (proto.Message, error) {
	pbCh := new(netsyncpb.CheckHash)
	pbCh.BeginHash = make([]byte, crypto.HashSize)
	copy(pbCh.BeginHash[:], (*ch.BeginHash)[:])
	pbCh.Length = ch.Length
	return pbCh, nil
}

// FromProtoMessage converts proto message to CheckHash
func (ch *CheckHash) FromProtoMessage(message proto.Message) error {
	if m, ok := message.(*netsyncpb.CheckHash); ok {
		if m != nil {
			copy((ch.BeginHash)[:], m.BeginHash[:])
			ch.Length = m.Length
			return nil
		}
		return ErrEmptyProtoMessage
	}
	return ErrInvalidProtoMessage
}

// Marshal method marshal CheckHash object to binary
func (ch *CheckHash) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(ch)
}

// Unmarshal method unmarshal binary data to CheckHash object
func (ch *CheckHash) Unmarshal(data []byte) error {
	msg := &netsyncpb.CheckHash{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return ch.FromProtoMessage(msg)
}

// ToProtoMessage converts SyncCheckHash to proto message.
func (sch *SyncCheckHash) ToProtoMessage() (proto.Message, error) {
	pbSch := new(netsyncpb.SyncCheckHash)
	pbSch.RootHash = make([]byte, crypto.HashSize)
	copy(pbSch.RootHash[:], (*sch.RootHash)[:])
	return pbSch, nil
}

// FromProtoMessage converts proto message to SyncCheckHash
func (sch *SyncCheckHash) FromProtoMessage(message proto.Message) error {
	if m, ok := message.(*netsyncpb.SyncCheckHash); ok {
		if m != nil {
			copy((sch.RootHash)[:], m.RootHash[:])
			return nil
		}
		return ErrEmptyProtoMessage
	}
	return ErrInvalidProtoMessage
}

// Marshal method marshal SyncCheckHash object to binary
func (sch *SyncCheckHash) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(sch)
}

// Unmarshal method unmarshal binary data to SyncCheckHash object
func (sch *SyncCheckHash) Unmarshal(data []byte) error {
	msg := &netsyncpb.SyncCheckHash{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return err
	}
	return sch.FromProtoMessage(msg)
}

// ToProtoMessage converts SyncBlocks to proto message.
func (sb *SyncBlocks) ToProtoMessage() (proto.Message, error) {
	blocks, err := ConvBlocksToPbBlocks(sb.Blocks)
	if err != nil {
		return nil, err
	}
	return &netsyncpb.SyncBlocks{Blocks: blocks}, nil
}

// FromProtoMessage converts proto message to SyncBlocks
func (sb *SyncBlocks) FromProtoMessage(message proto.Message) error {
	if m, ok := message.(*netsyncpb.SyncBlocks); ok {
		if m != nil {
			var err error
			sb.Blocks, err = ConvPbBlocksToBlocks(m.Blocks)
			if err != nil {
				logger.Info(err.Error())
				return ErrInvalidProtoMessage
			}
			return nil
		}
		return ErrEmptyProtoMessage
	}
	return ErrInvalidProtoMessage
}

// Marshal method marshal SyncBlocks object to binary
func (sb *SyncBlocks) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(sb)
}

// Unmarshal method unmarshal binary data to SyncBlocks object
func (sb *SyncBlocks) Unmarshal(data []byte) error {
	msg := &netsyncpb.SyncBlocks{}
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
			return nil, fmt.Errorf("netsync FromProtoMessage] LocateHeaders contains " +
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

// ConvPbHeadersToHeaders convert []*corepb.Block to []*coreTypes.Block
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
