// Code generated by protoc-gen-go. DO NOT EDIT.
// source: p2p/pb/message.proto

package p2ppb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type MessageHeader struct {
	Magic                uint32   `protobuf:"varint,1,opt,name=magic,proto3" json:"magic,omitempty"`
	Code                 uint32   `protobuf:"varint,2,opt,name=code,proto3" json:"code,omitempty"`
	DataLength           uint32   `protobuf:"varint,3,opt,name=data_length,json=dataLength,proto3" json:"data_length,omitempty"`
	DataChecksum         uint32   `protobuf:"varint,4,opt,name=data_checksum,json=dataChecksum,proto3" json:"data_checksum,omitempty"`
	Reserved             []byte   `protobuf:"bytes,5,opt,name=reserved,proto3" json:"reserved,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *MessageHeader) Reset()         { *m = MessageHeader{} }
func (m *MessageHeader) String() string { return proto.CompactTextString(m) }
func (*MessageHeader) ProtoMessage()    {}
func (*MessageHeader) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_4c0c55773f90caaa, []int{0}
}
func (m *MessageHeader) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_MessageHeader.Unmarshal(m, b)
}
func (m *MessageHeader) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_MessageHeader.Marshal(b, m, deterministic)
}
func (dst *MessageHeader) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MessageHeader.Merge(dst, src)
}
func (m *MessageHeader) XXX_Size() int {
	return xxx_messageInfo_MessageHeader.Size(m)
}
func (m *MessageHeader) XXX_DiscardUnknown() {
	xxx_messageInfo_MessageHeader.DiscardUnknown(m)
}

var xxx_messageInfo_MessageHeader proto.InternalMessageInfo

func (m *MessageHeader) GetMagic() uint32 {
	if m != nil {
		return m.Magic
	}
	return 0
}

func (m *MessageHeader) GetCode() uint32 {
	if m != nil {
		return m.Code
	}
	return 0
}

func (m *MessageHeader) GetDataLength() uint32 {
	if m != nil {
		return m.DataLength
	}
	return 0
}

func (m *MessageHeader) GetDataChecksum() uint32 {
	if m != nil {
		return m.DataChecksum
	}
	return 0
}

func (m *MessageHeader) GetReserved() []byte {
	if m != nil {
		return m.Reserved
	}
	return nil
}

type Peers struct {
	Peers                []*PeerInfo `protobuf:"bytes,1,rep,name=peers,proto3" json:"peers,omitempty"`
	IsSyncing            bool        `protobuf:"varint,2,opt,name=isSyncing,proto3" json:"isSyncing,omitempty"`
	XXX_NoUnkeyedLiteral struct{}    `json:"-"`
	XXX_unrecognized     []byte      `json:"-"`
	XXX_sizecache        int32       `json:"-"`
}

func (m *Peers) Reset()         { *m = Peers{} }
func (m *Peers) String() string { return proto.CompactTextString(m) }
func (*Peers) ProtoMessage()    {}
func (*Peers) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_4c0c55773f90caaa, []int{1}
}
func (m *Peers) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Peers.Unmarshal(m, b)
}
func (m *Peers) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Peers.Marshal(b, m, deterministic)
}
func (dst *Peers) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Peers.Merge(dst, src)
}
func (m *Peers) XXX_Size() int {
	return xxx_messageInfo_Peers.Size(m)
}
func (m *Peers) XXX_DiscardUnknown() {
	xxx_messageInfo_Peers.DiscardUnknown(m)
}

var xxx_messageInfo_Peers proto.InternalMessageInfo

func (m *Peers) GetPeers() []*PeerInfo {
	if m != nil {
		return m.Peers
	}
	return nil
}

func (m *Peers) GetIsSyncing() bool {
	if m != nil {
		return m.IsSyncing
	}
	return false
}

type PeerInfo struct {
	Id                   string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Addrs                []string `protobuf:"bytes,2,rep,name=addrs,proto3" json:"addrs,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PeerInfo) Reset()         { *m = PeerInfo{} }
func (m *PeerInfo) String() string { return proto.CompactTextString(m) }
func (*PeerInfo) ProtoMessage()    {}
func (*PeerInfo) Descriptor() ([]byte, []int) {
	return fileDescriptor_message_4c0c55773f90caaa, []int{2}
}
func (m *PeerInfo) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PeerInfo.Unmarshal(m, b)
}
func (m *PeerInfo) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PeerInfo.Marshal(b, m, deterministic)
}
func (dst *PeerInfo) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PeerInfo.Merge(dst, src)
}
func (m *PeerInfo) XXX_Size() int {
	return xxx_messageInfo_PeerInfo.Size(m)
}
func (m *PeerInfo) XXX_DiscardUnknown() {
	xxx_messageInfo_PeerInfo.DiscardUnknown(m)
}

