"""Tests for the what-if scenario analysis REST endpoint.

POST /api/v1/risk/scenario accepts hypothetical positions and returns
a comparison of current vs. projected VaR.
"""

from __future__ import annotations

from decimal import Decimal

import pandas as pd
import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from risk_engine.domain.portfolio import Portfolio
from risk_engine.domain.position import Position
from risk_engine.rest.router_scenario import (
    ScenarioDependencies,
    configure_dependencies as configure_scenario_deps,
    router as scenario_router,
)
from risk_engine.var.historical import HistoricalVaR
from risk_engine.var.monte_carlo import MonteCarloVaR
from risk_engine.var.parametric import ParametricVaR


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture()
def empty_portfolio() -> Portfolio:
    """Empty portfolio with only cash."""
    return Portfolio(cash=Decimal("100000"), available_cash=Decimal("100000"))


@pytest.fixture()
def populated_portfolio(sample_positions: dict[str, Position]) -> Portfolio:
    """Portfolio with sample positions."""
    portfolio = Portfolio(
        positions=dict(sample_positions),
        cash=Decimal("100000"),
        available_cash=Decimal("100000"),
    )
    portfolio.compute_nav()
    return portfolio


@pytest.fixture()
def scenario_client(
    empty_portfolio: Portfolio,
    sample_returns_matrix: pd.DataFrame,
) -> TestClient:
    """TestClient wired with scenario dependencies (empty portfolio)."""
    deps = ScenarioDependencies(
        portfolio=empty_portfolio,
        historical_var=HistoricalVaR(),
        parametric_var=ParametricVaR(),
        monte_carlo_var=MonteCarloVaR(num_simulations=500),
        returns_matrix=sample_returns_matrix,
    )
    configure_scenario_deps(deps)

    app = FastAPI()
    app.include_router(scenario_router)
    return TestClient(app)


@pytest.fixture()
def scenario_client_with_portfolio(
    populated_portfolio: Portfolio,
    sample_returns_matrix: pd.DataFrame,
) -> TestClient:
    """TestClient wired with scenario dependencies (populated portfolio)."""
    deps = ScenarioDependencies(
        portfolio=populated_portfolio,
        historical_var=HistoricalVaR(),
        parametric_var=ParametricVaR(),
        monte_carlo_var=MonteCarloVaR(num_simulations=500),
        returns_matrix=sample_returns_matrix,
    )
    configure_scenario_deps(deps)

    app = FastAPI()
    app.include_router(scenario_router)
    return TestClient(app)


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestScenarioEndpoint:
    """Tests for POST /api/v1/risk/scenario."""

    def test_valid_scenario_returns_var(
        self, scenario_client: TestClient
    ) -> None:
        """POST with one hypothetical position returns all expected fields."""
        resp = scenario_client.post(
            "/api/v1/risk/scenario",
            json={
                "hypothetical_positions": [
                    {
                        "instrument_id": "AAPL",
                        "side": "buy",
                        "quantity": "100",
                        "price": "180.00",
                    }
                ]
            },
        )
        assert resp.status_code == 200
        data = resp.json()

        assert "current_var" in data
        assert "projected_var" in data
        assert "var_delta" in data
        assert "hypothetical_positions_added" in data

        # Each VaR section should have all three methods
        for section in ("current_var", "projected_var", "var_delta"):
            assert "historical" in data[section]
            assert "parametric" in data[section]
            assert "monte_carlo" in data[section]

        assert data["hypothetical_positions_added"] == 1

    def test_empty_scenario_rejected(
        self, scenario_client: TestClient
    ) -> None:
        """POST with empty hypothetical_positions list returns 422."""
        resp = scenario_client.post(
            "/api/v1/risk/scenario",
            json={"hypothetical_positions": []},
        )
        assert resp.status_code == 422

    def test_single_hypothetical_position(
        self, scenario_client_with_portfolio: TestClient
    ) -> None:
        """POST with one position; projected_var should differ from current."""
        resp = scenario_client_with_portfolio.post(
            "/api/v1/risk/scenario",
            json={
                "hypothetical_positions": [
                    {
                        "instrument_id": "AAPL",
                        "side": "buy",
                        "quantity": "500",
                        "price": "180.00",
                    }
                ]
            },
        )
        assert resp.status_code == 200
        data = resp.json()

        # With an existing portfolio, current VaR should be non-null
        assert data["current_var"]["historical"] is not None
        assert data["projected_var"]["historical"] is not None

        # Projected VaR should differ from current (more positions => different risk)
        assert data["projected_var"]["historical"] != data["current_var"]["historical"]

    def test_multiple_hypothetical_positions(
        self, scenario_client: TestClient
    ) -> None:
        """POST with two hypothetical positions should succeed."""
        resp = scenario_client.post(
            "/api/v1/risk/scenario",
            json={
                "hypothetical_positions": [
                    {
                        "instrument_id": "AAPL",
                        "side": "buy",
                        "quantity": "100",
                        "price": "180.00",
                    },
                    {
                        "instrument_id": "BTC-USD",
                        "side": "buy",
                        "quantity": "0.5",
                        "price": "65000.00",
                    },
                ]
            },
        )
        assert resp.status_code == 200
        data = resp.json()

        assert data["hypothetical_positions_added"] == 2
        assert data["projected_var"]["historical"] is not None

    def test_invalid_side_rejected(
        self, scenario_client: TestClient
    ) -> None:
        """POST with invalid side value returns 422."""
        resp = scenario_client.post(
            "/api/v1/risk/scenario",
            json={
                "hypothetical_positions": [
                    {
                        "instrument_id": "AAPL",
                        "side": "invalid",
                        "quantity": "100",
                        "price": "180.00",
                    }
                ]
            },
        )
        assert resp.status_code == 422

    def test_scenario_with_existing_portfolio(
        self, scenario_client_with_portfolio: TestClient
    ) -> None:
        """Pre-populated portfolio should have non-null current_var."""
        resp = scenario_client_with_portfolio.post(
            "/api/v1/risk/scenario",
            json={
                "hypothetical_positions": [
                    {
                        "instrument_id": "ETH-USD",
                        "side": "buy",
                        "quantity": "10",
                        "price": "3400.00",
                    }
                ]
            },
        )
        assert resp.status_code == 200
        data = resp.json()

        # Current VaR should be non-null since portfolio has positions
        assert data["current_var"]["historical"] is not None
        assert data["current_var"]["parametric"] is not None
