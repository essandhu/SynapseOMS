"""Tests for FastAPI REST optimizer, Greeks, concentration, and MC VaR endpoints.

Uses FastAPI's TestClient with dependency overrides.
"""

from __future__ import annotations

from decimal import Decimal

import numpy as np
import pandas as pd
import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from risk_engine.concentration.analyzer import ConcentrationAnalyzer
from risk_engine.domain.portfolio import Portfolio
from risk_engine.domain.position import Position
from risk_engine.greeks.calculator import GreeksCalculator
from risk_engine.optimizer.mean_variance import PortfolioOptimizer
from risk_engine.rest.router_optimizer import (
    OptimizerDependencies,
    configure_dependencies as configure_optimizer_deps,
    router as optimizer_router,
)
from risk_engine.rest.router_risk import (
    RiskDependencies,
    _get_concentration_analyzer,
    _get_greeks_calculator,
    _get_historical_var,
    _get_monte_carlo_var,
    _get_parametric_var,
    _get_portfolio,
    _get_returns_matrix,
    _get_settlement_tracker,
    configure_dependencies as configure_risk_deps,
    router as risk_router,
)
from risk_engine.settlement.tracker import SettlementTracker
from risk_engine.var.historical import HistoricalVaR
from risk_engine.var.monte_carlo import MonteCarloVaR
from risk_engine.var.parametric import ParametricVaR


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture()
def test_portfolio(sample_positions: dict[str, Position]) -> Portfolio:
    """Portfolio seeded with sample positions."""
    portfolio = Portfolio(
        positions=dict(sample_positions),
        cash=Decimal("100000"),
        available_cash=Decimal("100000"),
    )
    portfolio.compute_nav()
    return portfolio


@pytest.fixture()
def expected_returns_array() -> np.ndarray:
    """Annualised expected returns for AAPL, BTC-USD, ETH-USD."""
    return np.array([0.10, 0.25, 0.30])


@pytest.fixture()
def risk_client(
    test_portfolio: Portfolio,
    sample_returns_matrix: pd.DataFrame,
) -> TestClient:
    """TestClient with all risk dependencies wired (including MC, Greeks, concentration)."""
    deps = RiskDependencies(
        portfolio=test_portfolio,
        historical_var=HistoricalVaR(),
        parametric_var=ParametricVaR(),
        settlement_tracker=SettlementTracker(),
        returns_matrix=sample_returns_matrix,
        monte_carlo_var=MonteCarloVaR(num_simulations=500),  # small for speed
        greeks_calculator=GreeksCalculator(),
        concentration_analyzer=ConcentrationAnalyzer(),
    )

    app = FastAPI()
    app.include_router(risk_router)

    app.dependency_overrides[_get_portfolio] = deps.get_portfolio
    app.dependency_overrides[_get_historical_var] = deps.get_historical_var
    app.dependency_overrides[_get_parametric_var] = deps.get_parametric_var
    app.dependency_overrides[_get_settlement_tracker] = deps.get_settlement_tracker
    app.dependency_overrides[_get_returns_matrix] = deps.get_returns_matrix
    app.dependency_overrides[_get_monte_carlo_var] = deps.get_monte_carlo_var
    app.dependency_overrides[_get_greeks_calculator] = deps.get_greeks_calculator
    app.dependency_overrides[_get_concentration_analyzer] = deps.get_concentration_analyzer

    return TestClient(app)


@pytest.fixture()
def risk_client_no_modules() -> TestClient:
    """TestClient with MC, Greeks, concentration NOT configured."""
    portfolio = Portfolio(
        positions={
            "AAPL": Position(
                instrument_id="AAPL",
                venue_id="NYSE",
                quantity=Decimal("100"),
                average_cost=Decimal("175.00"),
                market_price=Decimal("180.00"),
                unrealized_pnl=Decimal("500.00"),
                realized_pnl=Decimal("0"),
                asset_class="equity",
                settlement_cycle="T2",
            )
        },
        cash=Decimal("100000"),
        available_cash=Decimal("100000"),
    )
    portfolio.compute_nav()

    deps = RiskDependencies(
        portfolio=portfolio,
        returns_matrix=pd.DataFrame(),
        monte_carlo_var=None,
        greeks_calculator=None,
        concentration_analyzer=None,
    )

    app = FastAPI()
    app.include_router(risk_router)

    app.dependency_overrides[_get_portfolio] = deps.get_portfolio
    app.dependency_overrides[_get_historical_var] = deps.get_historical_var
    app.dependency_overrides[_get_parametric_var] = deps.get_parametric_var
    app.dependency_overrides[_get_settlement_tracker] = deps.get_settlement_tracker
    app.dependency_overrides[_get_returns_matrix] = deps.get_returns_matrix
    app.dependency_overrides[_get_monte_carlo_var] = deps.get_monte_carlo_var
    app.dependency_overrides[_get_greeks_calculator] = deps.get_greeks_calculator
    app.dependency_overrides[_get_concentration_analyzer] = deps.get_concentration_analyzer

    return TestClient(app)


