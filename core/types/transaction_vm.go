// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"math/big"

	"github.com/BOXFoundation/boxd/core"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/crypto"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	proto "github.com/gogo/protobuf/proto"
)

// ContractType defines script contract type
type ContractType string

//
const (
	VMVersion = 0

	ContractUnkownType   ContractType = "contract_unkown"
	ContractCreationType ContractType = "contract_creation"
	ContractCallType     ContractType = "contract_call"
)

// VMTransaction defines the transaction used to interact with vm
type VMTransaction struct {
	version  int32
	from     *AddressHash
	to       *AddressHash
	originTx *crypto.HashType
	value    *big.Int
	gasPrice *big.Int
	gas      uint64
	nonce    uint64
	code     []byte
	typ      ContractType
}

// VMTxParams defines BoxTx params parsed from script pubkey
type VMTxParams struct {
	GasPrice uint64
	GasLimit uint64
	Nonce    uint64
	Version  int32
	Code     []byte
	To       *AddressHash
}

// NewVMTransaction new a VMTransaction instance with given parameters
func NewVMTransaction(
	value, gasPrice *big.Int, gas, nonce uint64, hash *crypto.HashType, typ ContractType,
	code []byte,
) *VMTransaction {
	return &VMTransaction{
		version:  VMVersion,
		typ:      typ,
		value:    value,
		originTx: hash,
		gasPrice: gasPrice,
		gas:      gas,
		nonce:    nonce,
		code:     code,
	}
}

// WithFrom sets from
func (tx *VMTransaction) WithFrom(from *AddressHash) *VMTransaction {
	tx.from = from
	return tx
}

// WithTo sets to
func (tx *VMTransaction) WithTo(to *AddressHash) *VMTransaction {
	tx.to = to
	return tx
}

// Version returns the version of the tx.
func (tx *VMTransaction) Version() int32 {
	return tx.version
}

// From returns the tx from addressHash.
func (tx *VMTransaction) From() *AddressHash {
	return tx.from
}

// To returns the tx to addressHash.
func (tx *VMTransaction) To() *AddressHash {
	return tx.to
}

// GasPrice returns the gasprice of the tx.
func (tx *VMTransaction) GasPrice() *big.Int {
	return tx.gasPrice
}

// Gas returns the gaslimit of the tx.
func (tx *VMTransaction) Gas() uint64 {
	return tx.gas
}

// Nonce returns the nonce of the tx from origin tx assigned by client user.
func (tx *VMTransaction) Nonce() uint64 {
	return tx.nonce
}

// Value returns the transfer value of the tx.
func (tx *VMTransaction) Value() *big.Int {
	return tx.value
}

// Data returns the code of the tx.
func (tx *VMTransaction) Data() []byte {
	return tx.code
}

// Type returns the type of the contract tx.
func (tx *VMTransaction) Type() ContractType {
	return tx.typ
}

// OriginTxHash returns the origin tx hash of the contract tx.
func (tx *VMTransaction) OriginTxHash() *crypto.HashType {
	return tx.originTx
}

// Receipt represents the result of a transaction.
type Receipt struct {
	TxHash          crypto.HashType
	TxIndex         uint32
	ContractAddress AddressHash
	Failed          bool
	GasUsed         uint64
	BlockHash       crypto.HashType
	BlockHeight     uint32
}

var _ conv.Convertible = (*Receipt)(nil)
var _ conv.Serializable = (*Receipt)(nil)

// NewReceipt news a Receipt
func NewReceipt(
	txHash *crypto.HashType, contractAddr *AddressHash, failed bool, gasUsed uint64,
) *Receipt {
	if txHash == nil {
		txHash = new(crypto.HashType)
	}
	if contractAddr == nil {
		contractAddr = new(AddressHash)
	}
	return &Receipt{
		TxHash:          *txHash,
		ContractAddress: *contractAddr,
		Failed:          failed,
		GasUsed:         gasUsed,
	}
}

