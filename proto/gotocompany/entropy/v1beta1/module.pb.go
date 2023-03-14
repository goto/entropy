// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: gotocompany/entropy/v1beta1/module.proto

package entropyv1beta1

import (
	reflect "reflect"
	sync "sync"

	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	structpb "google.golang.org/protobuf/types/known/structpb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Module struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Urn       string                 `protobuf:"bytes,1,opt,name=urn,proto3" json:"urn,omitempty"`
	Name      string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Project   string                 `protobuf:"bytes,3,opt,name=project,proto3" json:"project,omitempty"`
	CreatedAt *timestamppb.Timestamp `protobuf:"bytes,4,opt,name=created_at,json=createdAt,proto3" json:"created_at,omitempty"`
	UpdatedAt *timestamppb.Timestamp `protobuf:"bytes,5,opt,name=updated_at,json=updatedAt,proto3" json:"updated_at,omitempty"`
	Configs   *structpb.Value        `protobuf:"bytes,7,opt,name=configs,proto3" json:"configs,omitempty"`
}

func (x *Module) Reset() {
	*x = Module{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Module) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Module) ProtoMessage() {}

func (x *Module) ProtoReflect() protoreflect.Message {
	mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Module.ProtoReflect.Descriptor instead.
func (*Module) Descriptor() ([]byte, []int) {
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP(), []int{0}
}

func (x *Module) GetUrn() string {
	if x != nil {
		return x.Urn
	}
	return ""
}

func (x *Module) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Module) GetProject() string {
	if x != nil {
		return x.Project
	}
	return ""
}

func (x *Module) GetCreatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.CreatedAt
	}
	return nil
}

func (x *Module) GetUpdatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.UpdatedAt
	}
	return nil
}

func (x *Module) GetConfigs() *structpb.Value {
	if x != nil {
		return x.Configs
	}
	return nil
}

type ListModulesRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Project string `protobuf:"bytes,1,opt,name=project,proto3" json:"project,omitempty"`
}

func (x *ListModulesRequest) Reset() {
	*x = ListModulesRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListModulesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListModulesRequest) ProtoMessage() {}

func (x *ListModulesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListModulesRequest.ProtoReflect.Descriptor instead.
func (*ListModulesRequest) Descriptor() ([]byte, []int) {
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP(), []int{1}
}

func (x *ListModulesRequest) GetProject() string {
	if x != nil {
		return x.Project
	}
	return ""
}

type ListModulesResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Modules []*Module `protobuf:"bytes,1,rep,name=modules,proto3" json:"modules,omitempty"`
}

func (x *ListModulesResponse) Reset() {
	*x = ListModulesResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListModulesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListModulesResponse) ProtoMessage() {}

func (x *ListModulesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListModulesResponse.ProtoReflect.Descriptor instead.
func (*ListModulesResponse) Descriptor() ([]byte, []int) {
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP(), []int{2}
}

func (x *ListModulesResponse) GetModules() []*Module {
	if x != nil {
		return x.Modules
	}
	return nil
}

type GetModuleRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Urn string `protobuf:"bytes,1,opt,name=urn,proto3" json:"urn,omitempty"`
}

func (x *GetModuleRequest) Reset() {
	*x = GetModuleRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetModuleRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetModuleRequest) ProtoMessage() {}

func (x *GetModuleRequest) ProtoReflect() protoreflect.Message {
	mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetModuleRequest.ProtoReflect.Descriptor instead.
func (*GetModuleRequest) Descriptor() ([]byte, []int) {
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP(), []int{3}
}

func (x *GetModuleRequest) GetUrn() string {
	if x != nil {
		return x.Urn
	}
	return ""
}

type GetModuleResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Module *Module `protobuf:"bytes,1,opt,name=module,proto3" json:"module,omitempty"`
}

func (x *GetModuleResponse) Reset() {
	*x = GetModuleResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetModuleResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetModuleResponse) ProtoMessage() {}

func (x *GetModuleResponse) ProtoReflect() protoreflect.Message {
	mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetModuleResponse.ProtoReflect.Descriptor instead.
func (*GetModuleResponse) Descriptor() ([]byte, []int) {
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP(), []int{4}
}

func (x *GetModuleResponse) GetModule() *Module {
	if x != nil {
		return x.Module
	}
	return nil
}

type CreateModuleRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Module *Module `protobuf:"bytes,1,opt,name=module,proto3" json:"module,omitempty"`
}

func (x *CreateModuleRequest) Reset() {
	*x = CreateModuleRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateModuleRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateModuleRequest) ProtoMessage() {}

