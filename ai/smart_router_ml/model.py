"""FastAPI scoring service for the smart-order-router ML model.

Provides:
- ``POST /score``  — score candidate venues for an order
- ``GET  /health`` — liveness / readiness probe
"""

from __future__ import annotations

import os
from pathlib import Path
from typing import Any

import numpy as np
from fastapi import FastAPI
from pydantic import BaseModel, Field, field_validator

from smart_router_ml.features import FEATURE_COLUMNS, extract_features

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
MODEL_PATH = os.environ.get("MODEL_PATH", "./model.json")

# ---------------------------------------------------------------------------
# Lazy-loaded XGBoost model (None ⇒ cold-start heuristic)
# ---------------------------------------------------------------------------
_xgb_model: Any | None = None
_model_checked: bool = False


def _load_model() -> Any | None:
    """Attempt to load an XGBoost model from *MODEL_PATH*.

    Returns ``None`` when no file is present (cold-start mode).
    """
    global _xgb_model, _model_checked
    if _model_checked:
        return _xgb_model
    _model_checked = True

    p = Path(MODEL_PATH)
    if p.exists():
        import xgboost as xgb

        booster = xgb.Booster()
        booster.load_model(str(p))
        _xgb_model = booster
    return _xgb_model


# ---------------------------------------------------------------------------
# Cold-start heuristic weights
# ---------------------------------------------------------------------------
_W_SPREAD = 0.50
_W_FEE = 0.30
_W_LATENCY = 0.20


def _cold_start_score(candidates: list[dict[str, Any]]) -> list[float]:
    """Simple weighted-sum heuristic used before a trained model exists.

    score = -1 * (spread_bps * w1 + maker_taker_fee * w2 + venue_latency_p50 * w3)
    """
    scores: list[float] = []
    for c in candidates:
        s = -1.0 * (
            float(c["spread_bps"]) * _W_SPREAD
            + float(c["maker_taker_fee"]) * _W_FEE
            + float(c["venue_latency_p50"]) * _W_LATENCY
        )
        scores.append(s)
    return scores


# ---------------------------------------------------------------------------
# Pydantic models
# ---------------------------------------------------------------------------
class OrderInfo(BaseModel):
    size: float
    instrument_id: str


class CandidateVenue(BaseModel):
    venue_id: str
    order_size_pct_adv: float
    spread_bps: float
    book_depth_at_price: float
    venue_fill_rate_30d: float
    venue_latency_p50: float
    cross_venue_price_diff: float
    hour_of_day: float
    instrument_volatility: float
    maker_taker_fee: float
    time_since_last_fill: float


class ScoreRequest(BaseModel):
    order: OrderInfo
    candidates: list[CandidateVenue] = Field(..., min_length=1)


class VenueScore(BaseModel):
    venue_id: str
    score: float
    rank: int


class ScoreResponse(BaseModel):
    scores: list[VenueScore]


class HealthResponse(BaseModel):
    status: str
    mode: str


# ---------------------------------------------------------------------------
# FastAPI application
# ---------------------------------------------------------------------------
app = FastAPI(title="Smart Router ML Scorer", version="0.1.0")


@app.get("/health", response_model=HealthResponse)
def health() -> HealthResponse:
    model = _load_model()
    mode = "model" if model is not None else "cold_start"
    return HealthResponse(status="ok", mode=mode)


@app.post("/score", response_model=ScoreResponse)
def score(req: ScoreRequest) -> ScoreResponse:
    model = _load_model()
    candidates_dicts = [c.model_dump() for c in req.candidates]

    if model is not None:
        import xgboost as xgb

        feature_matrix = extract_features(candidates_dicts)
        dmatrix = xgb.DMatrix(feature_matrix, feature_names=FEATURE_COLUMNS)
        raw_scores = model.predict(dmatrix).tolist()
    else:
        raw_scores = _cold_start_score(candidates_dicts)

    # Build ranked output (highest score = rank 1)
    indexed = [(i, raw_scores[i]) for i in range(len(raw_scores))]
    indexed.sort(key=lambda t: t[1], reverse=True)

    results: list[VenueScore] = []
    for rank, (idx, sc) in enumerate(indexed, start=1):
        results.append(
            VenueScore(
                venue_id=req.candidates[idx].venue_id,
                score=round(sc, 6),
                rank=rank,
            )
        )

    return ScoreResponse(scores=results)


# ---------------------------------------------------------------------------
# Entrypoint for ``python -m smart_router_ml.model``
# ---------------------------------------------------------------------------
if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8090)
