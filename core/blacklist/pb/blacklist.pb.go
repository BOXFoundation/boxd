// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: blacklist.proto

package pb

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"
import pb "github.com/BOXFoundation/boxd/core/pb"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type Evidence struct {
	PubKeyChecksum uint32          `protobuf:"varint,1,opt,name=PubKeyChecksum,proto3" json:"PubKeyChecksum,omitempty"`
	Tx             *pb.Transaction `protobuf:"bytes,2,opt,name=Tx,proto3" json:"Tx,omitempty"`
	Block          *pb.Block       `protobuf:"bytes,3,opt,name=Block,proto3" json:"Block,omitempty"`
	Type           uint32          `protobuf:"varint,4,opt,name=Type,proto3" json:"Type,omitempty"`
	Err            string          `protobuf:"bytes,5,opt,name=Err,proto3" json:"Err,omitempty"`
	Ts             int64           `protobuf:"varint,6,opt,name=Ts,proto3" json:"Ts,omitempty"`
}

func (m *Evidence) Reset()         { *m = Evidence{} }
func (m *Evidence) String() string { return proto.CompactTextString(m) }
func (*Evidence) ProtoMessage()    {}
func (*Evidence) Descriptor() ([]byte, []int) {
	return fileDescriptor_blacklist_40b87b117eade5e1, []int{0}
}
func (m *Evidence) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Evidence) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Evidence.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (dst *Evidence) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Evidence.Merge(dst, src)
}
func (m *Evidence) XXX_Size() int {
	return m.Size()
}
func (m *Evidence) XXX_DiscardUnknown() {
	xxx_messageInfo_Evidence.DiscardUnknown(m)
}

var xxx_messageInfo_Evidence proto.InternalMessageInfo

func (m *Evidence) GetPubKeyChecksum() uint32 {
	if m != nil {
		return m.PubKeyChecksum
	}
	return 0
}

func (m *Evidence) GetTx() *pb.Transaction {
	if m != nil {
		return m.Tx
	}
	return nil
}

func (m *Evidence) GetBlock() *pb.Block {
	if m != nil {
		return m.Block
	}
	return nil
}

func (m *Evidence) GetType() uint32 {
	if m != nil {
		return m.Type
	}
	return 0
}

func (m *Evidence) GetErr() string {
	if m != nil {
		return m.Err
	}
	return ""
}

func (m *Evidence) GetTs() int64 {
	if m != nil {
		return m.Ts
	}
	return 0
}

type BlacklistMsg struct {
	Hash      []byte      `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	Evidences []*Evidence `protobuf:"bytes,2,rep,name=evidences,proto3" json:"evidences,omitempty"`
}

func (m *BlacklistMsg) Reset()         { *m = BlacklistMsg{} }
func (m *BlacklistMsg) String() string { return proto.CompactTextString(m) }
func (*BlacklistMsg) ProtoMessage()    {}
func (*BlacklistMsg) Descriptor() ([]byte, []int) {
	return fileDescriptor_blacklist_40b87b117eade5e1, []int{1}
}
func (m *BlacklistMsg) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *BlacklistMsg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_BlacklistMsg.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (dst *BlacklistMsg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlacklistMsg.Merge(dst, src)
}
func (m *BlacklistMsg) XXX_Size() int {
	return m.Size()
}
func (m *BlacklistMsg) XXX_DiscardUnknown() {
	xxx_messageInfo_BlacklistMsg.DiscardUnknown(m)
}

var xxx_messageInfo_BlacklistMsg proto.InternalMessageInfo

func (m *BlacklistMsg) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

func (m *BlacklistMsg) GetEvidences() []*Evidence {
	if m != nil {
		return m.Evidences
	}
	return nil
}

type BlacklistConfirmMsg struct {
	PubKeyChecksum uint32 `protobuf:"varint,1,opt,name=pubKeyChecksum,proto3" json:"pubKeyChecksum,omitempty"`
	Hash           []byte `protobuf:"bytes,2,opt,name=hash,proto3" json:"hash,omitempty"`
	Signature      []byte `protobuf:"bytes,3,opt,name=signature,proto3" json:"signature,omitempty"`
	Timestamp      int64  `protobuf:"varint,4,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
}