// WithTxIndex sets TxIndex field
func (rc *Receipt) WithTxIndex(i uint32) *Receipt {
	rc.TxIndex = i
	return rc
}

// WithBlockHash sets BlockHash field
func (rc *Receipt) WithBlockHash(hash *crypto.HashType) *Receipt {
	if hash == nil {
		hash = new(crypto.HashType)
	}
	rc.BlockHash = *hash
	return rc
}

// WithBlockHeight sets BlockHeight field
func (rc *Receipt) WithBlockHeight(h uint32) *Receipt {
	rc.BlockHeight = h
	return rc
}

// ToProtoMessage converts Receipt to proto message.
func (rc *Receipt) ToProtoMessage() (proto.Message, error) {
	return &corepb.Receipt{
		TxIndex: rc.TxIndex,
		Failed:  rc.Failed,
		GasUsed: rc.GasUsed,
	}, nil
}

// FromProtoMessage converts proto message to Receipt.
func (rc *Receipt) FromProtoMessage(message proto.Message) error {
	pbrc, ok := message.(*corepb.Receipt)
	if !ok {
		return core.ErrInvalidReceiptProtoMessage
	}
	if message == nil {
		return core.ErrEmptyProtoMessage
	}
	rc.TxIndex = pbrc.TxIndex
	rc.Failed = pbrc.Failed
	rc.GasUsed = pbrc.GasUsed
	return nil
}

// Marshal method marshal Receipt object to binary
func (rc *Receipt) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(rc)
}

// Unmarshal method unmarshal binary data to Receipt object
func (rc *Receipt) Unmarshal(data []byte) error {
	pbrc := new(corepb.Receipt)
	if err := proto.Unmarshal(data, pbrc); err != nil {
		return err
	}
	return rc.FromProtoMessage(pbrc)
}

// Receipts represents multiple receipts in a block
type Receipts []*Receipt

var _ conv.Convertible = (*Receipts)(nil)
var _ conv.Serializable = (*Receipts)(nil)

// Append appends receipts
func (rcs *Receipts) Append(rc ...*Receipt) *Receipts {
	for _, r := range rc {
		if r == nil {
			continue
		}
		*rcs = append(*rcs, r)
	}
	return rcs
}

// Hash calculates and returns receipts' hash
func (rcs *Receipts) Hash() *crypto.HashType {
	data, err := rcs.Marshal()
	if err != nil {
		return nil
	}
	hash := crypto.DoubleHashH(data)
	return &hash
}

// ToProtoMessage converts Receipt to proto message.
func (rcs *Receipts) ToProtoMessage() (proto.Message, error) {
	pbrcs := new(corepb.Receipts)
	for _, rc := range *rcs {
		pbrc := &corepb.Receipt{
			TxIndex: rc.TxIndex,
			Failed:  rc.Failed,
			GasUsed: rc.GasUsed,
		}
		pbrcs.Receipts = append(pbrcs.Receipts, pbrc)
	}
	return pbrcs, nil
}

// FromProtoMessage converts proto message to Receipt.
func (rcs *Receipts) FromProtoMessage(message proto.Message) error {
	pbrcs, ok := message.(*corepb.Receipts)
	if !ok {
		return core.ErrInvalidReceiptProtoMessage
	}
	if pbrcs == nil {
		return core.ErrEmptyProtoMessage
	}
	for _, pbrc := range pbrcs.Receipts {
		rc := new(Receipt)
		rc.TxIndex = pbrc.TxIndex
		rc.Failed = pbrc.Failed
		rc.GasUsed = pbrc.GasUsed
		*rcs = append(*rcs, rc)
	}
	return nil
}

// Marshal method marshal Receipt object to binary
func (rcs *Receipts) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(rcs)
}

// Unmarshal method unmarshal binary data to Receipt object
func (rcs *Receipts) Unmarshal(data []byte) error {
	pbrcs := new(corepb.Receipts)
	if err := proto.Unmarshal(data, pbrcs); err != nil {
		return err
	}
	return rcs.FromProtoMessage(pbrcs)
}
