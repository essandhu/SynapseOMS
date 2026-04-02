"""FastAPI router for anomaly detection endpoints."""

from __future__ import annotations

from typing import Any

import structlog
from fastapi import APIRouter, HTTPException

from risk_engine.anomaly.consumer import AnomalyAlertPipeline

logger = structlog.get_logger()

router = APIRouter(prefix="/api/v1", tags=["anomaly"])


# ---------------------------------------------------------------------------
# Dependency container
# ---------------------------------------------------------------------------


class AnomalyDependencies:
    """Holds shared state for anomaly route handlers."""

    def __init__(self, alert_pipeline: AnomalyAlertPipeline | None = None) -> None:
        self.alert_pipeline = alert_pipeline


_deps: AnomalyDependencies | None = None


def configure_dependencies(deps: AnomalyDependencies) -> None:
    """Replace the module-level dependency holder."""
    global _deps  # noqa: PLW0603
    _deps = deps


# ---------------------------------------------------------------------------
# Endpoints
# ---------------------------------------------------------------------------


@router.get("/anomalies")
async def get_anomalies(
    limit: int = 50,
    severity: str | None = None,
    instrument_id: str | None = None,
) -> dict[str, Any]:
    """Return recent anomaly alerts with optional filtering."""
    if _deps is None or _deps.alert_pipeline is None:
        raise HTTPException(status_code=503, detail="Anomaly detection not configured")

    alerts = _deps.alert_pipeline.get_recent(limit=limit)

    if severity:
        alerts = [a for a in alerts if a.severity == severity]
    if instrument_id:
        alerts = [a for a in alerts if a.instrument_id == instrument_id]

    return {
        "alerts": [
            {
                "id": a.id,
                "instrument_id": a.instrument_id,
                "venue_id": a.venue_id,
                "anomaly_score": a.anomaly_score,
                "severity": a.severity,
                "features": a.features,
                "description": a.description,
                "timestamp": a.timestamp.isoformat(),
                "acknowledged": a.acknowledged,
            }
            for a in alerts
        ],
        "total": len(alerts),
    }


@router.post("/anomalies/{alert_id}/acknowledge")
async def acknowledge_alert(alert_id: str) -> dict[str, Any]:
    """Mark an alert as acknowledged and return it."""
    if _deps is None or _deps.alert_pipeline is None:
        raise HTTPException(status_code=503, detail="Anomaly detection not configured")

    if not _deps.alert_pipeline.acknowledge(alert_id):
        raise HTTPException(status_code=404, detail=f"Alert {alert_id} not found")

    # Return the updated alert
    for a in _deps.alert_pipeline.get_recent(limit=100):
        if a.id == alert_id:
            return {
                "id": a.id,
                "instrument_id": a.instrument_id,
                "venue_id": a.venue_id,
                "anomaly_score": a.anomaly_score,
                "severity": a.severity,
                "features": a.features,
                "description": a.description,
                "timestamp": a.timestamp.isoformat(),
                "acknowledged": a.acknowledged,
            }

    raise HTTPException(
        status_code=404, detail=f"Alert {alert_id} not found after acknowledge"
    )
