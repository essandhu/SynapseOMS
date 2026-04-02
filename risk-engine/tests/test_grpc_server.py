"""Tests for gRPC RiskGateServicer pre-trade risk check logic.

Tests the four risk checks (order size, concentration, cash, VaR impact)
by calling the servicer directly with mock request/context objects,
bypassing actual gRPC transport.
"""

from __future__ import annotations

from decimal import Decimal
from types import SimpleNamespace
from unittest.mock import MagicMock

import pandas as pd
import pytest

from risk_engine.domain.portfolio import Portfolio
from risk_engine.domain.position import Position
from risk_engine.domain.risk_result import VaRResult
from risk_engine.grpc_server.server import RiskGateServicer
from risk_engine.var.parametric import ParametricVaR


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_request(
    order_id: str = "ORD-001",
    instrument_id: str = "AAPL",
    side: str = "BUY",
    quantity: str = "10",
    price: str = "150.00",
    asset_class: str = "equity",
) -> SimpleNamespace:
    """Build a lightweight request object matching the proto interface."""
    return SimpleNamespace(
        order_id=order_id,
        instrument_id=instrument_id,
        side=side,
        quantity=quantity,
        price=price,
        asset_class=asset_class,
    )


def _make_context() -> MagicMock:
    """Build a mock gRPC context with empty metadata."""
    ctx = MagicMock()
    ctx.invocation_metadata.return_value = []
    return ctx


def _normalize_response(response) -> dict:
    """Normalize a gRPC response (protobuf or dict) to a plain dict.

    Handles both the protobuf PreTradeRiskResponse (when proto stubs are
    compiled) and the dict fallback used when stubs are unavailable.
    """
    if isinstance(response, dict):
        return response

    # Protobuf response -- extract fields via attribute access
    checks = []
    for c in response.checks:
        checks.append({
            "name": c.name,
            "passed": c.passed,
            "message": c.message,
            "threshold": c.threshold,
            "actual": c.actual,
        })
    return {
        "approved": response.approved,
        "reject_reason": response.reject_reason,
        "checks": checks,
        "portfolio_var_before": response.portfolio_var_before,
        "portfolio_var_after": response.portfolio_var_after,
        "order_id": response.order_id,
    }


