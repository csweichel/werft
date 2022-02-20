// Code generated by protoc-gen-go. DO NOT EDIT.
// source: werft-ui.proto

package v2

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

type ListJobSpecsRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ListJobSpecsRequest) Reset()         { *m = ListJobSpecsRequest{} }
func (m *ListJobSpecsRequest) String() string { return proto.CompactTextString(m) }
func (*ListJobSpecsRequest) ProtoMessage()    {}
func (*ListJobSpecsRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_8d41ca2a021dc92d, []int{0}
}

func (m *ListJobSpecsRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ListJobSpecsRequest.Unmarshal(m, b)
}
func (m *ListJobSpecsRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ListJobSpecsRequest.Marshal(b, m, deterministic)
}
func (m *ListJobSpecsRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ListJobSpecsRequest.Merge(m, src)
}
func (m *ListJobSpecsRequest) XXX_Size() int {
	return xxx_messageInfo_ListJobSpecsRequest.Size(m)
}
func (m *ListJobSpecsRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ListJobSpecsRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ListJobSpecsRequest proto.InternalMessageInfo

type ListJobSpecsResponse struct {
	Repo                 *Repository          `protobuf:"bytes,1,opt,name=repo,proto3" json:"repo,omitempty"`
	Name                 string               `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Path                 string               `protobuf:"bytes,3,opt,name=path,proto3" json:"path,omitempty"`
	Description          string               `protobuf:"bytes,4,opt,name=description,proto3" json:"description,omitempty"`
	Arguments            []*DesiredAnnotation `protobuf:"bytes,5,rep,name=arguments,proto3" json:"arguments,omitempty"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *ListJobSpecsResponse) Reset()         { *m = ListJobSpecsResponse{} }
func (m *ListJobSpecsResponse) String() string { return proto.CompactTextString(m) }
func (*ListJobSpecsResponse) ProtoMessage()    {}
func (*ListJobSpecsResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_8d41ca2a021dc92d, []int{1}
}

func (m *ListJobSpecsResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ListJobSpecsResponse.Unmarshal(m, b)
}
func (m *ListJobSpecsResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ListJobSpecsResponse.Marshal(b, m, deterministic)
}
func (m *ListJobSpecsResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ListJobSpecsResponse.Merge(m, src)
}
func (m *ListJobSpecsResponse) XXX_Size() int {
	return xxx_messageInfo_ListJobSpecsResponse.Size(m)
}
func (m *ListJobSpecsResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ListJobSpecsResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ListJobSpecsResponse proto.InternalMessageInfo

func (m *ListJobSpecsResponse) GetRepo() *Repository {
	if m != nil {
		return m.Repo
	}
	return nil
}

func (m *ListJobSpecsResponse) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *ListJobSpecsResponse) GetPath() string {
	if m != nil {
		return m.Path
	}
	return ""
}

func (m *ListJobSpecsResponse) GetDescription() string {
	if m != nil {
		return m.Description
	}
	return ""
}

func (m *ListJobSpecsResponse) GetArguments() []*DesiredAnnotation {
	if m != nil {
		return m.Arguments
	}
	return nil
}

// DesiredAnnotation describes an annotation a job should have
type DesiredAnnotation struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Required             bool     `protobuf:"varint,2,opt,name=required,proto3" json:"required,omitempty"`
	Description          string   `protobuf:"bytes,3,opt,name=description,proto3" json:"description,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DesiredAnnotation) Reset()         { *m = DesiredAnnotation{} }
func (m *DesiredAnnotation) String() string { return proto.CompactTextString(m) }
func (*DesiredAnnotation) ProtoMessage()    {}
func (*DesiredAnnotation) Descriptor() ([]byte, []int) {
	return fileDescriptor_8d41ca2a021dc92d, []int{2}
}

func (m *DesiredAnnotation) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DesiredAnnotation.Unmarshal(m, b)
}
func (m *DesiredAnnotation) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DesiredAnnotation.Marshal(b, m, deterministic)
}
func (m *DesiredAnnotation) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DesiredAnnotation.Merge(m, src)
}
func (m *DesiredAnnotation) XXX_Size() int {
	return xxx_messageInfo_DesiredAnnotation.Size(m)
}
func (m *DesiredAnnotation) XXX_DiscardUnknown() {
	xxx_messageInfo_DesiredAnnotation.DiscardUnknown(m)
}

var xxx_messageInfo_DesiredAnnotation proto.InternalMessageInfo

func (m *DesiredAnnotation) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *DesiredAnnotation) GetRequired() bool {
	if m != nil {
		return m.Required
	}
	return false
}

func (m *DesiredAnnotation) GetDescription() string {
	if m != nil {
		return m.Description
	}
	return ""
}

type IsReadOnlyRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *IsReadOnlyRequest) Reset()         { *m = IsReadOnlyRequest{} }
func (m *IsReadOnlyRequest) String() string { return proto.CompactTextString(m) }
func (*IsReadOnlyRequest) ProtoMessage()    {}
func (*IsReadOnlyRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_8d41ca2a021dc92d, []int{3}
}

func (m *IsReadOnlyRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_IsReadOnlyRequest.Unmarshal(m, b)
}
func (m *IsReadOnlyRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_IsReadOnlyRequest.Marshal(b, m, deterministic)
}
func (m *IsReadOnlyRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_IsReadOnlyRequest.Merge(m, src)
}
func (m *IsReadOnlyRequest) XXX_Size() int {
	return xxx_messageInfo_IsReadOnlyRequest.Size(m)
}
func (m *IsReadOnlyRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_IsReadOnlyRequest.DiscardUnknown(m)
}

var xxx_messageInfo_IsReadOnlyRequest proto.InternalMessageInfo

type IsReadOnlyResponse struct {
	Readonly             bool     `protobuf:"varint,1,opt,name=readonly,proto3" json:"readonly,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *IsReadOnlyResponse) Reset()         { *m = IsReadOnlyResponse{} }
func (m *IsReadOnlyResponse) String() string { return proto.CompactTextString(m) }
func (*IsReadOnlyResponse) ProtoMessage()    {}
func (*IsReadOnlyResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_8d41ca2a021dc92d, []int{4}
}

func (m *IsReadOnlyResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_IsReadOnlyResponse.Unmarshal(m, b)
}
func (m *IsReadOnlyResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_IsReadOnlyResponse.Marshal(b, m, deterministic)
}
func (m *IsReadOnlyResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_IsReadOnlyResponse.Merge(m, src)
}
func (m *IsReadOnlyResponse) XXX_Size() int {
	return xxx_messageInfo_IsReadOnlyResponse.Size(m)
}
func (m *IsReadOnlyResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_IsReadOnlyResponse.DiscardUnknown(m)
}

var xxx_messageInfo_IsReadOnlyResponse proto.InternalMessageInfo

func (m *IsReadOnlyResponse) GetReadonly() bool {
	if m != nil {
		return m.Readonly
	}
	return false
}

func init() {
	proto.RegisterType((*ListJobSpecsRequest)(nil), "v2.ListJobSpecsRequest")
	proto.RegisterType((*ListJobSpecsResponse)(nil), "v2.ListJobSpecsResponse")
	proto.RegisterType((*DesiredAnnotation)(nil), "v2.DesiredAnnotation")
	proto.RegisterType((*IsReadOnlyRequest)(nil), "v2.IsReadOnlyRequest")
	proto.RegisterType((*IsReadOnlyResponse)(nil), "v2.IsReadOnlyResponse")
}

func init() {
	proto.RegisterFile("werft-ui.proto", fileDescriptor_8d41ca2a021dc92d)
}

var fileDescriptor_8d41ca2a021dc92d = []byte{
	// 356 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x92, 0x41, 0x4f, 0xe2, 0x40,
	0x14, 0xc7, 0xb7, 0xc0, 0xee, 0xc2, 0xeb, 0x86, 0x84, 0x61, 0xd1, 0xa6, 0xa7, 0xa6, 0x89, 0x91,
	0x8b, 0x2d, 0x29, 0x67, 0x0f, 0x1a, 0x3d, 0x60, 0x4c, 0x4c, 0x6a, 0x8c, 0x89, 0xb7, 0xd2, 0x3e,
	0x61, 0x22, 0xcc, 0x0c, 0x33, 0x53, 0x08, 0x9f, 0xc2, 0xcf, 0xe3, 0xb7, 0x33, 0x9d, 0x0a, 0x54,
	0xea, 0x6d, 0xe6, 0xf7, 0x5e, 0xfa, 0x7e, 0xfd, 0xcf, 0x83, 0xee, 0x06, 0xe5, 0xab, 0xbe, 0xc8,
	0x69, 0x20, 0x24, 0xd7, 0x9c, 0x34, 0xd6, 0x91, 0x6b, 0x1b, 0x56, 0x02, 0x7f, 0x00, 0xfd, 0x7b,
	0xaa, 0xf4, 0x1d, 0x9f, 0x3e, 0x0a, 0x4c, 0x55, 0x8c, 0xab, 0x1c, 0x95, 0xf6, 0x3f, 0x2c, 0xf8,
	0xff, 0x9d, 0x2b, 0xc1, 0x99, 0x42, 0xe2, 0x43, 0x4b, 0xa2, 0xe0, 0x8e, 0xe5, 0x59, 0x43, 0x3b,
	0xea, 0x06, 0xeb, 0x28, 0x88, 0x51, 0x70, 0x45, 0x35, 0x97, 0xdb, 0xd8, 0xd4, 0x08, 0x81, 0x16,
	0x4b, 0x96, 0xe8, 0x34, 0x3c, 0x6b, 0xd8, 0x89, 0xcd, 0xb9, 0x60, 0x22, 0xd1, 0x73, 0xa7, 0x59,
	0xb2, 0xe2, 0x4c, 0x3c, 0xb0, 0x33, 0x54, 0xa9, 0xa4, 0x42, 0x53, 0xce, 0x9c, 0x96, 0x29, 0x55,
	0x11, 0x19, 0x43, 0x27, 0x91, 0xb3, 0x7c, 0x89, 0x4c, 0x2b, 0xe7, 0xb7, 0xd7, 0x1c, 0xda, 0xd1,
	0xa0, 0x18, 0x79, 0x83, 0x8a, 0x4a, 0xcc, 0xae, 0x18, 0xe3, 0x3a, 0x29, 0x3a, 0xe3, 0x43, 0x9f,
	0x8f, 0xd0, 0xab, 0xd5, 0xf7, 0x4e, 0x56, 0xc5, 0xc9, 0x85, 0xb6, 0xc4, 0x55, 0x5e, 0x74, 0x1a,
	0xd7, 0x76, 0xbc, 0xbf, 0x1f, 0xbb, 0x35, 0x6b, 0x6e, 0x7e, 0x1f, 0x7a, 0x13, 0x15, 0x63, 0x92,
	0x3d, 0xb0, 0xc5, 0x76, 0x97, 0xdb, 0x08, 0x48, 0x15, 0x7e, 0x85, 0x66, 0x06, 0x25, 0x19, 0x67,
	0x8b, 0xad, 0x11, 0x30, 0x83, 0xca, 0x7b, 0xf4, 0x6e, 0xc1, 0xdf, 0xe7, 0xe2, 0x41, 0x9e, 0x26,
	0xe4, 0x16, 0xfe, 0x55, 0x43, 0x27, 0xa7, 0xc5, 0xbf, 0xfe, 0xf0, 0x3c, 0xae, 0x53, 0x2f, 0x94,
	0xa3, 0xfc, 0x5f, 0x23, 0x8b, 0x5c, 0x02, 0x1c, 0x24, 0x88, 0x09, 0xac, 0x66, 0xea, 0x9e, 0x1c,
	0xe3, 0xdd, 0x07, 0xae, 0xcf, 0x5f, 0xce, 0x66, 0x54, 0xcf, 0xf3, 0x69, 0x90, 0xf2, 0x65, 0x98,
	0xaa, 0x0d, 0xd2, 0x74, 0x8e, 0x8b, 0xd0, 0xac, 0x4d, 0x28, 0xde, 0x66, 0x61, 0x22, 0x68, 0xb8,
	0x8e, 0xa6, 0x7f, 0xcc, 0x0a, 0x8d, 0x3f, 0x03, 0x00, 0x00, 0xff, 0xff, 0x47, 0xf8, 0xf3, 0xe3,
	0x65, 0x02, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// WerftUIClient is the client API for WerftUI service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type WerftUIClient interface {
	// ListJobSpecs returns a list of jobs that can be started through the UI.
	ListJobSpecs(ctx context.Context, in *ListJobSpecsRequest, opts ...grpc.CallOption) (WerftUI_ListJobSpecsClient, error)
	// IsReadOnly returns true if the UI is readonly.
	IsReadOnly(ctx context.Context, in *IsReadOnlyRequest, opts ...grpc.CallOption) (*IsReadOnlyResponse, error)
}

type werftUIClient struct {
	cc grpc.ClientConnInterface
}

func NewWerftUIClient(cc grpc.ClientConnInterface) WerftUIClient {
	return &werftUIClient{cc}
}

func (c *werftUIClient) ListJobSpecs(ctx context.Context, in *ListJobSpecsRequest, opts ...grpc.CallOption) (WerftUI_ListJobSpecsClient, error) {
	stream, err := c.cc.NewStream(ctx, &_WerftUI_serviceDesc.Streams[0], "/v2.WerftUI/ListJobSpecs", opts...)
	if err != nil {
		return nil, err
	}
	x := &werftUIListJobSpecsClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type WerftUI_ListJobSpecsClient interface {
	Recv() (*ListJobSpecsResponse, error)
	grpc.ClientStream
}

type werftUIListJobSpecsClient struct {
	grpc.ClientStream
}

func (x *werftUIListJobSpecsClient) Recv() (*ListJobSpecsResponse, error) {
	m := new(ListJobSpecsResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *werftUIClient) IsReadOnly(ctx context.Context, in *IsReadOnlyRequest, opts ...grpc.CallOption) (*IsReadOnlyResponse, error) {
	out := new(IsReadOnlyResponse)
	err := c.cc.Invoke(ctx, "/v2.WerftUI/IsReadOnly", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// WerftUIServer is the server API for WerftUI service.
type WerftUIServer interface {
	// ListJobSpecs returns a list of jobs that can be started through the UI.
	ListJobSpecs(*ListJobSpecsRequest, WerftUI_ListJobSpecsServer) error
	// IsReadOnly returns true if the UI is readonly.
	IsReadOnly(context.Context, *IsReadOnlyRequest) (*IsReadOnlyResponse, error)
}

// UnimplementedWerftUIServer can be embedded to have forward compatible implementations.
type UnimplementedWerftUIServer struct {
}

func (*UnimplementedWerftUIServer) ListJobSpecs(req *ListJobSpecsRequest, srv WerftUI_ListJobSpecsServer) error {
	return status.Errorf(codes.Unimplemented, "method ListJobSpecs not implemented")
}
func (*UnimplementedWerftUIServer) IsReadOnly(ctx context.Context, req *IsReadOnlyRequest) (*IsReadOnlyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method IsReadOnly not implemented")
}

func RegisterWerftUIServer(s *grpc.Server, srv WerftUIServer) {
	s.RegisterService(&_WerftUI_serviceDesc, srv)
}

func _WerftUI_ListJobSpecs_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ListJobSpecsRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(WerftUIServer).ListJobSpecs(m, &werftUIListJobSpecsServer{stream})
}

type WerftUI_ListJobSpecsServer interface {
	Send(*ListJobSpecsResponse) error
	grpc.ServerStream
}

type werftUIListJobSpecsServer struct {
	grpc.ServerStream
}

func (x *werftUIListJobSpecsServer) Send(m *ListJobSpecsResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _WerftUI_IsReadOnly_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(IsReadOnlyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WerftUIServer).IsReadOnly(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/v2.WerftUI/IsReadOnly",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WerftUIServer).IsReadOnly(ctx, req.(*IsReadOnlyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _WerftUI_serviceDesc = grpc.ServiceDesc{
	ServiceName: "v2.WerftUI",
	HandlerType: (*WerftUIServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "IsReadOnly",
			Handler:    _WerftUI_IsReadOnly_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ListJobSpecs",
			Handler:       _WerftUI_ListJobSpecs_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "werft-ui.proto",
}
