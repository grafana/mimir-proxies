// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v3.21.12
// source: protos/errorx/v1/errors.proto

package errorxpb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// ErrorxType lists all of the errorx types that we have. The conversion
// function ignores the GRPC status type when doing the conversion.
type ErrorxType int32

const (
	ErrorxType_UNKNOWN                ErrorxType = 0
	ErrorxType_INTERNAL               ErrorxType = 1
	ErrorxType_BAD_REQUEST            ErrorxType = 2
	ErrorxType_REQUIRES_PROXY_REQUEST ErrorxType = 3
	ErrorxType_RATE_LIMITED           ErrorxType = 4
	ErrorxType_DISABLED               ErrorxType = 5
	ErrorxType_UNIMPLEMENTED          ErrorxType = 6
	ErrorxType_UNPROCESSABLE_ENTITY   ErrorxType = 7
	ErrorxType_CONFLICT               ErrorxType = 8
	ErrorxType_TOO_MANY_REQUESTS      ErrorxType = 9
	ErrorxType_UNSUPPORTED_MEDIA_TYPE ErrorxType = 10
)

// Enum value maps for ErrorxType.
var (
	ErrorxType_name = map[int32]string{
		0:  "UNKNOWN",
		1:  "INTERNAL",
		2:  "BAD_REQUEST",
		3:  "REQUIRES_PROXY_REQUEST",
		4:  "RATE_LIMITED",
		5:  "DISABLED",
		6:  "UNIMPLEMENTED",
		7:  "UNPROCESSABLE_ENTITY",
		8:  "CONFLICT",
		9:  "TOO_MANY_REQUESTS",
		10: "UNSUPPORTED_MEDIA_TYPE",
	}
	ErrorxType_value = map[string]int32{
		"UNKNOWN":                0,
		"INTERNAL":               1,
		"BAD_REQUEST":            2,
		"REQUIRES_PROXY_REQUEST": 3,
		"RATE_LIMITED":           4,
		"DISABLED":               5,
		"UNIMPLEMENTED":          6,
		"UNPROCESSABLE_ENTITY":   7,
		"CONFLICT":               8,
		"TOO_MANY_REQUESTS":      9,
		"UNSUPPORTED_MEDIA_TYPE": 10,
	}
)

func (x ErrorxType) Enum() *ErrorxType {
	p := new(ErrorxType)
	*p = x
	return p
}

func (x ErrorxType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ErrorxType) Descriptor() protoreflect.EnumDescriptor {
	return file_protos_errorx_v1_errors_proto_enumTypes[0].Descriptor()
}

func (ErrorxType) Type() protoreflect.EnumType {
	return &file_protos_errorx_v1_errors_proto_enumTypes[0]
}

func (x ErrorxType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ErrorxType.Descriptor instead.
func (ErrorxType) EnumDescriptor() ([]byte, []int) {
	return file_protos_errorx_v1_errors_proto_rawDescGZIP(), []int{0}
}

// ErrorDetails are set for GRPC Status responses to provide extra context for
// conversion back to an errorx type.
type ErrorDetails struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type ErrorxType `protobuf:"varint,1,opt,name=type,proto3,enum=errorx.ErrorxType" json:"type,omitempty"`
	// Reason is used by RequiresProxyRequest for logging.
	Reason string `protobuf:"bytes,2,opt,name=Reason,proto3" json:"Reason,omitempty"`
}

func (x *ErrorDetails) Reset() {
	*x = ErrorDetails{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protos_errorx_v1_errors_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ErrorDetails) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ErrorDetails) ProtoMessage() {}