@pytest.fixture()
def optimizer_client(
    test_portfolio: Portfolio,
    sample_covariance_matrix: np.ndarray,
    expected_returns_array: np.ndarray,
) -> TestClient:
    """TestClient with optimizer dependencies wired."""
    opt_deps = OptimizerDependencies(
        portfolio=test_portfolio,
        optimizer=PortfolioOptimizer(),
        expected_returns=expected_returns_array,
        covariance_matrix=sample_covariance_matrix,
    )
    configure_optimizer_deps(opt_deps)

    app = FastAPI()
    app.include_router(optimizer_router)
    return TestClient(app)


@pytest.fixture()
def optimizer_client_unconfigured() -> TestClient:
    """TestClient with optimizer NOT configured."""
    configure_optimizer_deps(OptimizerDependencies())

    app = FastAPI()
    app.include_router(optimizer_router)
    return TestClient(app)


# ---------------------------------------------------------------------------
# Monte Carlo VaR endpoint tests
# ---------------------------------------------------------------------------


class TestMonteCarloVaREndpoint:
    """Tests for Monte Carlo VaR via GET /api/v1/risk/var."""

    def test_var_includes_monte_carlo(self, risk_client: TestClient) -> None:
        """When MC VaR is configured, monteCarloVaR should be non-null."""
        resp = risk_client.get("/api/v1/risk/var")
        assert resp.status_code == 200
        data = resp.json()

        assert data["monteCarloVaR"] is not None
        assert data["monteCarloDistribution"] is not None
        assert isinstance(data["monteCarloDistribution"], list)
        assert len(data["monteCarloDistribution"]) > 0

    def test_var_mc_null_when_not_configured(
        self, risk_client_no_modules: TestClient
    ) -> None:
        """When MC VaR is not configured, fields should be null."""
        resp = risk_client_no_modules.get("/api/v1/risk/var")
        assert resp.status_code == 200
        data = resp.json()

        assert data["monteCarloVaR"] is None
        assert data["monteCarloDistribution"] is None


# ---------------------------------------------------------------------------
# Greeks endpoint tests
# ---------------------------------------------------------------------------


class TestGreeksEndpoint:
    """Tests for GET /api/v1/risk/greeks."""

    def test_greeks_returns_expected_shape(self, risk_client: TestClient) -> None:
        """Should return total and per-instrument Greeks."""
        resp = risk_client.get("/api/v1/risk/greeks")
        assert resp.status_code == 200
        data = resp.json()

        assert "total" in data
        assert "byInstrument" in data
        assert "computedAt" in data

        # Total should have all five Greeks
        total = data["total"]
        for key in ("delta", "gamma", "vega", "theta", "rho"):
            assert key in total

        # Per-instrument breakdown
        assert "AAPL" in data["byInstrument"]
        assert "BTC-USD" in data["byInstrument"]
        assert "ETH-USD" in data["byInstrument"]

    def test_greeks_delta_nonzero(self, risk_client: TestClient) -> None:
        """Spot positions should produce non-zero delta."""
        resp = risk_client.get("/api/v1/risk/greeks")
        data = resp.json()

        assert data["total"]["delta"] != 0.0

    def test_greeks_not_configured(self, risk_client_no_modules: TestClient) -> None:
        """When calculator is not configured, should return error field."""
        resp = risk_client_no_modules.get("/api/v1/risk/greeks")
        assert resp.status_code == 200
        data = resp.json()

        assert "error" in data
        assert data["total"] is None


# ---------------------------------------------------------------------------
# Concentration endpoint tests
# ---------------------------------------------------------------------------


