// Package grpc provides gRPC client wrappers for inter-service communication.
package grpc

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/logging"
	pb "github.com/synapse-oms/gateway/internal/proto/riskpb"
)

// RiskCheckResult contains the outcome of a pre-trade risk check.
type RiskCheckResult struct {
	Approved     bool
	RejectReason string
	VaRBefore    string
	VaRAfter     string
}

// RiskClient is the interface for pre-trade risk checking.
// Implementations may call a remote gRPC service or return a static result.
type RiskClient interface {
	CheckPreTradeRisk(ctx context.Context, order *domain.Order) (*RiskCheckResult, error)
	Close() error
}

// grpcRiskClient connects to the Risk Engine over gRPC.
type grpcRiskClient struct {
	address string
	logger  *slog.Logger
	conn    *grpc.ClientConn
	client  pb.RiskGateClient
}

// NewRiskClient creates a RiskClient that connects to the Risk Engine gRPC
// server at the given address (e.g. "risk-engine:50051").
func NewRiskClient(address string) (RiskClient, error) {
	logger := logging.NewDefault("gateway", "risk-client")
	logger.Info("connecting to risk engine gRPC server",
		slog.String("address", address),
	)

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(pb.NewCodec())),
	)
	if err != nil {
		return nil, err
	}

	return &grpcRiskClient{
		address: address,
		logger:  logger,
		conn:    conn,
		client:  pb.NewRiskGateClient(conn),
	}, nil
}

// CheckPreTradeRisk calls the Risk Engine's CheckPreTradeRisk RPC.
// On transport errors, it returns the error (the pipeline decides fail-open/closed).
func (c *grpcRiskClient) CheckPreTradeRisk(ctx context.Context, order *domain.Order) (*RiskCheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	req := &pb.PreTradeRiskRequest{
		OrderId:      string(order.ID),
		InstrumentId: order.InstrumentID,
		Side:         sideString(order.Side),
		Quantity:     order.Quantity.String(),
		Price:        order.Price.String(),
		AssetClass:   assetClassString(order.AssetClass),
		VenueId:      order.VenueID,
	}

	resp, err := c.client.CheckPreTradeRisk(ctx, req)
	if err != nil {
		c.logger.Warn("risk engine gRPC call failed",
			slog.String("order_id", string(order.ID)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	c.logger.Debug("risk check completed",
		slog.String("order_id", string(order.ID)),
		slog.Bool("approved", resp.Approved),
	)

	return &RiskCheckResult{
		Approved:     resp.Approved,
		RejectReason: resp.RejectReason,
		VaRBefore:    resp.PortfolioVarBefore,
		VaRAfter:     resp.PortfolioVarAfter,
	}, nil
}

// Close shuts down the gRPC connection.
func (c *grpcRiskClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// FailOpenRiskClient is a RiskClient that always approves orders.
// Used when no risk engine address is configured or for paper trading.
type FailOpenRiskClient struct {
	logger *slog.Logger
}

// NewFailOpenRiskClient creates a RiskClient that approves all orders.
func NewFailOpenRiskClient() RiskClient {
	return &FailOpenRiskClient{
		logger: logging.NewDefault("gateway", "risk-client-failopen"),
	}
}

// CheckPreTradeRisk always returns approved=true.
func (c *FailOpenRiskClient) CheckPreTradeRisk(_ context.Context, _ *domain.Order) (*RiskCheckResult, error) {
	return &RiskCheckResult{Approved: true}, nil
}

// Close is a no-op for the fail-open client.
func (c *FailOpenRiskClient) Close() error {
	return nil
}

func sideString(s domain.OrderSide) string {
	switch s {
	case domain.SideBuy:
		return "BUY"
	case domain.SideSell:
		return "SELL"
	default:
		return "UNKNOWN"
	}
}

func assetClassString(ac domain.AssetClass) string {
	switch ac {
	case domain.AssetClassEquity:
		return "equity"
	case domain.AssetClassCrypto:
		return "crypto"
	default:
		return "other"
	}
}
