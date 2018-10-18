package netsync

import (
	"errors"
	"fmt"

	coreTypes "github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/crypto"
	netsyncpb "github.com/BOXFoundation/boxd/netsync/pb"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	"github.com/gogo/protobuf/proto"
)

var (
	ErrEmptyProtoMessage   = errors.New("Empty proto message")
	ErrInvalidProtoMessage = errors.New("Invalid proto message")
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
	Headers []*crypto.HashType
}

// SyncBlocks includes blocks sended from synchronized peer to local node
type SyncBlocks struct {
	Blocks []*coreTypes.Block
}

// ConvHashesToBytesArray convert []*crypto.HashType to [][]byte
func ConvHashesToBytesArray(hashes []*crypto.HashType) [][]byte {
	bytesArray := make([][]byte, 0, len(hashes))
	for _, v := range hashes {
		bytesArray = append(bytesArray, (*v)[:])
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
