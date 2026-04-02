"""Tests for the FastAPI scoring service."""

import json
import os

import pytest
from fastapi.testclient import TestClient

from smart_router_ml.model import app


@pytest.fixture
def client():
    return TestClient(app)


def _make_request_body(num_candidates: int = 2) -> dict:
    candidates = []
    for i in range(num_candidates):
        candidates.append(
            {
                "venue_id": f"venue_{i}",
                "order_size_pct_adv": 0.05 + i * 0.01,
                "spread_bps": 1.2 + i * 0.5,
                "book_depth_at_price": 50000.0,
                "venue_fill_rate_30d": 0.92 - i * 0.05,
                "venue_latency_p50": 0.5 + i * 0.1,
                "cross_venue_price_diff": 0.3,
                "hour_of_day": 14.0,
                "instrument_volatility": 0.02,
                "maker_taker_fee": 0.8 + i * 0.2,
                "time_since_last_fill": 120.0,
            }
        )
    return {
        "order": {"size": 1000, "instrument_id": "AAPL"},
        "candidates": candidates,
    }


class TestHealthEndpoint:
    def test_health_returns_ok(self, client: TestClient):
        resp = client.get("/health")
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "ok"
        assert "mode" in data


class TestScoreEndpoint:
    def test_score_returns_valid_json(self, client: TestClient):
        body = _make_request_body(3)
        resp = client.post("/score", json=body)
        assert resp.status_code == 200
        data = resp.json()
        assert "scores" in data
        assert len(data["scores"]) == 3

    def test_score_entries_have_required_fields(self, client: TestClient):
        body = _make_request_body(2)
        resp = client.post("/score", json=body)
        data = resp.json()
        for entry in data["scores"]:
            assert "venue_id" in entry
            assert "score" in entry
            assert "rank" in entry
            assert isinstance(entry["score"], float)
            assert isinstance(entry["rank"], int)

    def test_scores_are_ranked_correctly(self, client: TestClient):
        body = _make_request_body(3)
        resp = client.post("/score", json=body)
        data = resp.json()
        ranks = [s["rank"] for s in data["scores"]]
        assert sorted(ranks) == list(range(1, len(ranks) + 1))
        # Verify ordering: rank 1 should have the highest score
        scores_by_rank = {s["rank"]: s["score"] for s in data["scores"]}
        for r in range(1, len(ranks)):
            assert scores_by_rank[r] >= scores_by_rank[r + 1]

    def test_cold_start_returns_reasonable_scores(self, client: TestClient):
        """In cold-start mode (no model file), scores should still be sensible."""
        body = _make_request_body(2)
        resp = client.post("/score", json=body)
        data = resp.json()
        for entry in data["scores"]:
            # Scores should be finite negative numbers (cost-based)
            assert entry["score"] < 0
            assert entry["score"] > -1000  # sanity bound

    def test_venue_ids_match_input(self, client: TestClient):
        body = _make_request_body(2)
        resp = client.post("/score", json=body)
        data = resp.json()
        input_ids = {c["venue_id"] for c in body["candidates"]}
        output_ids = {s["venue_id"] for s in data["scores"]}
        assert input_ids == output_ids

    def test_empty_candidates_returns_422(self, client: TestClient):
        body = {"order": {"size": 100, "instrument_id": "AAPL"}, "candidates": []}
        resp = client.post("/score", json=body)
        assert resp.status_code == 422

    def test_missing_fields_returns_422(self, client: TestClient):
        body = {"order": {"size": 100, "instrument_id": "AAPL"}, "candidates": [{"venue_id": "x"}]}
        resp = client.post("/score", json=body)
        assert resp.status_code == 422
