// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: db.proto

package rpcpb

import (
	context "context"
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	grpc "google.golang.org/grpc"
	io "io"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type GetDatabaseKeysRequest struct {
	Table  string `protobuf:"bytes,1,opt,name=table,proto3" json:"table,omitempty"`
	Prefix string `protobuf:"bytes,2,opt,name=prefix,proto3" json:"prefix,omitempty"`
	Skip   int32  `protobuf:"varint,3,opt,name=skip,proto3" json:"skip,omitempty"`
	Limit  int32  `protobuf:"varint,4,opt,name=limit,proto3" json:"limit,omitempty"`
}

func (m *GetDatabaseKeysRequest) Reset()         { *m = GetDatabaseKeysRequest{} }
func (m *GetDatabaseKeysRequest) String() string { return proto.CompactTextString(m) }
func (*GetDatabaseKeysRequest) ProtoMessage()    {}
func (*GetDatabaseKeysRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_8817812184a13374, []int{0}
}
func (m *GetDatabaseKeysRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GetDatabaseKeysRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GetDatabaseKeysRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GetDatabaseKeysRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetDatabaseKeysRequest.Merge(m, src)
}
func (m *GetDatabaseKeysRequest) XXX_Size() int {
	return m.Size()
}
func (m *GetDatabaseKeysRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GetDatabaseKeysRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GetDatabaseKeysRequest proto.InternalMessageInfo

func (m *GetDatabaseKeysRequest) GetTable() string {
	if m != nil {
		return m.Table
	}
	return ""
}

func (m *GetDatabaseKeysRequest) GetPrefix() string {
	if m != nil {
		return m.Prefix
	}
	return ""
}

func (m *GetDatabaseKeysRequest) GetSkip() int32 {
	if m != nil {
		return m.Skip
	}
	return 0
}

func (m *GetDatabaseKeysRequest) GetLimit() int32 {
	if m != nil {
		return m.Limit
	}
	return 0
}

type GetDatabaseKeysResponse struct {
	Code    int32    `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	Message string   `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	Skip    int32    `protobuf:"varint,3,opt,name=skip,proto3" json:"skip,omitempty"`
	Keys    []string `protobuf:"bytes,4,rep,name=keys,proto3" json:"keys,omitempty"`
}

func (m *GetDatabaseKeysResponse) Reset()         { *m = GetDatabaseKeysResponse{} }
func (m *GetDatabaseKeysResponse) String() string { return proto.CompactTextString(m) }
func (*GetDatabaseKeysResponse) ProtoMessage()    {}
func (*GetDatabaseKeysResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_8817812184a13374, []int{1}
}
func (m *GetDatabaseKeysResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GetDatabaseKeysResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GetDatabaseKeysResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GetDatabaseKeysResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetDatabaseKeysResponse.Merge(m, src)
}
func (m *GetDatabaseKeysResponse) XXX_Size() int {
	return m.Size()
}
func (m *GetDatabaseKeysResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GetDatabaseKeysResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GetDatabaseKeysResponse proto.InternalMessageInfo

func (m *GetDatabaseKeysResponse) GetCode() int32 {
	if m != nil {
		return m.Code
	}
	return 0
}

func (m *GetDatabaseKeysResponse) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

func (m *GetDatabaseKeysResponse) GetSkip() int32 {
	if m != nil {
		return m.Skip
	}
	return 0
}

func (m *GetDatabaseKeysResponse) GetKeys() []string {
	if m != nil {
		return m.Keys
	}
	return nil
}

type GetDatabaseValueRequest struct {
	Table string `protobuf:"bytes,1,opt,name=table,proto3" json:"table,omitempty"`
	Key   string `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
}

func (m *GetDatabaseValueRequest) Reset()         { *m = GetDatabaseValueRequest{} }
func (m *GetDatabaseValueRequest) String() string { return proto.CompactTextString(m) }
func (*GetDatabaseValueRequest) ProtoMessage()    {}
func (*GetDatabaseValueRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_8817812184a13374, []int{2}
}
func (m *GetDatabaseValueRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GetDatabaseValueRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GetDatabaseValueRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GetDatabaseValueRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetDatabaseValueRequest.Merge(m, src)
}
func (m *GetDatabaseValueRequest) XXX_Size() int {
	return m.Size()
}
func (m *GetDatabaseValueRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GetDatabaseValueRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GetDatabaseValueRequest proto.InternalMessageInfo

func (m *GetDatabaseValueRequest) GetTable() string {
	if m != nil {
		return m.Table
	}
	return ""
}

func (m *GetDatabaseValueRequest) GetKey() string {
	if m != nil {
		return m.Key
	}
	return ""
}

type GetDatabaseValueResponse struct {
	Code    int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	Value   []byte `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
}

func (m *GetDatabaseValueResponse) Reset()         { *m = GetDatabaseValueResponse{} }
func (m *GetDatabaseValueResponse) String() string { return proto.CompactTextString(m) }
func (*GetDatabaseValueResponse) ProtoMessage()    {}
func (*GetDatabaseValueResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_8817812184a13374, []int{3}
}
func (m *GetDatabaseValueResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GetDatabaseValueResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GetDatabaseValueResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GetDatabaseValueResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetDatabaseValueResponse.Merge(m, src)
}
func (m *GetDatabaseValueResponse) XXX_Size() int {
	return m.Size()
}
func (m *GetDatabaseValueResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GetDatabaseValueResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GetDatabaseValueResponse proto.InternalMessageInfo

func (m *GetDatabaseValueResponse) GetCode() int32 {
	if m != nil {
		return m.Code
	}
	return 0
}

func (m *GetDatabaseValueResponse) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

func (m *GetDatabaseValueResponse) GetValue() []byte {
	if m != nil {
		return m.Value
	}
	return nil
}

func init() {
	proto.RegisterType((*GetDatabaseKeysRequest)(nil), "rpcpb.GetDatabaseKeysRequest")
	proto.RegisterType((*GetDatabaseKeysResponse)(nil), "rpcpb.GetDatabaseKeysResponse")
	proto.RegisterType((*GetDatabaseValueRequest)(nil), "rpcpb.GetDatabaseValueRequest")
	proto.RegisterType((*GetDatabaseValueResponse)(nil), "rpcpb.GetDatabaseValueResponse")
}

func init() { proto.RegisterFile("db.proto", fileDescriptor_8817812184a13374) }

var fileDescriptor_8817812184a13374 = []byte{
	// 371 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x92, 0xbb, 0x4e, 0xf3, 0x30,
	0x1c, 0xc5, 0xeb, 0x36, 0xe9, 0xf7, 0xd5, 0x20, 0xb5, 0xb2, 0x4a, 0xb1, 0x2a, 0x08, 0x55, 0xa6,
	0xaa, 0x43, 0x23, 0x60, 0xeb, 0xc6, 0x45, 0x62, 0x60, 0xcb, 0xc0, 0x88, 0xe4, 0x34, 0x7f, 0x42,
	0x68, 0x12, 0x9b, 0xda, 0xad, 0xe8, 0xca, 0x13, 0x20, 0xf1, 0x52, 0x8c, 0x95, 0x58, 0x18, 0x51,
	0x8b, 0xc4, 0x6b, 0xa0, 0x38, 0xa9, 0x04, 0xbd, 0x30, 0xb0, 0x9d, 0x63, 0x27, 0xe7, 0xf7, 0xbf,
	0x18, 0xff, 0xf7, 0xbd, 0xae, 0x18, 0x72, 0xc5, 0x89, 0x39, 0x14, 0x7d, 0xe1, 0x35, 0xf7, 0x02,
	0xce, 0x83, 0x08, 0x1c, 0x26, 0x42, 0x87, 0x25, 0x09, 0x57, 0x4c, 0x85, 0x3c, 0x91, 0xd9, 0x47,
	0xb6, 0xc0, 0x8d, 0x0b, 0x50, 0xe7, 0x4c, 0x31, 0x8f, 0x49, 0xb8, 0x84, 0x89, 0x74, 0xe1, 0x7e,
	0x04, 0x52, 0x91, 0x3a, 0x36, 0x15, 0xf3, 0x22, 0xa0, 0xa8, 0x85, 0xda, 0x15, 0x37, 0x33, 0xa4,
	0x81, 0xcb, 0x62, 0x08, 0x37, 0xe1, 0x03, 0x2d, 0xea, 0xe3, 0xdc, 0x11, 0x82, 0x0d, 0x39, 0x08,
	0x05, 0x2d, 0xb5, 0x50, 0xdb, 0x74, 0xb5, 0x4e, 0x13, 0xa2, 0x30, 0x0e, 0x15, 0x35, 0xf4, 0x61,
	0x66, 0x6c, 0x8e, 0x77, 0x57, 0x88, 0x52, 0xf0, 0x44, 0x42, 0x1a, 0xd2, 0xe7, 0x7e, 0x46, 0x34,
	0x5d, 0xad, 0x09, 0xc5, 0xff, 0x62, 0x90, 0x92, 0x05, 0x90, 0x13, 0x17, 0x76, 0x2d, 0x92, 0x60,
	0x63, 0x00, 0x13, 0x49, 0x8d, 0x56, 0xa9, 0x5d, 0x71, 0xb5, 0xb6, 0x4f, 0x7e, 0x00, 0xaf, 0x58,
	0x34, 0x82, 0xdf, 0x7b, 0xac, 0xe1, 0xd2, 0x00, 0x26, 0x39, 0x2e, 0x95, 0xf6, 0x35, 0xa6, 0xab,
	0x11, 0x7f, 0x2a, 0xba, 0x8e, 0xcd, 0x71, 0xfa, 0xbb, 0xae, 0x7a, 0xdb, 0xcd, 0xcc, 0xd1, 0x27,
	0xc2, 0xd5, 0x45, 0xfa, 0x19, 0x8f, 0x63, 0x96, 0xf8, 0xe4, 0x16, 0x57, 0x97, 0xe6, 0x44, 0xf6,
	0xbb, 0x7a, 0xa5, 0xdd, 0xf5, 0x1b, 0x6b, 0x5a, 0x9b, 0xae, 0xb3, 0x4a, 0xed, 0xc6, 0xe3, 0xeb,
	0xc7, 0x73, 0xb1, 0x66, 0x6f, 0x39, 0xe3, 0x43, 0xc7, 0xf7, 0x9c, 0x74, 0x3a, 0x3d, 0xd4, 0x21,
	0x77, 0xb8, 0xb6, 0xdc, 0x1d, 0x59, 0x93, 0xf5, 0x7d, 0x72, 0xcd, 0x83, 0x8d, 0xf7, 0x39, 0x6c,
	0x47, 0xc3, 0xaa, 0x36, 0xce, 0x61, 0x01, 0xa8, 0x1e, 0xea, 0x9c, 0xd2, 0x97, 0x99, 0x85, 0xa6,
	0x33, 0x0b, 0xbd, 0xcf, 0x2c, 0xf4, 0x34, 0xb7, 0x0a, 0xd3, 0xb9, 0x55, 0x78, 0x9b, 0x5b, 0x05,
	0xaf, 0xac, 0x1f, 0xe4, 0xf1, 0x57, 0x00, 0x00, 0x00, 0xff, 0xff, 0x59, 0x55, 0xaa, 0x97, 0xc1,
	0x02, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// DatabaseCommandClient is the client API for DatabaseCommand service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type DatabaseCommandClient interface {
	// get all keys of database
	GetDatabaseKeys(ctx context.Context, in *GetDatabaseKeysRequest, opts ...grpc.CallOption) (*GetDatabaseKeysResponse, error)
	// get value of associate with passed key in database
	GetDatabaseValue(ctx context.Context, in *GetDatabaseValueRequest, opts ...grpc.CallOption) (*GetDatabaseValueResponse, error)
}

type databaseCommandClient struct {
	cc *grpc.ClientConn
}

func NewDatabaseCommandClient(cc *grpc.ClientConn) DatabaseCommandClient {
	return &databaseCommandClient{cc}
}

func (c *databaseCommandClient) GetDatabaseKeys(ctx context.Context, in *GetDatabaseKeysRequest, opts ...grpc.CallOption) (*GetDatabaseKeysResponse, error) {
	out := new(GetDatabaseKeysResponse)
	err := c.cc.Invoke(ctx, "/rpcpb.DatabaseCommand/GetDatabaseKeys", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *databaseCommandClient) GetDatabaseValue(ctx context.Context, in *GetDatabaseValueRequest, opts ...grpc.CallOption) (*GetDatabaseValueResponse, error) {
	out := new(GetDatabaseValueResponse)
	err := c.cc.Invoke(ctx, "/rpcpb.DatabaseCommand/GetDatabaseValue", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DatabaseCommandServer is the server API for DatabaseCommand service.
type DatabaseCommandServer interface {
	// get all keys of database
	GetDatabaseKeys(context.Context, *GetDatabaseKeysRequest) (*GetDatabaseKeysResponse, error)
	// get value of associate with passed key in database
	GetDatabaseValue(context.Context, *GetDatabaseValueRequest) (*GetDatabaseValueResponse, error)
}

func RegisterDatabaseCommandServer(s *grpc.Server, srv DatabaseCommandServer) {
	s.RegisterService(&_DatabaseCommand_serviceDesc, srv)
}

func _DatabaseCommand_GetDatabaseKeys_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetDatabaseKeysRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DatabaseCommandServer).GetDatabaseKeys(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rpcpb.DatabaseCommand/GetDatabaseKeys",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DatabaseCommandServer).GetDatabaseKeys(ctx, req.(*GetDatabaseKeysRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DatabaseCommand_GetDatabaseValue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetDatabaseValueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DatabaseCommandServer).GetDatabaseValue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rpcpb.DatabaseCommand/GetDatabaseValue",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DatabaseCommandServer).GetDatabaseValue(ctx, req.(*GetDatabaseValueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _DatabaseCommand_serviceDesc = grpc.ServiceDesc{
	ServiceName: "rpcpb.DatabaseCommand",
	HandlerType: (*DatabaseCommandServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetDatabaseKeys",
			Handler:    _DatabaseCommand_GetDatabaseKeys_Handler,
		},
		{
			MethodName: "GetDatabaseValue",
			Handler:    _DatabaseCommand_GetDatabaseValue_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "db.proto",
}

func (m *GetDatabaseKeysRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GetDatabaseKeysRequest) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Table) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintDb(dAtA, i, uint64(len(m.Table)))
		i += copy(dAtA[i:], m.Table)
	}
	if len(m.Prefix) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintDb(dAtA, i, uint64(len(m.Prefix)))
		i += copy(dAtA[i:], m.Prefix)
	}
	if m.Skip != 0 {
		dAtA[i] = 0x18
		i++
		i = encodeVarintDb(dAtA, i, uint64(m.Skip))
	}
	if m.Limit != 0 {
		dAtA[i] = 0x20
		i++
		i = encodeVarintDb(dAtA, i, uint64(m.Limit))
	}
	return i, nil
}

func (m *GetDatabaseKeysResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GetDatabaseKeysResponse) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Code != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintDb(dAtA, i, uint64(m.Code))
	}
	if len(m.Message) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintDb(dAtA, i, uint64(len(m.Message)))
		i += copy(dAtA[i:], m.Message)
	}
	if m.Skip != 0 {
		dAtA[i] = 0x18
		i++
		i = encodeVarintDb(dAtA, i, uint64(m.Skip))
	}
	if len(m.Keys) > 0 {
		for _, s := range m.Keys {
			dAtA[i] = 0x22
			i++
			l = len(s)
			for l >= 1<<7 {
				dAtA[i] = uint8(uint64(l)&0x7f | 0x80)
				l >>= 7
				i++
			}
			dAtA[i] = uint8(l)
			i++
			i += copy(dAtA[i:], s)
		}
	}
	return i, nil
}

func (m *GetDatabaseValueRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GetDatabaseValueRequest) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Table) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintDb(dAtA, i, uint64(len(m.Table)))
		i += copy(dAtA[i:], m.Table)
	}
	if len(m.Key) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintDb(dAtA, i, uint64(len(m.Key)))
		i += copy(dAtA[i:], m.Key)
	}
	return i, nil
}