func (x *CreateModuleRequest) ProtoReflect() protoreflect.Message {
	mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateModuleRequest.ProtoReflect.Descriptor instead.
func (*CreateModuleRequest) Descriptor() ([]byte, []int) {
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP(), []int{5}
}

func (x *CreateModuleRequest) GetModule() *Module {
	if x != nil {
		return x.Module
	}
	return nil
}

type CreateModuleResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Module *Module `protobuf:"bytes,1,opt,name=module,proto3" json:"module,omitempty"`
}

func (x *CreateModuleResponse) Reset() {
	*x = CreateModuleResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateModuleResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateModuleResponse) ProtoMessage() {}

func (x *CreateModuleResponse) ProtoReflect() protoreflect.Message {
	mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateModuleResponse.ProtoReflect.Descriptor instead.
func (*CreateModuleResponse) Descriptor() ([]byte, []int) {
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP(), []int{6}
}

func (x *CreateModuleResponse) GetModule() *Module {
	if x != nil {
		return x.Module
	}
	return nil
}

type UpdateModuleRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Urn     string          `protobuf:"bytes,1,opt,name=urn,proto3" json:"urn,omitempty"`
	Configs *structpb.Value `protobuf:"bytes,3,opt,name=configs,proto3" json:"configs,omitempty"`
}

func (x *UpdateModuleRequest) Reset() {
	*x = UpdateModuleRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateModuleRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateModuleRequest) ProtoMessage() {}

func (x *UpdateModuleRequest) ProtoReflect() protoreflect.Message {
	mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateModuleRequest.ProtoReflect.Descriptor instead.
func (*UpdateModuleRequest) Descriptor() ([]byte, []int) {
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP(), []int{7}
}

func (x *UpdateModuleRequest) GetUrn() string {
	if x != nil {
		return x.Urn
	}
	return ""
}

func (x *UpdateModuleRequest) GetConfigs() *structpb.Value {
	if x != nil {
		return x.Configs
	}
	return nil
}

type UpdateModuleResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Module *Module `protobuf:"bytes,1,opt,name=module,proto3" json:"module,omitempty"`
}

func (x *UpdateModuleResponse) Reset() {
	*x = UpdateModuleResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateModuleResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateModuleResponse) ProtoMessage() {}

func (x *UpdateModuleResponse) ProtoReflect() protoreflect.Message {
	mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateModuleResponse.ProtoReflect.Descriptor instead.
func (*UpdateModuleResponse) Descriptor() ([]byte, []int) {
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP(), []int{8}
}

func (x *UpdateModuleResponse) GetModule() *Module {
	if x != nil {
		return x.Module
	}
	return nil
}

type DeleteModuleRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Urn string `protobuf:"bytes,1,opt,name=urn,proto3" json:"urn,omitempty"`
}

func (x *DeleteModuleRequest) Reset() {
	*x = DeleteModuleRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteModuleRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteModuleRequest) ProtoMessage() {}

func (x *DeleteModuleRequest) ProtoReflect() protoreflect.Message {
	mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteModuleRequest.ProtoReflect.Descriptor instead.
func (*DeleteModuleRequest) Descriptor() ([]byte, []int) {
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP(), []int{9}
}

func (x *DeleteModuleRequest) GetUrn() string {
	if x != nil {
		return x.Urn
	}
	return ""
}

type DeleteModuleResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *DeleteModuleResponse) Reset() {
	*x = DeleteModuleResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteModuleResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteModuleResponse) ProtoMessage() {}