func (x *ErrorDetails) ProtoReflect() protoreflect.Message {
	mi := &file_protos_errorx_v1_errors_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ErrorDetails.ProtoReflect.Descriptor instead.
func (*ErrorDetails) Descriptor() ([]byte, []int) {
	return file_protos_errorx_v1_errors_proto_rawDescGZIP(), []int{0}
}

func (x *ErrorDetails) GetType() ErrorxType {
	if x != nil {
		return x.Type
	}
	return ErrorxType_UNKNOWN
}

func (x *ErrorDetails) GetReason() string {
	if x != nil {
		return x.Reason
	}
	return ""
}

var File_protos_errorx_v1_errors_proto protoreflect.FileDescriptor

var file_protos_errorx_v1_errors_proto_rawDesc = []byte{
	0x0a, 0x1d, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x78, 0x2f,
	0x76, 0x31, 0x2f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x06, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x78, 0x22, 0x4e, 0x0a, 0x0c, 0x45, 0x72, 0x72, 0x6f, 0x72,
	0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x12, 0x26, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x12, 0x2e, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x78, 0x2e, 0x45,
	0x72, 0x72, 0x6f, 0x72, 0x78, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12,
	0x16, 0x0a, 0x06, 0x52, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x06, 0x52, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x2a, 0xe2, 0x01, 0x0a, 0x0a, 0x45, 0x72, 0x72, 0x6f,
	0x72, 0x78, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57,
	0x4e, 0x10, 0x00, 0x12, 0x0c, 0x0a, 0x08, 0x49, 0x4e, 0x54, 0x45, 0x52, 0x4e, 0x41, 0x4c, 0x10,
	0x01, 0x12, 0x0f, 0x0a, 0x0b, 0x42, 0x41, 0x44, 0x5f, 0x52, 0x45, 0x51, 0x55, 0x45, 0x53, 0x54,
	0x10, 0x02, 0x12, 0x1a, 0x0a, 0x16, 0x52, 0x45, 0x51, 0x55, 0x49, 0x52, 0x45, 0x53, 0x5f, 0x50,
	0x52, 0x4f, 0x58, 0x59, 0x5f, 0x52, 0x45, 0x51, 0x55, 0x45, 0x53, 0x54, 0x10, 0x03, 0x12, 0x10,
	0x0a, 0x0c, 0x52, 0x41, 0x54, 0x45, 0x5f, 0x4c, 0x49, 0x4d, 0x49, 0x54, 0x45, 0x44, 0x10, 0x04,
	0x12, 0x0c, 0x0a, 0x08, 0x44, 0x49, 0x53, 0x41, 0x42, 0x4c, 0x45, 0x44, 0x10, 0x05, 0x12, 0x11,
	0x0a, 0x0d, 0x55, 0x4e, 0x49, 0x4d, 0x50, 0x4c, 0x45, 0x4d, 0x45, 0x4e, 0x54, 0x45, 0x44, 0x10,
	0x06, 0x12, 0x18, 0x0a, 0x14, 0x55, 0x4e, 0x50, 0x52, 0x4f, 0x43, 0x45, 0x53, 0x53, 0x41, 0x42,
	0x4c, 0x45, 0x5f, 0x45, 0x4e, 0x54, 0x49, 0x54, 0x59, 0x10, 0x07, 0x12, 0x0c, 0x0a, 0x08, 0x43,
	0x4f, 0x4e, 0x46, 0x4c, 0x49, 0x43, 0x54, 0x10, 0x08, 0x12, 0x15, 0x0a, 0x11, 0x54, 0x4f, 0x4f,
	0x5f, 0x4d, 0x41, 0x4e, 0x59, 0x5f, 0x52, 0x45, 0x51, 0x55, 0x45, 0x53, 0x54, 0x53, 0x10, 0x09,
	0x12, 0x1a, 0x0a, 0x16, 0x55, 0x4e, 0x53, 0x55, 0x50, 0x50, 0x4f, 0x52, 0x54, 0x45, 0x44, 0x5f,
	0x4d, 0x45, 0x44, 0x49, 0x41, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x10, 0x0a, 0x42, 0x0e, 0x5a, 0x0c,
	0x70, 0x6b, 0x67, 0x2f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x78, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_protos_errorx_v1_errors_proto_rawDescOnce sync.Once
	file_protos_errorx_v1_errors_proto_rawDescData = file_protos_errorx_v1_errors_proto_rawDesc
)

func file_protos_errorx_v1_errors_proto_rawDescGZIP() []byte {
	file_protos_errorx_v1_errors_proto_rawDescOnce.Do(func() {
		file_protos_errorx_v1_errors_proto_rawDescData = protoimpl.X.CompressGZIP(file_protos_errorx_v1_errors_proto_rawDescData)
	})
	return file_protos_errorx_v1_errors_proto_rawDescData
}

var file_protos_errorx_v1_errors_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_protos_errorx_v1_errors_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_protos_errorx_v1_errors_proto_goTypes = []interface{}{
	(ErrorxType)(0),      // 0: errorx.ErrorxType
	(*ErrorDetails)(nil), // 1: errorx.ErrorDetails
}
var file_protos_errorx_v1_errors_proto_depIdxs = []int32{
	0, // 0: errorx.ErrorDetails.type:type_name -> errorx.ErrorxType
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_protos_errorx_v1_errors_proto_init() }
func file_protos_errorx_v1_errors_proto_init() {
	if File_protos_errorx_v1_errors_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_protos_errorx_v1_errors_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ErrorDetails); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_protos_errorx_v1_errors_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_protos_errorx_v1_errors_proto_goTypes,
		DependencyIndexes: file_protos_errorx_v1_errors_proto_depIdxs,
		EnumInfos:         file_protos_errorx_v1_errors_proto_enumTypes,
		MessageInfos:      file_protos_errorx_v1_errors_proto_msgTypes,
	}.Build()
	File_protos_errorx_v1_errors_proto = out.File
	file_protos_errorx_v1_errors_proto_rawDesc = nil
	file_protos_errorx_v1_errors_proto_goTypes = nil
	file_protos_errorx_v1_errors_proto_depIdxs = nil
}
