// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/BOXFoundation/boxd/core"
	corepb "github.com/BOXFoundation/boxd/core/pb"
	"github.com/BOXFoundation/boxd/crypto"
	conv "github.com/BOXFoundation/boxd/p2p/convert"
	"github.com/BOXFoundation/boxd/util/bloom"
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

	// BloomByteLength represents the number of bytes used in a header log bloom.
	BloomByteLength = 256
	// BloomBitLength represents the number of bits used in a header log bloom.
	BloomBitLength = 8 * BloomByteLength
	// BloomHashNum represents the number of hash functions.
	BloomHashNum = 3
)

// VMTxParams defines BoxTx params parsed from script pubkey
type VMTxParams struct {
	GasLimit uint64
	Nonce    uint64
	Version  int32
	From     *AddressHash
	To       *AddressHash
}

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

// NewVMTransaction new a VMTransaction instance with given parameters
func NewVMTransaction(
	value *big.Int, gas, gasPrice, nonce uint64, hash *crypto.HashType, typ ContractType,
	code []byte,
) *VMTransaction {
	return &VMTransaction{
		version:  VMVersion,
		typ:      typ,
		value:    value,
		originTx: hash,
		gasPrice: new(big.Int).SetUint64(gasPrice),
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

// String returns the content of vm transaction to print
func (tx *VMTransaction) String() string {
	var to *AddressContract
	if tx.to != nil && *tx.to != ZeroAddressHash {
		to, _ = NewContractAddressFromHash(tx.to[:])
	}
	code := tx.code
	if len(code) > 256 {
		code = code[:256]
	}
	return fmt.Sprintf("{version: %d, from: %s, to: %s, originTx: %s, value: %d, "+
		"gasPrice: %d, gas: %d, nonce: %d, code: %s, typ: %s}", tx.version, tx.from,
		to, tx.originTx, tx.value, tx.gasPrice, tx.gas, tx.nonce,
		hex.EncodeToString(code), tx.typ)
}

// Receipt represents the result of a transaction.
type Receipt struct {
	TxHash          crypto.HashType
	TxIndex         uint32
	ContractAddress AddressHash
	Deployed        bool
	Failed          bool
	GasUsed         uint64
	BlockHash       crypto.HashType
	BlockHeight     uint32
	InternalTxs     []*crypto.HashType
	ErrMsg          []byte

	Logs  []*Log
	Bloom bloom.Filter
}

var _ conv.Convertible = (*Receipt)(nil)
var _ conv.Serializable = (*Receipt)(nil)

// NewReceipt news a Receipt
func NewReceipt(
	txHash *crypto.HashType, contractAddr *AddressHash, deployed, failed bool,
	gasUsed uint64, errMsg []byte, logs []*Log,
) *Receipt {
	if txHash == nil {
		txHash = new(crypto.HashType)
	}
	if contractAddr == nil {
		contractAddr = new(AddressHash)
	}
	rc := &Receipt{
		TxHash:          *txHash,
		ContractAddress: *contractAddr,
		Deployed:        deployed,
		Failed:          failed,
		GasUsed:         gasUsed,
		ErrMsg:          errMsg,
		Logs:            logs,
	}
	rc.Bloom = createLogBloom(rc.Logs)
	return rc
}

// WithTxHash sets txHash field
func (rc *Receipt) WithTxHash(hash *crypto.HashType) *Receipt {
	rc.TxHash = *hash
	return rc
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

// ApppendInternalTxs append internal txs to InternalTxs field
func (rc *Receipt) ApppendInternalTxs(hashes ...*crypto.HashType) {
	rc.InternalTxs = append(rc.InternalTxs, hashes...)
}

// CreateReceiptsBloom create a bloom filter matches Receipts.
func CreateReceiptsBloom(rcs Receipts) bloom.Filter {
	bloom := bloom.NewFilterWithMK(BloomBitLength, BloomHashNum)
	for _, rc := range rcs {
		bloom.Merge(createLogBloom(rc.Logs))
	}
	return bloom
}

func createLogBloom(logs []*Log) bloom.Filter {
	bloom := bloom.NewFilterWithMK(BloomBitLength, BloomHashNum)
	for _, log := range logs {
		bloom.Add(log.Address.Bytes())
		for _, topic := range log.Topics {
			bloom.Add(topic.Bytes())
		}
	}
	return bloom
}

// ToProtoMessage converts Receipt to proto message.
func (rc *Receipt) ToProtoMessage() (proto.Message, error) {

	var logs []*corepb.Log
	for _, l := range rc.Logs {
		log, err := l.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		if log, ok := log.(*corepb.Log); ok {
			logs = append(logs, log)
		}
	}
	if rc.Bloom == nil {
		rc.Bloom = CreateReceiptsBloom(nil)
	}
	bloom, err := rc.Bloom.Marshal()
	if err != nil {
		return nil, err
	}
	// internal txs
	internalTxsBytes := make([][]byte, 0, len(rc.InternalTxs))
	for _, v := range rc.InternalTxs {
		internalTxsBytes = append(internalTxsBytes, v[:])
	}

	return &corepb.Receipt{
		TxHash:      rc.TxHash[:],
		TxIndex:     rc.TxIndex,
		Deployed:    rc.Deployed,
		Failed:      rc.Failed,
		GasUsed:     rc.GasUsed,
		InternalTxs: internalTxsBytes,
		ErrMsg:      rc.ErrMsg[:],
		Logs:        logs,
		Bloom:       bloom,
	}, nil
}

// FromProtoMessage converts proto message to Receipt.
func (rc *Receipt) FromProtoMessage(message proto.Message) error {
	if message == nil {
		return core.ErrEmptyProtoMessage
	}
	pbrc, ok := message.(*corepb.Receipt)
	if !ok {
		return core.ErrInvalidReceiptProtoMessage
	}

	var logs []*Log
	for _, l := range pbrc.Logs {
		log := new(Log)
		if err := log.FromProtoMessage(l); err != nil {
			return err
		}
		logs = append(logs, log)
	}
	if rc.Bloom == nil {
		rc.Bloom = CreateReceiptsBloom(nil)
	}
	err := rc.Bloom.Unmarshal(pbrc.Bloom)
	if err != nil {
		return err
	}
	if err := rc.TxHash.SetBytes(pbrc.TxHash); err != nil {
		return err
	}
	rc.TxIndex = pbrc.TxIndex
	rc.Deployed = pbrc.Deployed
	rc.Failed = pbrc.Failed
	rc.GasUsed = pbrc.GasUsed
	rc.ErrMsg = make([]byte, len(pbrc.ErrMsg))
	copy(rc.ErrMsg, pbrc.ErrMsg[:])
	rc.Logs = logs
	// internal txs
	rc.InternalTxs = make([]*crypto.HashType, 0, len(pbrc.InternalTxs))
	for _, v := range pbrc.InternalTxs {
		hash := new(crypto.HashType)
		if err := hash.SetBytes(v); err != nil {
			return err
		}
		rc.InternalTxs = append(rc.InternalTxs, hash)
	}
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

func (rcs *Receipts) toHashReceipts() (*HashReceipts, error) {

	pbrcs := new(HashReceipts)

	for _, rc := range *rcs {
		hashrc := &hashReceipt{
			TxIndex:     rc.TxIndex,
			Deployed:    rc.Deployed,
			Failed:      rc.Failed,
			GasUsed:     rc.GasUsed,
			InternalTxs: rc.InternalTxs,
			BlockHeight: rc.BlockHeight,
			ErrMsg:      rc.ErrMsg[:],
		}
		hashrc.TxHash.SetBytes(rc.TxHash[:])
		hashrc.ContractAddress.SetBytes(rc.ContractAddress.Bytes())

		hashrc.Logs = []*hashLog{}
		for _, log := range rc.Logs {
			hashlog := log.toHashLog()
			hashrc.Logs = append(hashrc.Logs, hashlog)
		}

		if rc.Bloom == nil {
			rc.Bloom = CreateReceiptsBloom(nil)
		}
		err := rc.Bloom.Copy(rc.Bloom)
		if err != nil {
			return nil, err
		}
		*pbrcs = append(*pbrcs, hashrc)
	}
	return pbrcs, nil
}

// Hash calculates and returns receipts' hash
func (rcs *Receipts) Hash() *crypto.HashType {

	hashrcs, err := rcs.toHashReceipts()
	if err != nil {
		logger.Error(err)
		return nil
	}

	data, err := hashrcs.Marshal()
	if err != nil {
		logger.Error(err)
		return nil
	}
	hash := crypto.DoubleHashH(data)
	return &hash
}

// GetTxReceipt returns a tx receipt in receipts
func (rcs *Receipts) GetTxReceipt(hash *crypto.HashType) *Receipt {
	for _, r := range *rcs {
		if r.TxHash == *hash {
			return r
		}
	}
	return nil
}

// ToProtoMessage converts Receipt to proto message.
func (rcs *Receipts) ToProtoMessage() (proto.Message, error) {
	pbrcs := new(corepb.Receipts)
	for _, rc := range *rcs {
		pbrc, err := rc.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		pbrcs.Receipts = append(pbrcs.Receipts, pbrc.(*corepb.Receipt))
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
		if err := rc.FromProtoMessage(pbrc); err != nil {
			return err
		}
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

// hashReceipt is a wrapper around a Receipt that used by Hash()
type hashReceipt struct {
	TxHash          crypto.HashType
	TxIndex         uint32
	ContractAddress AddressHash
	Deployed        bool
	Failed          bool
	GasUsed         uint64
	InternalTxs     []*crypto.HashType
	BlockHash       crypto.HashType
	BlockHeight     uint32
	ErrMsg          []byte

	Logs  []*hashLog
	Bloom bloom.Filter
}

var _ conv.Convertible = (*hashReceipt)(nil)
var _ conv.Serializable = (*hashReceipt)(nil)

// ToProtoMessage converts Receipt to proto message.
func (rc *hashReceipt) ToProtoMessage() (proto.Message, error) {

	var logs []*corepb.HashLog
	for _, l := range rc.Logs {
		log, err := l.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		if log, ok := log.(*corepb.HashLog); ok {
			logs = append(logs, log)
		}
	}
	if rc.Bloom == nil {
		rc.Bloom = CreateReceiptsBloom(nil)
	}
	bloom, err := rc.Bloom.Marshal()
	if err != nil {
		return nil, err
	}
	// internal txs
	internalTxsBytes := make([][]byte, 0, len(rc.InternalTxs))
	for _, v := range rc.InternalTxs {
		internalTxsBytes = append(internalTxsBytes, v[:])
	}

	return &corepb.HashReceipt{
		TxHash:      rc.TxHash[:],
		TxIndex:     rc.TxIndex,
		Deployed:    rc.Deployed,
		Failed:      rc.Failed,
		GasUsed:     rc.GasUsed,
		InternalTxs: internalTxsBytes,
		ErrMsg:      rc.ErrMsg[:],
		Logs:        logs,
		Bloom:       bloom,
	}, nil
}

// FromProtoMessage converts proto message to Receipt.
func (rc *hashReceipt) FromProtoMessage(message proto.Message) error {
	if message == nil {
		return core.ErrEmptyProtoMessage
	}
	pbrc, ok := message.(*corepb.Receipt)
	if !ok {
		return core.ErrInvalidReceiptProtoMessage
	}

	var logs []*hashLog
	for _, l := range pbrc.Logs {
		log := new(hashLog)
		if err := log.FromProtoMessage(l); err != nil {
			return err
		}
		logs = append(logs, log)
	}
	if rc.Bloom == nil {
		rc.Bloom = CreateReceiptsBloom(nil)
	}
	err := rc.Bloom.Unmarshal(pbrc.Bloom)
	if err != nil {
		return err
	}
	if err := rc.TxHash.SetBytes(pbrc.TxHash); err != nil {
		return err
	}
	rc.TxIndex = pbrc.TxIndex
	rc.Deployed = pbrc.Deployed
	rc.Failed = pbrc.Failed
	rc.GasUsed = pbrc.GasUsed
	rc.ErrMsg = make([]byte, len(pbrc.ErrMsg))
	copy(rc.ErrMsg, pbrc.ErrMsg[:])
	rc.Logs = logs
	// internal txs
	rc.InternalTxs = make([]*crypto.HashType, 0, len(pbrc.InternalTxs))
	for _, v := range pbrc.InternalTxs {
		hash := new(crypto.HashType)
		if err := hash.SetBytes(v); err != nil {
			return err
		}
		rc.InternalTxs = append(rc.InternalTxs, hash)
	}
	return nil
}

// Marshal method marshal Receipt object to binary
func (rc *hashReceipt) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(rc)
}

// Unmarshal method unmarshal binary data to Receipt object
func (rc *hashReceipt) Unmarshal(data []byte) error {
	pbrc := new(corepb.HashReceipt)
	if err := proto.Unmarshal(data, pbrc); err != nil {
		return err
	}
	return rc.FromProtoMessage(pbrc)
}

// HashReceipts is a wrapper around a Log that used by Hash()
type HashReceipts []*hashReceipt

var _ conv.Convertible = (*HashReceipts)(nil)
var _ conv.Serializable = (*HashReceipts)(nil)

// ToProtoMessage converts Receipt to proto message.
func (rcs *HashReceipts) ToProtoMessage() (proto.Message, error) {
	pbrcs := new(corepb.HashReceipts)
	for _, rc := range *rcs {
		pbrc, err := rc.ToProtoMessage()
		if err != nil {
			return nil, err
		}
		pbrcs.Receipts = append(pbrcs.Receipts, pbrc.(*corepb.HashReceipt))
	}
	return pbrcs, nil
}

// FromProtoMessage converts proto message to Receipt.
func (rcs *HashReceipts) FromProtoMessage(message proto.Message) error {
	pbrcs, ok := message.(*corepb.HashReceipts)
	if !ok {
		return core.ErrInvalidReceiptProtoMessage
	}
	if pbrcs == nil {
		return core.ErrEmptyProtoMessage
	}
	for _, pbrc := range pbrcs.Receipts {
		rc := new(hashReceipt)
		if err := rc.FromProtoMessage(pbrc); err != nil {
			return err
		}
		*rcs = append(*rcs, rc)
	}
	return nil
}

// Marshal method marshal Receipt object to binary
func (rcs *HashReceipts) Marshal() (data []byte, err error) {
	return conv.MarshalConvertible(rcs)
}

// Unmarshal method unmarshal binary data to Receipt object
func (rcs *HashReceipts) Unmarshal(data []byte) error {
	pbrcs := new(corepb.HashReceipts)
	if err := proto.Unmarshal(data, pbrcs); err != nil {
		return err
	}
	return rcs.FromProtoMessage(pbrcs)
}