func (x *DeleteModuleResponse) ProtoReflect() protoreflect.Message {
	mi := &file_gotocompany_entropy_v1beta1_module_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteModuleResponse.ProtoReflect.Descriptor instead.
func (*DeleteModuleResponse) Descriptor() ([]byte, []int) {
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP(), []int{10}
}

var File_gotocompany_entropy_v1beta1_module_proto protoreflect.FileDescriptor

var file_gotocompany_entropy_v1beta1_module_proto_rawDesc = []byte{
	0x0a, 0x28, 0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2f, 0x65, 0x6e,
	0x74, 0x72, 0x6f, 0x70, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x6d, 0x6f,
	0x64, 0x75, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1b, 0x67, 0x6f, 0x74, 0x6f,
	0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x2e,
	0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f,
	0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0xf6, 0x01, 0x0a, 0x06, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x12,
	0x10, 0x0a, 0x03, 0x75, 0x72, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72,
	0x6e, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x12,
	0x39, 0x0a, 0x0a, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x5f, 0x61, 0x74, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52,
	0x09, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x41, 0x74, 0x12, 0x39, 0x0a, 0x0a, 0x75, 0x70,
	0x64, 0x61, 0x74, 0x65, 0x64, 0x5f, 0x61, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x75, 0x70, 0x64, 0x61,
	0x74, 0x65, 0x64, 0x41, 0x74, 0x12, 0x30, 0x0a, 0x07, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x73,
	0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x07,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x73, 0x4a, 0x04, 0x08, 0x06, 0x10, 0x07, 0x22, 0x2e, 0x0a,
	0x12, 0x4c, 0x69, 0x73, 0x74, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x54, 0x0a,
	0x13, 0x4c, 0x69, 0x73, 0x74, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x3d, 0x0a, 0x07, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x73, 0x18,
	0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x23, 0x2e, 0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6d, 0x70,
	0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65,
	0x74, 0x61, 0x31, 0x2e, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52, 0x07, 0x6d, 0x6f, 0x64, 0x75,
	0x6c, 0x65, 0x73, 0x22, 0x24, 0x0a, 0x10, 0x47, 0x65, 0x74, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x75, 0x72, 0x6e, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x6e, 0x22, 0x50, 0x0a, 0x11, 0x47, 0x65, 0x74,
	0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x3b,
	0x0a, 0x06, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x23,
	0x2e, 0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74,
	0x72, 0x6f, 0x70, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x4d, 0x6f, 0x64,
	0x75, 0x6c, 0x65, 0x52, 0x06, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x22, 0x52, 0x0a, 0x13, 0x43,
	0x72, 0x65, 0x61, 0x74, 0x65, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x3b, 0x0a, 0x06, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x23, 0x2e, 0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79,
	0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2e, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52, 0x06, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x22,
	0x53, 0x0a, 0x14, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x3b, 0x0a, 0x06, 0x6d, 0x6f, 0x64, 0x75, 0x6c,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x23, 0x2e, 0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f,
	0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x2e, 0x76, 0x31,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52, 0x06, 0x6d, 0x6f,
	0x64, 0x75, 0x6c, 0x65, 0x22, 0x5f, 0x0a, 0x13, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4d, 0x6f,
	0x64, 0x75, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x75,
	0x72, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x6e, 0x12, 0x30, 0x0a,
	0x07, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x07, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x73, 0x4a,
	0x04, 0x08, 0x02, 0x10, 0x03, 0x22, 0x53, 0x0a, 0x14, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4d,
	0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x3b, 0x0a,
	0x06, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x23, 0x2e,
	0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74, 0x72,
	0x6f, 0x70, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x4d, 0x6f, 0x64, 0x75,
	0x6c, 0x65, 0x52, 0x06, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x22, 0x27, 0x0a, 0x13, 0x44, 0x65,
	0x6c, 0x65, 0x74, 0x65, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x10, 0x0a, 0x03, 0x75, 0x72, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03,
	0x75, 0x72, 0x6e, 0x22, 0x16, 0x0a, 0x14, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4d, 0x6f, 0x64,
	0x75, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0xf0, 0x05, 0x0a, 0x0d,
	0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x8a, 0x01,
	0x0a, 0x0b, 0x4c, 0x69, 0x73, 0x74, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x73, 0x12, 0x2f, 0x2e,
	0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74, 0x72,
	0x6f, 0x70, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74,
	0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x30,
	0x2e, 0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74,
	0x72, 0x6f, 0x70, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x4c, 0x69, 0x73,
	0x74, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x22, 0x18, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x12, 0x12, 0x10, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74,
	0x61, 0x31, 0x2f, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x73, 0x12, 0x8a, 0x01, 0x0a, 0x09, 0x47,
	0x65, 0x74, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x12, 0x2d, 0x2e, 0x67, 0x6f, 0x74, 0x6f, 0x63,
	0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x2e, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x2e, 0x2e, 0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f,
	0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x2e, 0x76, 0x31,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x1e, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x18, 0x12,
	0x16, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65,
	0x73, 0x2f, 0x7b, 0x75, 0x72, 0x6e, 0x7d, 0x12, 0x95, 0x01, 0x0a, 0x0c, 0x43, 0x72, 0x65, 0x61,
	0x74, 0x65, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x12, 0x30, 0x2e, 0x67, 0x6f, 0x74, 0x6f, 0x63,
	0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x2e, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4d, 0x6f, 0x64,
	0x75, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x31, 0x2e, 0x67, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79,
	0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4d,
	0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x20, 0x82,
	0xd3, 0xe4, 0x93, 0x02, 0x1a, 0x3a, 0x06, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x22, 0x10, 0x2f,
	0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x73, 0x12,
	0x96, 0x01, 0x0a, 0x0c, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65,
	0x12, 0x30, 0x2e, 0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65,
	0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x55,
	0x70, 0x64, 0x61, 0x74, 0x65, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x31, 0x2e, 0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79,
	0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x21, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x1b, 0x3a, 0x01, 0x2a,
	0x32, 0x16, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x6d, 0x6f, 0x64, 0x75, 0x6c,
	0x65, 0x73, 0x2f, 0x7b, 0x75, 0x72, 0x6e, 0x7d, 0x12, 0x93, 0x01, 0x0a, 0x0c, 0x44, 0x65, 0x6c,
	0x65, 0x74, 0x65, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x12, 0x30, 0x2e, 0x67, 0x6f, 0x74, 0x6f,
	0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x2e,
	0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4d, 0x6f,
	0x64, 0x75, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x31, 0x2e, 0x67, 0x6f,
	0x74, 0x6f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e, 0x79, 0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70,
	0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65,
	0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x1e,
	0x82, 0xd3, 0xe4, 0x93, 0x02, 0x18, 0x2a, 0x16, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2f, 0x6d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x73, 0x2f, 0x7b, 0x75, 0x72, 0x6e, 0x7d, 0x42, 0x75,
	0x0a, 0x26, 0x63, 0x6f, 0x6d, 0x2e, 0x67, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x6e,
	0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x6e, 0x2e, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79,
	0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x42, 0x12, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65,
	0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x35,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x67, 0x6f, 0x74, 0x6f, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x6e, 0x2f, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x2f, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x3b, 0x65, 0x6e, 0x74, 0x72, 0x6f, 0x70, 0x79, 0x76, 0x31,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_gotocompany_entropy_v1beta1_module_proto_rawDescOnce sync.Once
	file_gotocompany_entropy_v1beta1_module_proto_rawDescData = file_gotocompany_entropy_v1beta1_module_proto_rawDesc
)

func file_gotocompany_entropy_v1beta1_module_proto_rawDescGZIP() []byte {
	file_gotocompany_entropy_v1beta1_module_proto_rawDescOnce.Do(func() {
		file_gotocompany_entropy_v1beta1_module_proto_rawDescData = protoimpl.X.CompressGZIP(file_gotocompany_entropy_v1beta1_module_proto_rawDescData)
	})
	return file_gotocompany_entropy_v1beta1_module_proto_rawDescData
}

var file_gotocompany_entropy_v1beta1_module_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_gotocompany_entropy_v1beta1_module_proto_goTypes = []interface{}{
	(*Module)(nil),                // 0: gotocompany.entropy.v1beta1.Module
	(*ListModulesRequest)(nil),    // 1: gotocompany.entropy.v1beta1.ListModulesRequest
	(*ListModulesResponse)(nil),   // 2: gotocompany.entropy.v1beta1.ListModulesResponse
	(*GetModuleRequest)(nil),      // 3: gotocompany.entropy.v1beta1.GetModuleRequest
	(*GetModuleResponse)(nil),     // 4: gotocompany.entropy.v1beta1.GetModuleResponse
	(*CreateModuleRequest)(nil),   // 5: gotocompany.entropy.v1beta1.CreateModuleRequest
	(*CreateModuleResponse)(nil),  // 6: gotocompany.entropy.v1beta1.CreateModuleResponse
	(*UpdateModuleRequest)(nil),   // 7: gotocompany.entropy.v1beta1.UpdateModuleRequest
	(*UpdateModuleResponse)(nil),  // 8: gotocompany.entropy.v1beta1.UpdateModuleResponse
	(*DeleteModuleRequest)(nil),   // 9: gotocompany.entropy.v1beta1.DeleteModuleRequest
	(*DeleteModuleResponse)(nil),  // 10: gotocompany.entropy.v1beta1.DeleteModuleResponse
	(*timestamppb.Timestamp)(nil), // 11: google.protobuf.Timestamp
	(*structpb.Value)(nil),        // 12: google.protobuf.Value
}
var file_gotocompany_entropy_v1beta1_module_proto_depIdxs = []int32{
	11, // 0: gotocompany.entropy.v1beta1.Module.created_at:type_name -> google.protobuf.Timestamp
	11, // 1: gotocompany.entropy.v1beta1.Module.updated_at:type_name -> google.protobuf.Timestamp
	12, // 2: gotocompany.entropy.v1beta1.Module.configs:type_name -> google.protobuf.Value
	0,  // 3: gotocompany.entropy.v1beta1.ListModulesResponse.modules:type_name -> gotocompany.entropy.v1beta1.Module
	0,  // 4: gotocompany.entropy.v1beta1.GetModuleResponse.module:type_name -> gotocompany.entropy.v1beta1.Module
	0,  // 5: gotocompany.entropy.v1beta1.CreateModuleRequest.module:type_name -> gotocompany.entropy.v1beta1.Module
	0,  // 6: gotocompany.entropy.v1beta1.CreateModuleResponse.module:type_name -> gotocompany.entropy.v1beta1.Module
	12, // 7: gotocompany.entropy.v1beta1.UpdateModuleRequest.configs:type_name -> google.protobuf.Value
	0,  // 8: gotocompany.entropy.v1beta1.UpdateModuleResponse.module:type_name -> gotocompany.entropy.v1beta1.Module
	1,  // 9: gotocompany.entropy.v1beta1.ModuleService.ListModules:input_type -> gotocompany.entropy.v1beta1.ListModulesRequest
	3,  // 10: gotocompany.entropy.v1beta1.ModuleService.GetModule:input_type -> gotocompany.entropy.v1beta1.GetModuleRequest
	5,  // 11: gotocompany.entropy.v1beta1.ModuleService.CreateModule:input_type -> gotocompany.entropy.v1beta1.CreateModuleRequest
	7,  // 12: gotocompany.entropy.v1beta1.ModuleService.UpdateModule:input_type -> gotocompany.entropy.v1beta1.UpdateModuleRequest
	9,  // 13: gotocompany.entropy.v1beta1.ModuleService.DeleteModule:input_type -> gotocompany.entropy.v1beta1.DeleteModuleRequest
	2,  // 14: gotocompany.entropy.v1beta1.ModuleService.ListModules:output_type -> gotocompany.entropy.v1beta1.ListModulesResponse
	4,  // 15: gotocompany.entropy.v1beta1.ModuleService.GetModule:output_type -> gotocompany.entropy.v1beta1.GetModuleResponse
	6,  // 16: gotocompany.entropy.v1beta1.ModuleService.CreateModule:output_type -> gotocompany.entropy.v1beta1.CreateModuleResponse
	8,  // 17: gotocompany.entropy.v1beta1.ModuleService.UpdateModule:output_type -> gotocompany.entropy.v1beta1.UpdateModuleResponse
	10, // 18: gotocompany.entropy.v1beta1.ModuleService.DeleteModule:output_type -> gotocompany.entropy.v1beta1.DeleteModuleResponse
	14, // [14:19] is the sub-list for method output_type
	9,  // [9:14] is the sub-list for method input_type
	9,  // [9:9] is the sub-list for extension type_name
	9,  // [9:9] is the sub-list for extension extendee
	0,  // [0:9] is the sub-list for field type_name
}

func init() { file_gotocompany_entropy_v1beta1_module_proto_init() }
func file_gotocompany_entropy_v1beta1_module_proto_init() {
	if File_gotocompany_entropy_v1beta1_module_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_gotocompany_entropy_v1beta1_module_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Module); i {
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
		file_gotocompany_entropy_v1beta1_module_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListModulesRequest); i {
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
		file_gotocompany_entropy_v1beta1_module_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListModulesResponse); i {
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
		file_gotocompany_entropy_v1beta1_module_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetModuleRequest); i {
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
		file_gotocompany_entropy_v1beta1_module_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetModuleResponse); i {
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
		file_gotocompany_entropy_v1beta1_module_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateModuleRequest); i {
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
		file_gotocompany_entropy_v1beta1_module_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateModuleResponse); i {
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
		file_gotocompany_entropy_v1beta1_module_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateModuleRequest); i {
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
		file_gotocompany_entropy_v1beta1_module_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateModuleResponse); i {
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
		file_gotocompany_entropy_v1beta1_module_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteModuleRequest); i {
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
		file_gotocompany_entropy_v1beta1_module_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteModuleResponse); i {
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
			RawDescriptor: file_gotocompany_entropy_v1beta1_module_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_gotocompany_entropy_v1beta1_module_proto_goTypes,
		DependencyIndexes: file_gotocompany_entropy_v1beta1_module_proto_depIdxs,
		MessageInfos:      file_gotocompany_entropy_v1beta1_module_proto_msgTypes,
	}.Build()
	File_gotocompany_entropy_v1beta1_module_proto = out.File
	file_gotocompany_entropy_v1beta1_module_proto_rawDesc = nil
	file_gotocompany_entropy_v1beta1_module_proto_goTypes = nil
	file_gotocompany_entropy_v1beta1_module_proto_depIdxs = nil
}