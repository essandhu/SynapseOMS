"""Tests for FastAPI REST AI endpoints (rebalancing + execution reports).

Uses FastAPI's TestClient with mock AI modules to avoid Anthropic API calls.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from ai.execution_analyst.types import ExecutionReport, TradeContext
from ai.rebalancing_assistant.types import ExtractedConstraints, RebalanceRequest

from risk_engine.rest.router_ai import (
    AIDependencies,
    configure_dependencies,
    router as ai_router,
)


# ---------------------------------------------------------------------------
# Mock factories
# ---------------------------------------------------------------------------


def _mock_constraints() -> ExtractedConstraints:
    """Return a predetermined ExtractedConstraints for testing."""
    return ExtractedConstraints(
        objective="maximize_sharpe",
        target_return=0.12,
        risk_aversion=1.0,
        long_only=True,
        max_single_weight=0.4,
        asset_class_bounds=None,
        sector_limits=None,
        target_volatility=None,
        max_turnover_usd=None,
        instruments_to_include=None,
        instruments_to_exclude=None,
        reasoning="User wants maximum risk-adjusted returns with diversification.",
    )


def _mock_execution_report() -> ExecutionReport:
    """Return a predetermined ExecutionReport for testing."""
    return ExecutionReport(
        overall_grade="B+",
        implementation_shortfall_bps=2.5,
        summary="Good execution with minimal market impact.",
        venue_analysis=[{"venue": "NYSE", "fill_rate": 0.95}],
        recommendations=["Consider splitting large orders across venues."],
        market_impact_estimate_bps=1.8,
    )


def _make_mock_rebalancing_assistant() -> MagicMock:
    """Create a mock RebalancingAssistant that returns predetermined constraints."""
    assistant = MagicMock()
    # extract_constraints is sync in the real class
    assistant.extract_constraints.return_value = _mock_constraints()
    return assistant


def _make_mock_execution_analyst() -> MagicMock:
    """Create a mock ExecutionAnalyst that returns a predetermined report."""
    analyst = MagicMock()
    # analyze_execution is async in the real class
    analyst.analyze_execution = AsyncMock(return_value=_mock_execution_report())
    return analyst


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture()
def ai_client() -> TestClient:
    """TestClient with mock AI dependencies configured."""
    deps = AIDependencies(
        execution_analyst=_make_mock_execution_analyst(),
        rebalancing_assistant=_make_mock_rebalancing_assistant(),
    )
    configure_dependencies(deps)

    app = FastAPI()
    app.include_router(ai_router)
    return TestClient(app)


@pytest.fixture()
def ai_client_unconfigured() -> TestClient:
    """TestClient with NO AI dependencies configured."""
    configure_dependencies(None)

    app = FastAPI()
    app.include_router(ai_router)
    return TestClient(app)


# ---------------------------------------------------------------------------
# Rebalance endpoint tests
# ---------------------------------------------------------------------------


class TestRebalanceEndpoint:
    """Tests for POST /api/v1/ai/rebalance."""

    def test_rebalance_endpoint_returns_constraints(
        self, ai_client: TestClient
    ) -> None:
        """Mock rebalancing_assistant.extract_constraints, verify response has constraints and reasoning."""
        resp = ai_client.post(
            "/api/v1/ai/rebalance",
            json={"prompt": "Maximize risk-adjusted returns, keep it diversified"},
        )
        assert resp.status_code == 200
        data = resp.json()

        assert "constraints" in data
        assert "reasoning" in data
        assert data["constraints"]["objective"] == "maximize_sharpe"
        assert data["constraints"]["longOnly"] is True
        assert data["constraints"]["maxSingleWeight"] == 0.4
        assert "risk-adjusted" in data["reasoning"].lower()

    def test_rebalance_503_without_api_key(
        self, ai_client_unconfigured: TestClient
    ) -> None:
        """No AI deps configured, returns 503."""
        resp = ai_client_unconfigured.post(
            "/api/v1/ai/rebalance",
            json={"prompt": "Rebalance my portfolio"},
        )
        assert resp.status_code == 503
        assert "not configured" in resp.json()["detail"].lower()


# ---------------------------------------------------------------------------
# Execution report endpoint tests
# ---------------------------------------------------------------------------


_SAMPLE_EXECUTION_BODY = {
    "side": "buy",
    "quantity": 1000,
    "instrument_id": "AAPL",
    "asset_class": "equity",
    "order_type": "limit",
    "limit_price": 180.50,
    "submitted_at": "2026-04-01T10:00:00",
    "completed_at": "2026-04-01T10:05:00",
    "venues": "NYSE, NASDAQ",
    "fill_count": 3,
    "fill_table": "fill1: 400@180.45, fill2: 300@180.50, fill3: 300@180.52",
    "arrival_price": 180.40,
    "spread_bps": 1.2,
    "vwap_5min": 180.48,
    "adv_30d": 50000000,
    "size_pct_adv": 0.002,
    "venue_comparison_table": "NYSE: 70%, NASDAQ: 30%",
}


class TestExecutionReportEndpoint:
    """Tests for POST /api/v1/ai/execution-report and GET /api/v1/ai/execution-reports."""

    def test_execution_report_stored_and_retrieved(
        self, ai_client: TestClient
    ) -> None:
        """Post execution report, then GET /execution-reports returns it."""
        # Post an execution report
        resp = ai_client.post(
            "/api/v1/ai/execution-report",
            json=_SAMPLE_EXECUTION_BODY,
        )
        assert resp.status_code == 200
        report = resp.json()
        assert report["overall_grade"] == "B+"
        assert report["summary"] == "Good execution with minimal market impact."
        assert "orderId" in report
        assert "analyzedAt" in report

        # Retrieve stored reports
        resp2 = ai_client.get("/api/v1/ai/execution-reports")
        assert resp2.status_code == 200
        reports = resp2.json()
        assert len(reports) >= 1
        assert reports[0]["overall_grade"] == "B+"

    def test_execution_reports_empty_initially(
        self, ai_client_unconfigured: TestClient
    ) -> None:
        """GET returns 503 when AI not configured."""
        # With unconfigured deps, the GET endpoint returns 503
        resp = ai_client_unconfigured.get("/api/v1/ai/execution-reports")
        assert resp.status_code == 503


class TestExecutionReportsEmptyList:
    """Test that a freshly configured AI client has no reports."""

    def test_execution_reports_empty_initially(self) -> None:
        """GET returns empty list when no reports have been submitted."""
        deps = AIDependencies(
            execution_analyst=_make_mock_execution_analyst(),
            rebalancing_assistant=_make_mock_rebalancing_assistant(),
        )
        configure_dependencies(deps)

        app = FastAPI()
        app.include_router(ai_router)
        client = TestClient(app)

        resp = client.get("/api/v1/ai/execution-reports")
        assert resp.status_code == 200
        assert resp.json() == []
