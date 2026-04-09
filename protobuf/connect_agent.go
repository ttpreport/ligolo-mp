package protobuf

// Hand-rolled proto message for ConnectAgentReq.
// Equivalent to:
//   syntax = "proto3";
//   package ligolo;
//   option go_package = "github.com/ttpreport/ligolo-mp/v2/protobuf";
//   message ConnectAgentReq { string Address = 1; }

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

// rawDesc encodes the FileDescriptorProto for the minimal proto file above.
var file_ligolo_connect_proto_rawDesc = []byte{
	// name = "ligolo_connect.proto"
	0x0a, 0x14, 0x6c, 0x69, 0x67, 0x6f, 0x6c, 0x6f, 0x5f, 0x63, 0x6f, 0x6e,
	0x6e, 0x65, 0x63, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	// package = "ligolo"
	0x12, 0x06, 0x6c, 0x69, 0x67, 0x6f, 0x6c, 0x6f,
	// message ConnectAgentReq { ... }  (43 bytes)
	0x22, 0x2b,
	// name = "ConnectAgentReq"
	0x0a, 0x0f, 0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x41, 0x67, 0x65,
	0x6e, 0x74, 0x52, 0x65, 0x71,
	// field: Address (24 bytes)
	0x12, 0x18,
	// name = "Address"
	0x0a, 0x07, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73,
	// number = 1
	0x18, 0x01,
	// label = LABEL_OPTIONAL
	0x20, 0x01,
	// type = TYPE_STRING
	0x28, 0x09,
	// json_name = "address"
	0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73,
	// options (44 bytes)
	0x42, 0x2c,
	// go_package = "github.com/ttpreport/ligolo-mp/v2/protobuf"
	0x5a, 0x2a,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x74,
	0x74, 0x70, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x2f, 0x6c, 0x69, 0x67,
	0x6f, 0x6c, 0x6f, 0x2d, 0x6d, 0x70, 0x2f, 0x76, 0x32, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	// syntax = "proto3"
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_ligolo_connect_proto_rawDescOnce sync.Once
	file_ligolo_connect_proto_rawDescData []byte
)

func file_ligolo_connect_proto_rawDescGZIP() []byte {
	file_ligolo_connect_proto_rawDescOnce.Do(func() {
		file_ligolo_connect_proto_rawDescData = file_ligolo_connect_proto_rawDesc
		file_ligolo_connect_proto_rawDescData = protoimpl.X.CompressGZIP(file_ligolo_connect_proto_rawDescData)
	})
	return file_ligolo_connect_proto_rawDescData
}

var file_ligolo_connect_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_ligolo_connect_proto_goTypes = []interface{}{
	(*ConnectAgentReq)(nil), // 0: ligolo.ConnectAgentReq
}
var file_ligolo_connect_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

var File_ligolo_connect_proto protoreflect.FileDescriptor

// ConnectAgentReq is the request message for the ConnectAgent RPC.
type ConnectAgentReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address string `protobuf:"bytes,1,opt,name=Address,proto3" json:"Address,omitempty"`
}

func (x *ConnectAgentReq) Reset() {
	*x = ConnectAgentReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_ligolo_connect_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ConnectAgentReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConnectAgentReq) ProtoMessage() {}

func (x *ConnectAgentReq) ProtoReflect() protoreflect.Message {
	mi := &file_ligolo_connect_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ConnectAgentReq) Descriptor() ([]byte, []int) {
	return file_ligolo_connect_proto_rawDescGZIP(), []int{0}
}

func (x *ConnectAgentReq) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func init() { file_ligolo_connect_proto_init() }

func file_ligolo_connect_proto_init() {
	if File_ligolo_connect_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_ligolo_connect_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ConnectAgentReq); i {
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
			RawDescriptor: file_ligolo_connect_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_ligolo_connect_proto_goTypes,
		DependencyIndexes: file_ligolo_connect_proto_depIdxs,
		MessageInfos:      file_ligolo_connect_proto_msgTypes,
	}.Build()
	File_ligolo_connect_proto = out.File
	file_ligolo_connect_proto_rawDesc = nil
	file_ligolo_connect_proto_goTypes = nil
	file_ligolo_connect_proto_depIdxs = nil
}