def _make_servicer(
    portfolio: Portfolio | None = None,
    var_engine: ParametricVaR | None = None,
    returns_matrix: pd.DataFrame | None = None,
    **kwargs,
) -> RiskGateServicer:
    """Create a RiskGateServicer with sensible defaults."""
    if portfolio is None:
        portfolio = Portfolio()
    if var_engine is None:
        var_engine = ParametricVaR()
    return RiskGateServicer(
        portfolio=portfolio,
        var_engine=var_engine,
        returns_matrix=returns_matrix,
        **kwargs,
    )


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestCheckPreTradeRisk:
    """Test suite for the pre-trade risk check logic."""

    def test_approve_small_order(self, sample_portfolio: Portfolio) -> None:
        """A small order that passes all checks should be approved."""
        servicer = _make_servicer(portfolio=sample_portfolio)
        request = _make_request(
            instrument_id="AAPL",
            side="BUY",
            quantity="5",
            price="180.00",
        )
        response = _normalize_response(
            servicer.CheckPreTradeRisk(request, _make_context())
        )

        assert response["approved"] is True
        assert response["reject_reason"] == ""
        # All checks should have passed
        for check in response["checks"]:
            assert check["passed"] is True

    def test_reject_order_exceeding_max_notional(
        self, sample_portfolio: Portfolio
    ) -> None:
        """Order with notional > max_order_notional should be rejected."""
        servicer = _make_servicer(
            portfolio=sample_portfolio,
            max_order_notional=Decimal("10000"),
        )
        # 100 shares * $180 = $18,000 > $10,000 limit
        request = _make_request(
            instrument_id="AAPL",
            side="BUY",
            quantity="100",
            price="180.00",
        )
        response = _normalize_response(
            servicer.CheckPreTradeRisk(request, _make_context())
        )

        assert response["approved"] is False
        assert "exceeds max" in response["reject_reason"]
        size_check = next(
            c for c in response["checks"] if c["name"] == "order_size_limit"
        )
        assert size_check["passed"] is False

    def test_reject_concentration_limit(self, sample_portfolio: Portfolio) -> None:
        """Order that pushes position concentration above limit should be rejected."""
        servicer = _make_servicer(
            portfolio=sample_portfolio,
            max_position_concentration=0.10,  # 10% to make it easier to exceed
        )
        # Large order that would exceed 10% concentration
        request = _make_request(
            instrument_id="AAPL",
            side="BUY",
            quantity="200",
            price="180.00",
        )
        response = _normalize_response(
            servicer.CheckPreTradeRisk(request, _make_context())
        )

        assert response["approved"] is False
        concentration_check = next(
            c for c in response["checks"] if c["name"] == "position_concentration"
        )
        assert concentration_check["passed"] is False
        assert "exceeds limit" in concentration_check["message"]

    def test_reject_buy_insufficient_cash(self) -> None:
        """Buy order should be rejected when notional exceeds available cash."""
        portfolio = Portfolio(
            cash=Decimal("1000"),
            available_cash=Decimal("1000"),
        )
        portfolio.compute_nav()
        servicer = _make_servicer(portfolio=portfolio)

        # 100 * $150 = $15,000 > $1,000 available
        request = _make_request(
            instrument_id="AAPL",
            side="BUY",
            quantity="100",
            price="150.00",
        )
        response = _normalize_response(
            servicer.CheckPreTradeRisk(request, _make_context())
        )

        assert response["approved"] is False
        cash_check = next(
            c for c in response["checks"] if c["name"] == "available_cash"
        )
        assert cash_check["passed"] is False
        assert "Insufficient cash" in cash_check["message"]

    def test_sell_order_skips_cash_check(self, sample_portfolio: Portfolio) -> None:
        """Sell orders should not perform the available_cash check."""
        servicer = _make_servicer(portfolio=sample_portfolio)
        request = _make_request(
            instrument_id="AAPL",
            side="SELL",
            quantity="5",
            price="180.00",
        )
        response = _normalize_response(
            servicer.CheckPreTradeRisk(request, _make_context())
        )

        check_names = [c["name"] for c in response["checks"]]
        assert "available_cash" not in check_names

    def test_var_impact_check_with_returns_data(
        self,
        sample_portfolio: Portfolio,
        sample_returns_matrix: pd.DataFrame,
    ) -> None:
        """VaR impact check should run when returns_matrix is provided."""
        servicer = _make_servicer(
            portfolio=sample_portfolio,
            returns_matrix=sample_returns_matrix,
        )
        request = _make_request(
            instrument_id="AAPL",
            side="BUY",
            quantity="5",
            price="180.00",
        )
        response = _normalize_response(
            servicer.CheckPreTradeRisk(request, _make_context())
        )

        check_names = [c["name"] for c in response["checks"]]
        assert "var_impact" in check_names
        # VaR values should be populated (not "0")
        assert (
            response["portfolio_var_before"] != "0"
            or response["portfolio_var_after"] != "0"
        )

    def test_var_impact_check_skipped_without_returns(
        self, sample_portfolio: Portfolio
    ) -> None:
        """VaR impact check should be skipped when no returns_matrix is set."""
        servicer = _make_servicer(portfolio=sample_portfolio, returns_matrix=None)
        request = _make_request(
            instrument_id="AAPL",
            side="BUY",
            quantity="5",
            price="180.00",
        )
        response = _normalize_response(
            servicer.CheckPreTradeRisk(request, _make_context())
        )

        check_names = [c["name"] for c in response["checks"]]
        assert "var_impact" not in check_names
        assert response["portfolio_var_before"] == "0"
        assert response["portfolio_var_after"] == "0"

    def test_empty_portfolio_approves_small_order(self) -> None:
        """An empty portfolio with enough cash should approve small orders."""
        portfolio = Portfolio(
            cash=Decimal("100000"),
            available_cash=Decimal("100000"),
        )
        portfolio.compute_nav()
        servicer = _make_servicer(portfolio=portfolio)

        request = _make_request(
            instrument_id="NEW-STOCK",
            side="BUY",
            quantity="10",
            price="50.00",
        )
        response = _normalize_response(
            servicer.CheckPreTradeRisk(request, _make_context())
        )

        assert response["approved"] is True
        assert response["reject_reason"] == ""

    def test_order_at_exact_notional_limit(self, sample_portfolio: Portfolio) -> None:
        """Order with notional exactly equal to max should be approved."""
        servicer = _make_servicer(
            portfolio=sample_portfolio,
            max_order_notional=Decimal("900"),
        )
        # 5 * $180 = $900 == limit
        request = _make_request(
            instrument_id="AAPL",
            side="BUY",
            quantity="5",
            price="180.00",
        )
        response = _normalize_response(
            servicer.CheckPreTradeRisk(request, _make_context())
        )

        size_check = next(
            c for c in response["checks"] if c["name"] == "order_size_limit"
        )
        assert size_check["passed"] is True