func (m *BlacklistConfirmMsg) Reset()         { *m = BlacklistConfirmMsg{} }
func (m *BlacklistConfirmMsg) String() string { return proto.CompactTextString(m) }
func (*BlacklistConfirmMsg) ProtoMessage()    {}
func (*BlacklistConfirmMsg) Descriptor() ([]byte, []int) {
	return fileDescriptor_blacklist_40b87b117eade5e1, []int{2}
}
func (m *BlacklistConfirmMsg) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *BlacklistConfirmMsg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_BlacklistConfirmMsg.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (dst *BlacklistConfirmMsg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlacklistConfirmMsg.Merge(dst, src)
}
func (m *BlacklistConfirmMsg) XXX_Size() int {
	return m.Size()
}
func (m *BlacklistConfirmMsg) XXX_DiscardUnknown() {
	xxx_messageInfo_BlacklistConfirmMsg.DiscardUnknown(m)
}

var xxx_messageInfo_BlacklistConfirmMsg proto.InternalMessageInfo

func (m *BlacklistConfirmMsg) GetPubKeyChecksum() uint32 {
	if m != nil {
		return m.PubKeyChecksum
	}
	return 0
}

func (m *BlacklistConfirmMsg) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

func (m *BlacklistConfirmMsg) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

func (m *BlacklistConfirmMsg) GetTimestamp() int64 {
	if m != nil {
		return m.Timestamp
	}
	return 0
}

func init() {
	proto.RegisterType((*Evidence)(nil), "pb.Evidence")
	proto.RegisterType((*BlacklistMsg)(nil), "pb.BlacklistMsg")
	proto.RegisterType((*BlacklistConfirmMsg)(nil), "pb.BlacklistConfirmMsg")
}
func (m *Evidence) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Evidence) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.PubKeyChecksum != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintBlacklist(dAtA, i, uint64(m.PubKeyChecksum))
	}
	if m.Tx != nil {
		dAtA[i] = 0x12
		i++
		i = encodeVarintBlacklist(dAtA, i, uint64(m.Tx.Size()))
		n1, err := m.Tx.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	if m.Block != nil {
		dAtA[i] = 0x1a
		i++
		i = encodeVarintBlacklist(dAtA, i, uint64(m.Block.Size()))
		n2, err := m.Block.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n2
	}
	if m.Type != 0 {
		dAtA[i] = 0x20
		i++
		i = encodeVarintBlacklist(dAtA, i, uint64(m.Type))
	}
	if len(m.Err) > 0 {
		dAtA[i] = 0x2a
		i++
		i = encodeVarintBlacklist(dAtA, i, uint64(len(m.Err)))
		i += copy(dAtA[i:], m.Err)
	}
	if m.Ts != 0 {
		dAtA[i] = 0x30
		i++
		i = encodeVarintBlacklist(dAtA, i, uint64(m.Ts))
	}
	return i, nil
}

func (m *BlacklistMsg) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *BlacklistMsg) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Hash) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintBlacklist(dAtA, i, uint64(len(m.Hash)))
		i += copy(dAtA[i:], m.Hash)
	}
	if len(m.Evidences) > 0 {
		for _, msg := range m.Evidences {
			dAtA[i] = 0x12
			i++
			i = encodeVarintBlacklist(dAtA, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(dAtA[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	return i, nil
}

func (m *BlacklistConfirmMsg) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *BlacklistConfirmMsg) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.PubKeyChecksum != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintBlacklist(dAtA, i, uint64(m.PubKeyChecksum))
	}
	if len(m.Hash) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintBlacklist(dAtA, i, uint64(len(m.Hash)))
		i += copy(dAtA[i:], m.Hash)
	}
	if len(m.Signature) > 0 {
		dAtA[i] = 0x1a
		i++
		i = encodeVarintBlacklist(dAtA, i, uint64(len(m.Signature)))
		i += copy(dAtA[i:], m.Signature)
	}
	if m.Timestamp != 0 {
		dAtA[i] = 0x20
		i++
		i = encodeVarintBlacklist(dAtA, i, uint64(m.Timestamp))
	}
	return i, nil
}

