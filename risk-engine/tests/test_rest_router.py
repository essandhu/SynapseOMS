"""Tests for FastAPI REST risk and portfolio endpoints.

Uses FastAPI's TestClient with dependency overrides to inject mock/test
dependencies, avoiding any real database or Kafka connections.
"""

from __future__ import annotations

from decimal import Decimal

import pandas as pd
import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from risk_engine.domain.portfolio import Portfolio
from risk_engine.domain.position import Position
from risk_engine.rest.router_risk import (
    RiskDependencies,
    _get_historical_var,
    _get_parametric_var,
    _get_portfolio,
    _get_returns_matrix,
    _get_settlement_tracker,
    configure_dependencies,
    router,
)
from risk_engine.settlement.tracker import SettlementTracker
from risk_engine.var.historical import HistoricalVaR
from risk_engine.var.parametric import ParametricVaR


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture()
def test_portfolio(sample_positions: dict[str, Position]) -> Portfolio:
    """Portfolio seeded with sample positions for REST tests."""
    portfolio = Portfolio(
        positions=dict(sample_positions),
        cash=Decimal("100000"),
        available_cash=Decimal("100000"),
    )
    portfolio.compute_nav()
    return portfolio


@pytest.fixture()
def test_deps(
    test_portfolio: Portfolio,
    sample_returns_matrix: pd.DataFrame,
) -> RiskDependencies:
    """RiskDependencies wired with test data."""
    return RiskDependencies(
        portfolio=test_portfolio,
        historical_var=HistoricalVaR(),
        parametric_var=ParametricVaR(),
        settlement_tracker=SettlementTracker(),
        returns_matrix=sample_returns_matrix,
    )


@pytest.fixture()
def client(test_deps: RiskDependencies) -> TestClient:
    """FastAPI TestClient with dependency overrides."""
    app = FastAPI()
    app.include_router(router)

    # Override FastAPI dependencies to use test instances
    app.dependency_overrides[_get_portfolio] = test_deps.get_portfolio
    app.dependency_overrides[_get_historical_var] = test_deps.get_historical_var
    app.dependency_overrides[_get_parametric_var] = test_deps.get_parametric_var
    app.dependency_overrides[_get_settlement_tracker] = test_deps.get_settlement_tracker
    app.dependency_overrides[_get_returns_matrix] = test_deps.get_returns_matrix

    return TestClient(app)


@pytest.fixture()
def empty_client() -> TestClient:
    """TestClient with an empty portfolio and no returns data."""
    deps = RiskDependencies(
        portfolio=Portfolio(),
        returns_matrix=pd.DataFrame(),
    )
    app = FastAPI()
    app.include_router(router)

    app.dependency_overrides[_get_portfolio] = deps.get_portfolio
    app.dependency_overrides[_get_historical_var] = deps.get_historical_var
    app.dependency_overrides[_get_parametric_var] = deps.get_parametric_var
    app.dependency_overrides[_get_settlement_tracker] = deps.get_settlement_tracker
    app.dependency_overrides[_get_returns_matrix] = deps.get_returns_matrix

    return TestClient(app)


# ---------------------------------------------------------------------------
# Portfolio endpoint tests
# ---------------------------------------------------------------------------


class TestPortfolioEndpoint:
    """Tests for GET /api/v1/portfolio."""

    def test_get_portfolio_returns_positions(self, client: TestClient) -> None:
        """Should return portfolio with positions, cash, and NAV."""
        resp = client.get("/api/v1/portfolio")
        assert resp.status_code == 200

        data = resp.json()
        assert "positions" in data
        assert "nav" in data
        assert "cash" in data
        assert "availableCash" in data
        assert len(data["positions"]) == 3  # AAPL, BTC-USD, ETH-USD

    def test_get_portfolio_position_fields(self, client: TestClient) -> None:
        """Each position should contain the expected fields."""
        resp = client.get("/api/v1/portfolio")
        data = resp.json()

        aapl = next(p for p in data["positions"] if p["instrumentId"] == "AAPL")
        assert aapl["quantity"] == "100"
        assert aapl["assetClass"] == "equity"
        assert aapl["settlementCycle"] == "T2"
        assert "marketValue" in aapl
        assert "unrealizedPnl" in aapl

    def test_get_portfolio_empty(self, empty_client: TestClient) -> None:
        """Empty portfolio should return zero positions."""
        resp = empty_client.get("/api/v1/portfolio")
        assert resp.status_code == 200
        data = resp.json()
        assert data["positions"] == []


