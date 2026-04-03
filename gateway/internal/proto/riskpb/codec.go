package riskpb

import "fmt"

// protoMarshaler is implemented by hand-written proto message types.
type protoMarshaler interface {
	MarshalProto() ([]byte, error)
	UnmarshalProto([]byte) error
}

// Codec is a gRPC codec that uses manual protobuf wire encoding for riskpb
// types. It satisfies grpc/encoding.Codec.
type Codec struct{}

// NewCodec creates a riskpb-aware gRPC codec.
func NewCodec() *Codec {
	return &Codec{}
}

func (c *Codec) Marshal(v interface{}) ([]byte, error) {
	if m, ok := v.(protoMarshaler); ok {
		return m.MarshalProto()
	}
	return nil, fmt.Errorf("riskpb.Codec: cannot marshal %T", v)
}

func (c *Codec) Unmarshal(data []byte, v interface{}) error {
	if m, ok := v.(protoMarshaler); ok {
		return m.UnmarshalProto(data)
	}
	return fmt.Errorf("riskpb.Codec: cannot unmarshal into %T", v)
}

func (c *Codec) Name() string {
	return "proto"
}