func encodeVarintBlacklist(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *Evidence) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.PubKeyChecksum != 0 {
		n += 1 + sovBlacklist(uint64(m.PubKeyChecksum))
	}
	if m.Tx != nil {
		l = m.Tx.Size()
		n += 1 + l + sovBlacklist(uint64(l))
	}
	if m.Block != nil {
		l = m.Block.Size()
		n += 1 + l + sovBlacklist(uint64(l))
	}
	if m.Type != 0 {
		n += 1 + sovBlacklist(uint64(m.Type))
	}
	l = len(m.Err)
	if l > 0 {
		n += 1 + l + sovBlacklist(uint64(l))
	}
	if m.Ts != 0 {
		n += 1 + sovBlacklist(uint64(m.Ts))
	}
	return n
}

func (m *BlacklistMsg) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Hash)
	if l > 0 {
		n += 1 + l + sovBlacklist(uint64(l))
	}
	if len(m.Evidences) > 0 {
		for _, e := range m.Evidences {
			l = e.Size()
			n += 1 + l + sovBlacklist(uint64(l))
		}
	}
	return n
}

func (m *BlacklistConfirmMsg) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.PubKeyChecksum != 0 {
		n += 1 + sovBlacklist(uint64(m.PubKeyChecksum))
	}
	l = len(m.Hash)
	if l > 0 {
		n += 1 + l + sovBlacklist(uint64(l))
	}
	l = len(m.Signature)
	if l > 0 {
		n += 1 + l + sovBlacklist(uint64(l))
	}
	if m.Timestamp != 0 {
		n += 1 + sovBlacklist(uint64(m.Timestamp))
	}
	return n
}

