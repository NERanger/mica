package mica

import "google.golang.org/protobuf/proto"

type RPCMethod struct {
	Service     string
	Method      string
	NewRequest  func() proto.Message
	NewResponse func() proto.Message
}

func (m RPCMethod) ContractID() string {
	return m.Service + "." + m.Method
}