var xxx_messageInfo_PeerInfo proto.InternalMessageInfo

func (m *PeerInfo) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *PeerInfo) GetAddrs() []string {
	if m != nil {
		return m.Addrs
	}
	return nil
}

func init() {
	proto.RegisterType((*MessageHeader)(nil), "p2ppb.MessageHeader")
	proto.RegisterType((*Peers)(nil), "p2ppb.Peers")
	proto.RegisterType((*PeerInfo)(nil), "p2ppb.PeerInfo")
}

func init() { proto.RegisterFile("p2p/pb/message.proto", fileDescriptor_message_4c0c55773f90caaa) }

var fileDescriptor_message_4c0c55773f90caaa = []byte{
	// 251 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x44, 0x90, 0xd1, 0x4a, 0x84, 0x40,
	0x14, 0x86, 0x51, 0xd7, 0xd0, 0xb3, 0x6b, 0xc1, 0x61, 0x2f, 0x86, 0x08, 0x12, 0x23, 0xf0, 0xca,
	0x0d, 0x7b, 0x84, 0x6e, 0x0a, 0x36, 0x88, 0xe9, 0x01, 0x62, 0x74, 0x4e, 0xae, 0x94, 0x3a, 0xcc,
	0x58, 0xd0, 0xb3, 0xf4, 0xb2, 0x31, 0xc7, 0x6a, 0xef, 0xce, 0xff, 0x7d, 0x67, 0xe0, 0xfc, 0x03,
	0x5b, 0x53, 0x9b, 0x9d, 0x69, 0x76, 0x03, 0x39, 0xa7, 0x3a, 0xaa, 0x8c, 0x9d, 0xe6, 0x09, 0x63,
	0x53, 0x1b, 0xd3, 0x14, 0xdf, 0x01, 0x64, 0x8f, 0x8b, 0xb8, 0x27, 0xa5, 0xc9, 0xe2, 0x16, 0xe2,
	0x41, 0x75, 0x7d, 0x2b, 0x82, 0x3c, 0x28, 0x33, 0xb9, 0x04, 0x44, 0x58, 0xb5, 0x93, 0x26, 0x11,
	0x32, 0xe4, 0x19, 0x2f, 0x61, 0xad, 0xd5, 0xac, 0x5e, 0xde, 0x69, 0xec, 0xe6, 0x83, 0x88, 0x58,
	0x81, 0x47, 0x7b, 0x26, 0x78, 0x05, 0x19, 0x2f, 0xb4, 0x07, 0x6a, 0xdf, 0xdc, 0xc7, 0x20, 0x56,
	0xbc, 0xb2, 0xf1, 0xf0, 0xee, 0x97, 0xe1, 0x39, 0x24, 0x96, 0x1c, 0xd9, 0x4f, 0xd2, 0x22, 0xce,
	0x83, 0x72, 0x23, 0xff, 0x73, 0xb1, 0x87, 0xf8, 0x89, 0xc8, 0x3a, 0xbc, 0x86, 0xd8, 0xf8, 0x41,
	0x04, 0x79, 0x54, 0xae, 0xeb, 0xb3, 0x8a, 0xaf, 0xaf, 0xbc, 0x7c, 0x18, 0x5f, 0x27, 0xb9, 0x58,
	0xbc, 0x80, 0xb4, 0x77, 0xcf, 0x5f, 0x63, 0xdb, 0x8f, 0x1d, 0x9f, 0x9a, 0xc8, 0x23, 0x28, 0x6e,
	0x20, 0xf9, 0x7b, 0x80, 0xa7, 0x10, 0xf6, 0x9a, 0x2b, 0xa6, 0x32, 0xec, 0xb5, 0x6f, 0xad, 0xb4,
	0xb6, 0x4e, 0x84, 0x79, 0x54, 0xa6, 0x72, 0x09, 0xcd, 0x09, 0xff, 0xd5, 0xed, 0x4f, 0x00, 0x00,
	0x00, 0xff, 0xff, 0xc2, 0x60, 0x8e, 0xe3, 0x43, 0x01, 0x00, 0x00,
}
