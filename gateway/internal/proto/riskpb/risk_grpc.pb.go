package riskpb

import (
	"context"

	"google.golang.org/grpc"
)

const riskGateServiceName = "/synapse.risk.RiskGate/CheckPreTradeRisk"

// RiskGateClient is the client API for the RiskGate service.
type RiskGateClient interface {
	CheckPreTradeRisk(ctx context.Context, in *PreTradeRiskRequest, opts ...grpc.CallOption) (*PreTradeRiskResponse, error)
}

type riskGateClient struct {
	cc grpc.ClientConnInterface
}

// NewRiskGateClient creates a new RiskGate gRPC client.
func NewRiskGateClient(cc grpc.ClientConnInterface) RiskGateClient {
	return &riskGateClient{cc}
}

func (c *riskGateClient) CheckPreTradeRisk(ctx context.Context, in *PreTradeRiskRequest, opts ...grpc.CallOption) (*PreTradeRiskResponse, error) {
	out := new(PreTradeRiskResponse)
	err := c.cc.Invoke(ctx, riskGateServiceName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}
