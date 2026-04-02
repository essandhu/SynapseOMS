"""gRPC server for pre-trade risk checks (RiskGate service)."""

from risk_engine.grpc_server.server import RiskGateServicer, serve

__all__ = ["RiskGateServicer", "serve"]