func (m *GetDatabaseValueResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GetDatabaseValueResponse) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Code != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintDb(dAtA, i, uint64(m.Code))
	}
	if len(m.Message) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintDb(dAtA, i, uint64(len(m.Message)))
		i += copy(dAtA[i:], m.Message)
	}
	if len(m.Value) > 0 {
		dAtA[i] = 0x1a
		i++
		i = encodeVarintDb(dAtA, i, uint64(len(m.Value)))
		i += copy(dAtA[i:], m.Value)
	}
	return i, nil
}

func encodeVarintDb(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *GetDatabaseKeysRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Table)
	if l > 0 {
		n += 1 + l + sovDb(uint64(l))
	}
	l = len(m.Prefix)
	if l > 0 {
		n += 1 + l + sovDb(uint64(l))
	}
	if m.Skip != 0 {
		n += 1 + sovDb(uint64(m.Skip))
	}
	if m.Limit != 0 {
		n += 1 + sovDb(uint64(m.Limit))
	}
	return n
}

func (m *GetDatabaseKeysResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Code != 0 {
		n += 1 + sovDb(uint64(m.Code))
	}
	l = len(m.Message)
	if l > 0 {
		n += 1 + l + sovDb(uint64(l))
	}
	if m.Skip != 0 {
		n += 1 + sovDb(uint64(m.Skip))
	}
	if len(m.Keys) > 0 {
		for _, s := range m.Keys {
			l = len(s)
			n += 1 + l + sovDb(uint64(l))
		}
	}
	return n
}