func sovBlacklist(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozBlacklist(x uint64) (n int) {
	return sovBlacklist(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *Evidence) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowBlacklist
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Evidence: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Evidence: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field PubKeyChecksum", wireType)
			}
			m.PubKeyChecksum = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.PubKeyChecksum |= (uint32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Tx", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthBlacklist
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Tx == nil {
				m.Tx = &pb.Transaction{}
			}
			if err := m.Tx.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Block", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthBlacklist
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Block == nil {
				m.Block = &pb.Block{}
			}
			if err := m.Block.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Type", wireType)
			}
			m.Type = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Type |= (uint32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Err", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthBlacklist
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Err = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 6:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Ts", wireType)
			}
			m.Ts = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Ts |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipBlacklist(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthBlacklist
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *BlacklistMsg) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowBlacklist
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: BlacklistMsg: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: BlacklistMsg: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Hash", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthBlacklist
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Hash = append(m.Hash[:0], dAtA[iNdEx:postIndex]...)
			if m.Hash == nil {
				m.Hash = []byte{}
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Evidences", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthBlacklist
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Evidences = append(m.Evidences, &Evidence{})
			if err := m.Evidences[len(m.Evidences)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipBlacklist(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthBlacklist
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *BlacklistConfirmMsg) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowBlacklist
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: BlacklistConfirmMsg: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: BlacklistConfirmMsg: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field PubKeyChecksum", wireType)
			}
			m.PubKeyChecksum = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.PubKeyChecksum |= (uint32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Hash", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthBlacklist
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Hash = append(m.Hash[:0], dAtA[iNdEx:postIndex]...)
			if m.Hash == nil {
				m.Hash = []byte{}
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Signature", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthBlacklist
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Signature = append(m.Signature[:0], dAtA[iNdEx:postIndex]...)
			if m.Signature == nil {
				m.Signature = []byte{}
			}
			iNdEx = postIndex
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Timestamp", wireType)
			}
			m.Timestamp = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Timestamp |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipBlacklist(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthBlacklist
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipBlacklist(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowBlacklist
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowBlacklist
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthBlacklist
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowBlacklist
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipBlacklist(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthBlacklist = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowBlacklist   = fmt.Errorf("proto: integer overflow")
)

func init() { proto.RegisterFile("blacklist.proto", fileDescriptor_blacklist_40b87b117eade5e1) }

var fileDescriptor_blacklist_40b87b117eade5e1 = []byte{
	// 361 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x51, 0xcf, 0x6a, 0xfa, 0x40,
	0x18, 0x74, 0x37, 0x2a, 0x3f, 0xd7, 0xe8, 0xaf, 0xac, 0x97, 0xa5, 0x94, 0x10, 0x14, 0x24, 0xf4,
	0x90, 0x50, 0xfb, 0x06, 0x11, 0x7b, 0x29, 0xfd, 0xc3, 0x92, 0x43, 0xaf, 0xd9, 0xb8, 0x35, 0x41,
	0x93, 0x5d, 0xb2, 0x49, 0xd1, 0x87, 0x28, 0xf4, 0x59, 0xfa, 0x14, 0x3d, 0x7a, 0xec, 0xb1, 0xe8,
	0x8b, 0x94, 0xac, 0x55, 0xdb, 0xd2, 0xdb, 0x30, 0xf3, 0xed, 0xcc, 0xce, 0xf7, 0xa1, 0xff, 0x6c,
	0x11, 0x46, 0xf3, 0x45, 0xa2, 0x0a, 0x57, 0xe6, 0xa2, 0x10, 0x18, 0x4a, 0x76, 0x7a, 0x31, 0x4b,
	0x8a, 0xb8, 0x64, 0x6e, 0x24, 0x52, 0xcf, 0xbf, 0x7b, 0xb8, 0x12, 0x65, 0x36, 0x0d, 0x8b, 0x44,
	0x64, 0x1e, 0x13, 0xcb, 0xa9, 0x17, 0x89, 0x9c, 0x7b, 0x92, 0x79, 0x6c, 0x21, 0xa2, 0xf9, 0xee,
	0x59, 0xff, 0x15, 0xa0, 0x7f, 0x93, 0xa7, 0x64, 0xca, 0xb3, 0x88, 0xe3, 0x21, 0xea, 0xde, 0x97,
	0xec, 0x9a, 0xaf, 0xc6, 0x31, 0x8f, 0xe6, 0xaa, 0x4c, 0x09, 0xb0, 0x81, 0xd3, 0xa1, 0xbf, 0x58,
	0x3c, 0x40, 0x30, 0x58, 0x12, 0x68, 0x03, 0xa7, 0x3d, 0xea, 0xb9, 0x95, 0xad, 0x64, 0x6e, 0x90,
	0x87, 0x99, 0x0a, 0xa3, 0x2a, 0x8e, 0xc2, 0x60, 0x89, 0x07, 0xa8, 0xe1, 0x57, 0x41, 0xc4, 0xd0,
	0x73, 0x9d, 0xfd, 0x9c, 0x26, 0xe9, 0x4e, 0xc3, 0x18, 0xd5, 0x83, 0x95, 0xe4, 0xa4, 0xae, 0x73,
	0x34, 0xc6, 0x27, 0xc8, 0x98, 0xe4, 0x39, 0x69, 0xd8, 0xc0, 0x69, 0xd1, 0x0a, 0xe2, 0x2e, 0x82,
	0x81, 0x22, 0x4d, 0x1b, 0x38, 0x06, 0x85, 0x81, 0xea, 0xdf, 0x22, 0xd3, 0xdf, 0xd7, 0xbf, 0x51,
	0xb3, 0xca, 0x25, 0x0e, 0x55, 0xac, 0x7f, 0x6b, 0x52, 0x8d, 0xf1, 0x39, 0x6a, 0xf1, 0xaf, 0x5e,
	0x8a, 0x40, 0xdb, 0x70, 0xda, 0x23, 0xd3, 0x95, 0xcc, 0xdd, 0x97, 0xa5, 0x47, 0xb9, 0xff, 0x0c,
	0x50, 0xef, 0x60, 0x38, 0x16, 0xd9, 0x63, 0x92, 0xa7, 0x95, 0xef, 0x10, 0x75, 0xe5, 0x9f, 0xfb,
	0xf8, 0xc9, 0x1e, 0xf2, 0xe1, 0xb7, 0xfc, 0x33, 0xd4, 0x52, 0xc9, 0x2c, 0x0b, 0x8b, 0x32, 0xe7,
	0x7a, 0x05, 0x26, 0x3d, 0x12, 0x95, 0x5a, 0x24, 0x29, 0x57, 0x45, 0x98, 0x4a, 0x5d, 0xde, 0xa0,
	0x47, 0xc2, 0x27, 0x6f, 0x1b, 0x0b, 0xac, 0x37, 0x16, 0xf8, 0xd8, 0x58, 0xe0, 0x65, 0x6b, 0xd5,
	0xd6, 0x5b, 0xab, 0xf6, 0xbe, 0xb5, 0x6a, 0xac, 0xa9, 0xaf, 0x76, 0xf9, 0x19, 0x00, 0x00, 0xff,
	0xff, 0xbe, 0x6d, 0xf6, 0x78, 0xff, 0x01, 0x00, 0x00,
}