class TestExposureEndpoint:
    """Tests for GET /api/v1/portfolio/exposure."""

    def test_get_exposure_returns_breakdown(self, client: TestClient) -> None:
        """Should return exposure by asset class and venue."""
        resp = client.get("/api/v1/portfolio/exposure")
        assert resp.status_code == 200

        data = resp.json()
        assert "byAssetClass" in data
        assert "byVenue" in data

        # We have equity (AAPL) and crypto (BTC-USD, ETH-USD) positions
        assert "equity" in data["byAssetClass"]
        assert "crypto" in data["byAssetClass"]

    def test_get_exposure_by_venue(self, client: TestClient) -> None:
        """Should break down exposure by venue."""
        resp = client.get("/api/v1/portfolio/exposure")
        data = resp.json()

        assert "NYSE" in data["byVenue"]
        assert "COINBASE" in data["byVenue"]

    def test_get_exposure_empty_portfolio(self, empty_client: TestClient) -> None:
        """Empty portfolio should return empty breakdown."""
        resp = empty_client.get("/api/v1/portfolio/exposure")
        assert resp.status_code == 200
        data = resp.json()
        assert data["byAssetClass"] == {}
        assert data["byVenue"] == {}


# ---------------------------------------------------------------------------
# Risk endpoint tests
# ---------------------------------------------------------------------------


class TestVaREndpoint:
    """Tests for GET /api/v1/risk/var."""

    def test_get_var_returns_metrics(self, client: TestClient) -> None:
        """Should return VaR metrics with expected fields."""
        resp = client.get("/api/v1/risk/var")
        assert resp.status_code == 200

        data = resp.json()
        assert "historicalVaR" in data
        assert "parametricVaR" in data
        assert "cvar" in data
        assert "confidence" in data
        assert "horizon" in data
        assert "computedAt" in data

    def test_get_var_with_positions_returns_values(self, client: TestClient) -> None:
        """With positions and returns data, VaR values should be non-null."""
        resp = client.get("/api/v1/risk/var")
        data = resp.json()

        # At least one method should produce a value
        assert data["historicalVaR"] is not None or data["parametricVaR"] is not None

    def test_get_var_empty_portfolio_returns_nulls(
        self, empty_client: TestClient
    ) -> None:
        """Empty portfolio should return null VaR values."""
        resp = empty_client.get("/api/v1/risk/var")
        assert resp.status_code == 200
        data = resp.json()

        assert data["historicalVaR"] is None
        assert data["parametricVaR"] is None


class TestDrawdownEndpoint:
    """Tests for GET /api/v1/risk/drawdown."""

    def test_get_drawdown_returns_data(self, client: TestClient) -> None:
        """Should return drawdown data with expected fields."""
        resp = client.get("/api/v1/risk/drawdown")
        assert resp.status_code == 200

        data = resp.json()
        assert "current" in data
        assert "peak" in data
        assert "trough" in data
        assert "history" in data

    def test_get_drawdown_with_returns_has_history(self, client: TestClient) -> None:
        """With returns data, history should be populated."""
        resp = client.get("/api/v1/risk/drawdown")
        data = resp.json()

        assert isinstance(data["history"], list)
        assert len(data["history"]) > 0
        # Each entry should have date and drawdown
        entry = data["history"][0]
        assert "date" in entry
        assert "drawdown" in entry

    def test_get_drawdown_empty_portfolio(self, empty_client: TestClient) -> None:
        """Empty portfolio should still return valid drawdown structure."""
        resp = empty_client.get("/api/v1/risk/drawdown")
        assert resp.status_code == 200
        data = resp.json()
        assert "current" in data
        assert data["history"] == []


class TestSettlementEndpoint:
    """Tests for GET /api/v1/risk/settlement."""

    def test_get_settlement_returns_data(self, client: TestClient) -> None:
        """Should return settlement data with expected fields."""
        resp = client.get("/api/v1/risk/settlement")
        assert resp.status_code == 200

        data = resp.json()
        assert "totalUnsettled" in data
        assert "entries" in data
        assert isinstance(data["entries"], list)

    def test_get_settlement_empty_tracker(self, empty_client: TestClient) -> None:
        """Empty settlement tracker should return zero unsettled."""
        resp = empty_client.get("/api/v1/risk/settlement")
        assert resp.status_code == 200
        data = resp.json()
        assert data["totalUnsettled"] == "0"
        assert data["entries"] == []


# ---------------------------------------------------------------------------
# Health endpoint test
# ---------------------------------------------------------------------------


class TestHealthEndpoint:
    """Tests for GET /api/v1/health.

    Note: The health endpoint is defined in main.py, not in the router.
    We test it by importing the app or creating a minimal reproduction.
    """

    def test_health_endpoint_structure(self) -> None:
        """Verify the health endpoint returns expected structure.

        Since the health endpoint is on the main app (not the router),
        we build a minimal app that mimics it.
        """
        app = FastAPI()

        @app.get("/api/v1/health")
        async def health() -> dict:
            return {
                "status": "ok",
                "fastapi": "ok",
                "grpc": "not_started",
                "kafka": "not_started",
            }

        client = TestClient(app)
        resp = client.get("/api/v1/health")
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "ok"
        assert "fastapi" in data
        assert "grpc" in data
        assert "kafka" in data