func (m *GetDatabaseValueRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Table)
	if l > 0 {
		n += 1 + l + sovDb(uint64(l))
	}
	l = len(m.Key)
	if l > 0 {
		n += 1 + l + sovDb(uint64(l))
	}
	return n
}

func (m *GetDatabaseValueResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Code != 0 {
		n += 1 + sovDb(uint64(m.Code))
	}
	l = len(m.Message)
	if l > 0 {
		n += 1 + l + sovDb(uint64(l))
	}
	l = len(m.Value)
	if l > 0 {
		n += 1 + l + sovDb(uint64(l))
	}
	return n
}

func sovDb(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozDb(x uint64) (n int) {
	return sovDb(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *GetDatabaseKeysRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDb
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
			return fmt.Errorf("proto: GetDatabaseKeysRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GetDatabaseKeysRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Table", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
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
				return ErrInvalidLengthDb
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Table = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Prefix", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
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
				return ErrInvalidLengthDb
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Prefix = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Skip", wireType)
			}
			m.Skip = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Skip |= (int32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Limit", wireType)
			}
			m.Limit = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Limit |= (int32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipDb(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthDb
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
func (m *GetDatabaseKeysResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDb
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
			return fmt.Errorf("proto: GetDatabaseKeysResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GetDatabaseKeysResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Code", wireType)
			}
			m.Code = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Code |= (int32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Message", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
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
				return ErrInvalidLengthDb
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Message = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Skip", wireType)
			}
			m.Skip = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Skip |= (int32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Keys", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
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
				return ErrInvalidLengthDb
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Keys = append(m.Keys, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDb(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthDb
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
func (m *GetDatabaseValueRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDb
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
			return fmt.Errorf("proto: GetDatabaseValueRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GetDatabaseValueRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Table", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
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
				return ErrInvalidLengthDb
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Table = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Key", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
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
				return ErrInvalidLengthDb
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Key = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDb(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthDb
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
func (m *GetDatabaseValueResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDb
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
			return fmt.Errorf("proto: GetDatabaseValueResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GetDatabaseValueResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Code", wireType)
			}
			m.Code = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Code |= (int32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Message", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
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
				return ErrInvalidLengthDb
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Message = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Value", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDb
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
				return ErrInvalidLengthDb
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Value = append(m.Value[:0], dAtA[iNdEx:postIndex]...)
			if m.Value == nil {
				m.Value = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDb(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthDb
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
func skipDb(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowDb
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
					return 0, ErrIntOverflowDb
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
					return 0, ErrIntOverflowDb
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
				return 0, ErrInvalidLengthDb
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowDb
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
				next, err := skipDb(dAtA[start:])
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
	ErrInvalidLengthDb = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowDb   = fmt.Errorf("proto: integer overflow")
)
