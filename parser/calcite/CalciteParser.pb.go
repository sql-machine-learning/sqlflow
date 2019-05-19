// Code generated by protoc-gen-go. DO NOT EDIT.
// source: CalciteParser.proto

package CalciteParser

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type CalciteParserRequest struct {
	Query                string   `protobuf:"bytes,1,opt,name=query,proto3" json:"query,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CalciteParserRequest) Reset()         { *m = CalciteParserRequest{} }
func (m *CalciteParserRequest) String() string { return proto.CompactTextString(m) }
func (*CalciteParserRequest) ProtoMessage()    {}
func (*CalciteParserRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_ce0f02fb19fab2e3, []int{0}
}

func (m *CalciteParserRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CalciteParserRequest.Unmarshal(m, b)
}
func (m *CalciteParserRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CalciteParserRequest.Marshal(b, m, deterministic)
}
func (m *CalciteParserRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CalciteParserRequest.Merge(m, src)
}
func (m *CalciteParserRequest) XXX_Size() int {
	return xxx_messageInfo_CalciteParserRequest.Size(m)
}
func (m *CalciteParserRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CalciteParserRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CalciteParserRequest proto.InternalMessageInfo

func (m *CalciteParserRequest) GetQuery() string {
	if m != nil {
		return m.Query
	}
	return ""
}

type CalciteParserReply struct {
	Sql                  string   `protobuf:"bytes,1,opt,name=sql,proto3" json:"sql,omitempty"`
	Extension            string   `protobuf:"bytes,2,opt,name=extension,proto3" json:"extension,omitempty"`
	Error                string   `protobuf:"bytes,3,opt,name=error,proto3" json:"error,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CalciteParserReply) Reset()         { *m = CalciteParserReply{} }
func (m *CalciteParserReply) String() string { return proto.CompactTextString(m) }
func (*CalciteParserReply) ProtoMessage()    {}
func (*CalciteParserReply) Descriptor() ([]byte, []int) {
	return fileDescriptor_ce0f02fb19fab2e3, []int{1}
}

func (m *CalciteParserReply) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CalciteParserReply.Unmarshal(m, b)
}
func (m *CalciteParserReply) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CalciteParserReply.Marshal(b, m, deterministic)
}
func (m *CalciteParserReply) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CalciteParserReply.Merge(m, src)
}
func (m *CalciteParserReply) XXX_Size() int {
	return xxx_messageInfo_CalciteParserReply.Size(m)
}
func (m *CalciteParserReply) XXX_DiscardUnknown() {
	xxx_messageInfo_CalciteParserReply.DiscardUnknown(m)
}

var xxx_messageInfo_CalciteParserReply proto.InternalMessageInfo

func (m *CalciteParserReply) GetSql() string {
	if m != nil {
		return m.Sql
	}
	return ""
}

func (m *CalciteParserReply) GetExtension() string {
	if m != nil {
		return m.Extension
	}
	return ""
}

func (m *CalciteParserReply) GetError() string {
	if m != nil {
		return m.Error
	}
	return ""
}

func init() {
	proto.RegisterType((*CalciteParserRequest)(nil), "CalciteParserRequest")
	proto.RegisterType((*CalciteParserReply)(nil), "CalciteParserReply")
}

func init() { proto.RegisterFile("CalciteParser.proto", fileDescriptor_ce0f02fb19fab2e3) }

var fileDescriptor_ce0f02fb19fab2e3 = []byte{
	// 168 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x76, 0x4e, 0xcc, 0x49,
	0xce, 0x2c, 0x49, 0x0d, 0x48, 0x2c, 0x2a, 0x4e, 0x2d, 0xd2, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x57,
	0xd2, 0xe1, 0x12, 0x41, 0x11, 0x0e, 0x4a, 0x2d, 0x2c, 0x4d, 0x2d, 0x2e, 0x11, 0x12, 0xe1, 0x62,
	0x2d, 0x2c, 0x4d, 0x2d, 0xaa, 0x94, 0x60, 0x54, 0x60, 0xd4, 0xe0, 0x0c, 0x82, 0x70, 0x94, 0xa2,
	0xb8, 0x84, 0xd0, 0x54, 0x17, 0xe4, 0x54, 0x0a, 0x09, 0x70, 0x31, 0x17, 0x17, 0xe6, 0x40, 0x55,
	0x82, 0x98, 0x42, 0x32, 0x5c, 0x9c, 0xa9, 0x15, 0x25, 0xa9, 0x79, 0xc5, 0x99, 0xf9, 0x79, 0x12,
	0x4c, 0x60, 0x71, 0x84, 0x00, 0xc8, 0xec, 0xd4, 0xa2, 0xa2, 0xfc, 0x22, 0x09, 0x66, 0x88, 0xd9,
	0x60, 0x8e, 0x91, 0x1b, 0x17, 0x2f, 0x8a, 0xd9, 0x42, 0xa6, 0x5c, 0xac, 0x60, 0x96, 0x90, 0xa8,
	0x1e, 0x36, 0x27, 0x4a, 0x09, 0xeb, 0x61, 0xba, 0x45, 0x89, 0xc1, 0x49, 0xcc, 0x09, 0xd5, 0x8d,
	0x01, 0x20, 0x7f, 0x06, 0x30, 0x24, 0xb1, 0x81, 0x3d, 0x6c, 0x0c, 0x08, 0x00, 0x00, 0xff, 0xff,
	0x51, 0x84, 0x88, 0xa2, 0x07, 0x01, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// CalciteParserClient is the client API for CalciteParser service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type CalciteParserClient interface {
	Parse(ctx context.Context, in *CalciteParserRequest, opts ...grpc.CallOption) (*CalciteParserReply, error)
}

type calciteParserClient struct {
	cc *grpc.ClientConn
}

func NewCalciteParserClient(cc *grpc.ClientConn) CalciteParserClient {
	return &calciteParserClient{cc}
}

func (c *calciteParserClient) Parse(ctx context.Context, in *CalciteParserRequest, opts ...grpc.CallOption) (*CalciteParserReply, error) {
	out := new(CalciteParserReply)
	err := c.cc.Invoke(ctx, "/CalciteParser/Parse", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CalciteParserServer is the server API for CalciteParser service.
type CalciteParserServer interface {
	Parse(context.Context, *CalciteParserRequest) (*CalciteParserReply, error)
}

// UnimplementedCalciteParserServer can be embedded to have forward compatible implementations.
type UnimplementedCalciteParserServer struct {
}

func (*UnimplementedCalciteParserServer) Parse(ctx context.Context, req *CalciteParserRequest) (*CalciteParserReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Parse not implemented")
}

func RegisterCalciteParserServer(s *grpc.Server, srv CalciteParserServer) {
	s.RegisterService(&_CalciteParser_serviceDesc, srv)
}

func _CalciteParser_Parse_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CalciteParserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CalciteParserServer).Parse(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/CalciteParser/Parse",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CalciteParserServer).Parse(ctx, req.(*CalciteParserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _CalciteParser_serviceDesc = grpc.ServiceDesc{
	ServiceName: "CalciteParser",
	HandlerType: (*CalciteParserServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Parse",
			Handler:    _CalciteParser_Parse_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "CalciteParser.proto",
}
