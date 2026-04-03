"""gRPC server implementing the RiskGate.CheckPreTradeRisk RPC.

Performs four pre-trade risk checks in sequence, targeting <10 ms total
latency by using ParametricVaR (analytical, no simulation).
"""

from __future__ import annotations

import copy
import time
from concurrent import futures
from datetime import datetime, timezone
from decimal import Decimal

import grpc
import structlog

from risk_engine.domain.portfolio import Portfolio
from risk_engine.domain.position import Position
from risk_engine.metrics import pretrade_check_rejected_total, pretrade_check_seconds
from risk_engine.var.parametric import ParametricVaR

logger = structlog.get_logger()

# ---------------------------------------------------------------------------
# Pre-trade risk check limits (configurable at init or via env/config)
# ---------------------------------------------------------------------------
MAX_POSITION_CONCENTRATION: float = 0.25  # 25% of NAV
MAX_ORDER_NOTIONAL: Decimal = Decimal("100000")  # $100K max per order
MAX_VAR_INCREASE_PCT: float = 0.10  # reject if VaR jumps >10% from one order


class RiskGateServicer:
    """gRPC servicer for pre-trade risk checks.

    Each call to ``CheckPreTradeRisk`` runs four checks:
    1. Order-size limit (notional cap).
    2. Position concentration (single-name % of NAV).
    3. Available cash (buy-side only).
    4. VaR impact (before/after projected fill).

    All checks are synchronous and designed for <10 ms total.
    """

    def __init__(
        self,
        portfolio: Portfolio,
        var_engine: ParametricVaR,
        returns_matrix=None,
        *,
        max_order_notional: Decimal = MAX_ORDER_NOTIONAL,
        max_position_concentration: float = MAX_POSITION_CONCENTRATION,
        max_var_increase_pct: float = MAX_VAR_INCREASE_PCT,
    ) -> None:
        self.portfolio = portfolio
        self.var_engine = var_engine
        self.returns_matrix = returns_matrix  # pd.DataFrame — set when data arrives
        self.max_order_notional = max_order_notional
        self.max_position_concentration = max_position_concentration
        self.max_var_increase_pct = max_var_increase_pct

    # ------------------------------------------------------------------
    # gRPC handler
    # ------------------------------------------------------------------

    def CheckPreTradeRisk(self, request, context):  # noqa: N802 — matches proto
        """Perform pre-trade risk checks on an incoming order."""
        start_time = time.monotonic()

        # Extract correlation ID from gRPC metadata for structured logging
        metadata = dict(context.invocation_metadata())
        correlation_id = metadata.get("x-correlation-id", "")
        log = logger.bind(correlation_id=correlation_id, order_id=request.order_id)

        checks: list[dict] = []
        approved = True
        reject_reason = ""

        quantity = Decimal(request.quantity)
        price = Decimal(request.price) if request.price else Decimal("0")
        # For market orders (price=0) fall back to last market price if position exists
        effective_price = price
        if effective_price == 0 and request.instrument_id in self.portfolio.positions:
            effective_price = self.portfolio.positions[request.instrument_id].market_price
        notional = quantity * effective_price if effective_price > 0 else quantity

        # Check 1: Order size limit
        size_check = self._check_order_size(notional)
        checks.append(size_check)

        # Check 2: Position concentration
        concentration_check = self._check_concentration(request.instrument_id, notional)
        checks.append(concentration_check)

        # Check 3: Available cash (buy orders only)
        if request.side.upper() == "BUY":
            cash_check = self._check_available_cash(notional)
            checks.append(cash_check)

        # Check 4: VaR impact (only if returns data is loaded)
        var_before = "0"
        var_after = "0"
        if self.returns_matrix is not None:
            var_before, var_after, var_check = self._check_var_impact(
                request.instrument_id,
                request.side,
                quantity,
                effective_price,
                request.asset_class,
            )
            checks.append(var_check)

        # Determine overall approval — first failing check wins
        for check in checks:
            if not check["passed"]:
                approved = False
                reject_reason = check["message"]
                break

        elapsed_ms = int((time.monotonic() - start_time) * 1000)
        pretrade_check_seconds.observe(time.monotonic() - start_time)
        if not approved:
            pretrade_check_rejected_total.inc()
        log.info("risk_check_completed", approved=approved, elapsed_ms=elapsed_ms)

        # Build protobuf response (fall back gracefully if stubs not generated)
        try:
            from risk_engine.proto.risk import risk_pb2  # type: ignore[import-untyped]

            response = risk_pb2.PreTradeRiskResponse(
                approved=approved,
                reject_reason=reject_reason,
                portfolio_var_before=str(var_before),
                portfolio_var_after=str(var_after),
                computed_at_ms=int(datetime.now(timezone.utc).timestamp() * 1000),
                order_id=request.order_id,
            )
            for c in checks:
                response.checks.append(
                    risk_pb2.RiskCheck(
                        name=c["name"],
                        passed=c["passed"],
                        message=c["message"],
                        threshold=c.get("threshold", ""),
                        actual=c.get("actual", ""),
                    )
                )
            return response
        except ImportError:
            # Proto stubs not compiled yet — return a plain object for testing
            log.warning("proto_stubs_unavailable", msg="returning dict-like response")
            return {
                "approved": approved,
                "reject_reason": reject_reason,
                "checks": checks,
                "portfolio_var_before": str(var_before),
                "portfolio_var_after": str(var_after),
                "computed_at_ms": int(datetime.now(timezone.utc).timestamp() * 1000),
                "order_id": request.order_id if hasattr(request, "order_id") else "",
            }

    # ------------------------------------------------------------------
    # Risk check implementations
    # ------------------------------------------------------------------

    def _check_order_size(self, notional: Decimal) -> dict:
        """Check 1: Reject if order notional exceeds the per-order cap."""
        passed = notional <= self.max_order_notional
        return {
            "name": "order_size_limit",
            "passed": passed,
            "message": (
                ""
                if passed
                else (
                    f"Order notional {notional} exceeds max "
                    f"{self.max_order_notional}"
                )
            ),
            "threshold": str(self.max_order_notional),
            "actual": str(notional),
        }

    def _check_concentration(self, instrument_id: str, notional: Decimal) -> dict:
        """Check 2: Reject if post-trade position would exceed concentration limit.

        Concentration = (existing_position_value + new_notional) / NAV.
        """
        nav = self.portfolio.nav if self.portfolio.nav > 0 else self.portfolio.cash
        if nav <= 0:
            # Cannot compute concentration without a positive NAV
            return {
                "name": "position_concentration",
                "passed": True,
                "message": "NAV is zero; concentration check skipped",
                "threshold": str(self.max_position_concentration),
                "actual": "0",
            }

        existing_value = Decimal("0")
        if instrument_id in self.portfolio.positions:
            existing_value = self.portfolio.positions[instrument_id].market_value

        projected_value = existing_value + notional
        concentration = float(projected_value / nav)
        passed = concentration <= self.max_position_concentration

        return {
            "name": "position_concentration",
            "passed": passed,
            "message": (
                ""
                if passed
                else (
                    f"Projected concentration {concentration:.2%} for "
                    f"{instrument_id} exceeds limit "
                    f"{self.max_position_concentration:.0%}"
                )
            ),
            "threshold": str(self.max_position_concentration),
            "actual": f"{concentration:.6f}",
        }

    def _check_available_cash(self, notional: Decimal) -> dict:
        """Check 3: Reject buy if notional exceeds available (uncommitted) cash."""
        passed = notional <= self.portfolio.available_cash
        return {
            "name": "available_cash",
            "passed": passed,
            "message": (
                ""
                if passed
                else (
                    f"Insufficient cash: need {notional}, "
                    f"available {self.portfolio.available_cash}"
                )
            ),
            "threshold": str(self.portfolio.available_cash),
            "actual": str(notional),
        }

    def _check_var_impact(
        self,
        instrument_id: str,
        side: str,
        quantity: Decimal,
        price: Decimal,
        asset_class: str,
    ) -> tuple[str, str, dict]:
        """Check 4: Compute VaR before and after a hypothetical fill.

        Returns ``(var_before, var_after, check_dict)``.

        Uses ParametricVaR for sub-millisecond computation.  The "after"
        portfolio is built by shallow-copying the position dict and
        applying the hypothetical trade without mutating real state.
        """
        # --- VaR BEFORE (current portfolio) ---
        var_before_result = self.var_engine.compute(
            self.portfolio.positions, self.returns_matrix
        )
        var_before = var_before_result.var_amount

        # --- Build hypothetical post-trade positions ---
        hypo_positions: dict[str, Position] = {
            iid: copy.copy(pos) for iid, pos in self.portfolio.positions.items()
        }

        side_upper = side.upper()
        if instrument_id in hypo_positions:
            pos = hypo_positions[instrument_id]
            if side_upper == "BUY":
                new_qty = pos.quantity + quantity
                if new_qty != Decimal("0"):
                    pos.average_cost = (
                        (pos.average_cost * pos.quantity) + (quantity * price)
                    ) / new_qty
                pos.quantity = new_qty
                pos.market_price = price
                pos.unrealized_pnl = (price - pos.average_cost) * new_qty
            elif side_upper == "SELL":
                new_qty = pos.quantity - quantity
                if new_qty <= 0:
                    del hypo_positions[instrument_id]
                else:
                    pos.quantity = new_qty
                    pos.market_price = price
                    pos.unrealized_pnl = (price - pos.average_cost) * new_qty
        else:
            if side_upper == "BUY":
                hypo_positions[instrument_id] = Position(
                    instrument_id=instrument_id,
                    venue_id="",
                    quantity=quantity,
                    average_cost=price,
                    market_price=price,
                    unrealized_pnl=Decimal("0"),
                    realized_pnl=Decimal("0"),
                    asset_class=asset_class,
                    settlement_cycle="T0" if asset_class == "crypto" else "T2",
                )
            # SELL on a non-existent position: VaR stays the same (would be
            # rejected by other checks anyway).

        # --- VaR AFTER (hypothetical portfolio) ---
        var_after_result = self.var_engine.compute(
            hypo_positions, self.returns_matrix
        )
        var_after = var_after_result.var_amount

        # Determine if the VaR increase is acceptable
        if var_before > 0:
            var_increase_pct = float((var_after - var_before) / var_before)
        else:
            var_increase_pct = 0.0 if var_after == 0 else 1.0

        passed = var_increase_pct <= self.max_var_increase_pct

        return (
            str(var_before),
            str(var_after),
            {
                "name": "var_impact",
                "passed": passed,
                "message": (
                    ""
                    if passed
                    else (
                        f"VaR increase {var_increase_pct:.2%} exceeds limit "
                        f"{self.max_var_increase_pct:.0%} "
                        f"(before={var_before}, after={var_after})"
                    )
                ),
                "threshold": f"{self.max_var_increase_pct:.6f}",
                "actual": f"{var_increase_pct:.6f}",
            },
        )


# ---------------------------------------------------------------------------
# Server lifecycle
# ---------------------------------------------------------------------------


def serve(
    portfolio: Portfolio,
    var_engine: ParametricVaR,
    port: int = 50051,
    returns_matrix=None,
    max_workers: int = 10,
) -> grpc.Server:
    """Create, configure, and start the gRPC server.

    Returns the ``grpc.Server`` instance so the caller can call
    ``server.wait_for_termination()`` or ``server.stop(grace)``.
    """
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers))
    servicer = RiskGateServicer(portfolio, var_engine, returns_matrix)

    # Register the servicer with the generated gRPC stubs
    try:
        from risk_engine.proto.risk import risk_pb2_grpc  # type: ignore[import-untyped]

        risk_pb2_grpc.add_RiskGateServicer_to_server(servicer, server)
    except ImportError:
        logger.warning(
            "proto_stubs_unavailable",
            msg="gRPC service registration skipped — compile proto stubs first",
        )
        return server

    server.add_insecure_port(f"[::]:{port}")
    server.start()
    logger.info("grpc_server_started", port=port)
    return server