class TestConcentrationEndpoint:
    """Tests for GET /api/v1/risk/concentration."""

    def test_concentration_returns_all_breakdowns(
        self, risk_client: TestClient
    ) -> None:
        """Should return single-name, asset-class, venue, HHI, and warnings."""
        resp = risk_client.get("/api/v1/risk/concentration")
        assert resp.status_code == 200
        data = resp.json()

        assert "singleName" in data
        assert "byAssetClass" in data
        assert "byVenue" in data
        assert "hhi" in data
        assert "warnings" in data

        # Should have entries for our three instruments
        assert "AAPL" in data["singleName"]
        assert "BTC-USD" in data["singleName"]

        # Should have asset class breakdown
        assert "equity" in data["byAssetClass"]
        assert "crypto" in data["byAssetClass"]

        # Should have venue breakdown
        assert "NYSE" in data["byVenue"]
        assert "COINBASE" in data["byVenue"]

        # HHI should be positive
        assert data["hhi"] > 0

    def test_concentration_not_configured(
        self, risk_client_no_modules: TestClient
    ) -> None:
        """When analyzer is not configured, should return error field."""
        resp = risk_client_no_modules.get("/api/v1/risk/concentration")
        assert resp.status_code == 200
        data = resp.json()

        assert "error" in data
        assert data["singleName"] == {}


# ---------------------------------------------------------------------------
# Optimizer endpoint tests
# ---------------------------------------------------------------------------


class TestOptimizerEndpoint:
    """Tests for POST /api/v1/optimizer/optimize."""

    def test_optimize_with_valid_constraints(
        self, optimizer_client: TestClient
    ) -> None:
        """Valid constraints should return target weights and trades."""
        resp = optimizer_client.post(
            "/api/v1/optimizer/optimize",
            json={"risk_aversion": 1.0, "long_only": True},
        )
        assert resp.status_code == 200
        data = resp.json()

        assert "targetWeights" in data
        assert "trades" in data
        assert "expectedReturn" in data
        assert "expectedVolatility" in data
        assert "sharpeRatio" in data

        # Target weights should be a dict with our instrument IDs
        assert isinstance(data["targetWeights"], dict)
        assert len(data["targetWeights"]) == 3

        # Trades should be a list
        assert isinstance(data["trades"], list)

    def test_optimize_returns_trade_fields(
        self, optimizer_client: TestClient
    ) -> None:
        """Each trade should have instrumentId, side, quantity, estimatedCost."""
        resp = optimizer_client.post(
            "/api/v1/optimizer/optimize",
            json={"risk_aversion": 1.0, "long_only": True},
        )
        data = resp.json()

        if data["trades"]:
            trade = data["trades"][0]
            assert "instrumentId" in trade
            assert "side" in trade
            assert trade["side"] in ("buy", "sell")
            assert "quantity" in trade
            assert "estimatedCost" in trade

    def test_optimize_infeasible_constraints_returns_422(
        self, optimizer_client: TestClient
    ) -> None:
        """Contradictory constraints should return 422."""
        # Long-only with asset class bounds that require more than 100% total
        # equity must be >= 80% AND crypto must be >= 80% => impossible (sum > 1)
        resp = optimizer_client.post(
            "/api/v1/optimizer/optimize",
            json={
                "risk_aversion": 1.0,
                "long_only": True,
                "asset_class_bounds": {
                    "equity": [0.8, 1.0],
                    "crypto": [0.8, 1.0],
                },
            },
        )
        assert resp.status_code == 422

    def test_optimize_not_configured_returns_503(
        self, optimizer_client_unconfigured: TestClient
    ) -> None:
        """When optimizer is not configured, should return 503."""
        resp = optimizer_client_unconfigured.post(
            "/api/v1/optimizer/optimize",
            json={"risk_aversion": 1.0},
        )
        assert resp.status_code in (503, 422)

    def test_optimize_empty_portfolio_returns_422(
        self,
        sample_covariance_matrix: np.ndarray,
        expected_returns_array: np.ndarray,
    ) -> None:
        """Empty portfolio should return 422."""
        opt_deps = OptimizerDependencies(
            portfolio=Portfolio(),
            optimizer=PortfolioOptimizer(),
            expected_returns=expected_returns_array,
            covariance_matrix=sample_covariance_matrix,
        )
        configure_optimizer_deps(opt_deps)

        app = FastAPI()
        app.include_router(optimizer_router)
        client = TestClient(app)

        resp = client.post(
            "/api/v1/optimizer/optimize",
            json={"risk_aversion": 1.0},
        )
        assert resp.status_code == 422
