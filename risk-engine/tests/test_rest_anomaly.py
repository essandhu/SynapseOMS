"""Tests for FastAPI REST anomaly detection endpoints.

Uses FastAPI's TestClient with a mock AnomalyAlertPipeline.
"""

from __future__ import annotations

from datetime import datetime, timezone
from unittest.mock import MagicMock

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from risk_engine.anomaly.detector import AnomalyAlert
from risk_engine.rest.router_anomaly import (
    AnomalyDependencies,
    configure_dependencies,
    router as anomaly_router,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_alert(
    *,
    alert_id: str = "alert-1",
    instrument_id: str = "BTC-USD",
    venue_id: str = "COINBASE",
    anomaly_score: float = 0.95,
    severity: str = "critical",
    description: str = "Unusual volume spike",
    acknowledged: bool = False,
) -> AnomalyAlert:
    return AnomalyAlert(
        id=alert_id,
        instrument_id=instrument_id,
        venue_id=venue_id,
        anomaly_score=anomaly_score,
        severity=severity,
        features={"volume_zscore": 3.2, "spread_zscore": 1.1},
        description=description,
        timestamp=datetime(2026, 4, 1, 12, 0, 0, tzinfo=timezone.utc),
        acknowledged=acknowledged,
    )


def _make_alerts() -> list[AnomalyAlert]:
    """Create a set of test alerts with varying severity and instruments."""
    return [
        _make_alert(alert_id="a1", severity="critical", instrument_id="BTC-USD"),
        _make_alert(alert_id="a2", severity="warning", instrument_id="ETH-USD"),
        _make_alert(alert_id="a3", severity="warning", instrument_id="BTC-USD"),
        _make_alert(alert_id="a4", severity="info", instrument_id="ETH-USD"),
    ]


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture()
def mock_pipeline() -> MagicMock:
    """Mock AnomalyAlertPipeline with pre-loaded alerts."""
    pipeline = MagicMock()
    alerts = _make_alerts()
    pipeline.get_recent.return_value = alerts
    pipeline.acknowledge.return_value = True
    return pipeline


@pytest.fixture()
def anomaly_client(mock_pipeline: MagicMock) -> TestClient:
    """TestClient with anomaly dependencies configured."""
    deps = AnomalyDependencies(alert_pipeline=mock_pipeline)
    configure_dependencies(deps)

    app = FastAPI()
    app.include_router(anomaly_router)
    return TestClient(app)


@pytest.fixture()
def anomaly_client_unconfigured() -> TestClient:
    """TestClient with anomaly dependencies NOT configured."""
    configure_dependencies(None)  # type: ignore[arg-type]

    app = FastAPI()
    app.include_router(anomaly_router)
    return TestClient(app)


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestGetAnomalies:
    """Tests for GET /api/v1/anomalies."""

    def test_get_anomalies_returns_alerts(
        self, anomaly_client: TestClient, mock_pipeline: MagicMock
    ) -> None:
        """Should return all alerts from the pipeline."""
        resp = anomaly_client.get("/api/v1/anomalies")
        assert resp.status_code == 200
        data = resp.json()

        assert "alerts" in data
        assert "total" in data
        assert data["total"] == 4
        assert len(data["alerts"]) == 4

        # Verify alert shape
        alert = data["alerts"][0]
        assert "id" in alert
        assert "instrument_id" in alert
        assert "venue_id" in alert
        assert "anomaly_score" in alert
        assert "severity" in alert
        assert "features" in alert
        assert "description" in alert
        assert "timestamp" in alert
        assert "acknowledged" in alert

        mock_pipeline.get_recent.assert_called_once_with(limit=50)

    def test_filter_by_severity(
        self, anomaly_client: TestClient
    ) -> None:
        """Should filter alerts by severity query parameter."""
        resp = anomaly_client.get("/api/v1/anomalies?severity=warning")
        assert resp.status_code == 200
        data = resp.json()

        assert data["total"] == 2
        for alert in data["alerts"]:
            assert alert["severity"] == "warning"

    def test_filter_by_instrument(
        self, anomaly_client: TestClient
    ) -> None:
        """Should filter alerts by instrument_id query parameter."""
        resp = anomaly_client.get("/api/v1/anomalies?instrument_id=ETH-USD")
        assert resp.status_code == 200
        data = resp.json()

        assert data["total"] == 2
        for alert in data["alerts"]:
            assert alert["instrument_id"] == "ETH-USD"

    def test_503_when_not_configured(
        self, anomaly_client_unconfigured: TestClient
    ) -> None:
        """Should return 503 when dependencies are not configured."""
        resp = anomaly_client_unconfigured.get("/api/v1/anomalies")
        assert resp.status_code == 503
        assert "not configured" in resp.json()["detail"].lower()


class TestAcknowledgeAlert:
    """Tests for POST /api/v1/anomalies/{alert_id}/acknowledge."""

    def test_acknowledge_flips_flag(
        self, anomaly_client: TestClient, mock_pipeline: MagicMock
    ) -> None:
        """Should acknowledge an alert and return it with acknowledged=true."""
        # The POST handler calls get_recent(limit=100) once after acknowledge
        acked_alert = _make_alert(alert_id="a1", acknowledged=True)
        mock_pipeline.get_recent.return_value = [acked_alert]
        mock_pipeline.acknowledge.return_value = True

        resp = anomaly_client.post("/api/v1/anomalies/a1/acknowledge")
        assert resp.status_code == 200
        data = resp.json()

        assert data["id"] == "a1"
        assert data["acknowledged"] is True
        mock_pipeline.acknowledge.assert_called_once_with("a1")

    def test_acknowledge_unknown_returns_404(
        self, anomaly_client: TestClient, mock_pipeline: MagicMock
    ) -> None:
        """Should return 404 when alert ID is not found."""
        mock_pipeline.acknowledge.return_value = False

        resp = anomaly_client.post("/api/v1/anomalies/fake-id/acknowledge")
        assert resp.status_code == 404
        assert "fake-id" in resp.json()["detail"]

    def test_acknowledge_503_when_not_configured(
        self, anomaly_client_unconfigured: TestClient
    ) -> None:
        """Should return 503 when dependencies are not configured."""
        resp = anomaly_client_unconfigured.post("/api/v1/anomalies/a1/acknowledge")
        assert resp.status_code == 503
