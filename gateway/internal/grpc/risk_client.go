// Package grpc provides gRPC client wrappers for inter-service communication.
package grpc

import (
	"context"
	"log/slog"

	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/logging"
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
// Until proto stubs are generated, it operates in fail-open mode.
type grpcRiskClient struct {
	address string
	logger  *slog.Logger
	// conn   *grpc.ClientConn        // Uncomment when grpc dependency is added
	// client pb.RiskGateClient       // Uncomment when proto stubs are generated
}

// NewRiskClient creates a RiskClient that connects to the Risk Engine at the
// given address. Since proto stubs and the grpc dependency are not yet wired,
// this currently returns a fail-open client that approves all orders.
func NewRiskClient(address string) (RiskClient, error) {
	logger := logging.NewDefault("gateway", "risk-client")
	logger.Info("creating risk client (fail-open until proto stubs generated)",
		slog.String("address", address),
	)

	// TODO: once google.golang.org/grpc is added to go.mod and proto stubs
	// are generated, establish a real connection:
	//
	//   conn, err := grpc.NewClient(address,
	//       grpc.WithTransportCredentials(insecure.NewCredentials()),
	//   )
	//   if err != nil {
	//       return nil, err
	//   }
	//   return &grpcRiskClient{address: address, conn: conn, logger: logger}, nil

	return &grpcRiskClient{address: address, logger: logger}, nil
}

// CheckPreTradeRisk calls the Risk Engine's CheckPreTradeRisk RPC.
// Currently operates in fail-open mode: all orders are approved.
func (c *grpcRiskClient) CheckPreTradeRisk(_ context.Context, order *domain.Order) (*RiskCheckResult, error) {
	// TODO: implement real gRPC call once proto stubs are generated:
	//
	//   resp, err := c.client.CheckPreTradeRisk(ctx, &pb.CheckPreTradeRiskRequest{
	//       OrderId:      string(order.ID),
	//       InstrumentId: order.InstrumentID,
	//       Side:         order.Side.String(),
	//       Quantity:     order.Quantity.String(),
	//       Price:        order.Price.String(),
	//   })
	//   if err != nil {
	//       // fail-open: log error, return approved
	//       c.logger.Warn("risk engine unavailable, failing open",
	//           slog.String("order_id", string(order.ID)),
	//           slog.String("error", err.Error()),
	//       )
	//       return &RiskCheckResult{Approved: true}, nil
	//   }

	c.logger.Debug("risk check (fail-open): approved",
		slog.String("order_id", string(order.ID)),
		slog.String("instrument", order.InstrumentID),
	)

	return &RiskCheckResult{
		Approved: true,
	}, nil
}

// Close shuts down the gRPC connection.
func (c *grpcRiskClient) Close() error {
	// TODO: close the real gRPC connection once wired
	// if c.conn != nil {
	//     return c.conn.Close()
	// }
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
