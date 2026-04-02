"""Tests for the streaming anomaly detector."""

from __future__ import annotations

from datetime import datetime, timezone

import numpy as np
import pytest

from risk_engine.anomaly.detector import AnomalyAlert, StreamingAnomalyDetector


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_snapshot(
    volume: float = 1000.0,
    price: float = 100.0,
    bid: float = 99.9,
    ask: float = 100.1,
    instrument_id: str = "ETH-USD",
    venue_id: str = "Binance",
) -> dict:
    return {
        "instrument_id": instrument_id,
        "venue_id": venue_id,
        "volume": volume,
        "price": price,
        "bid": bid,
        "ask": ask,
        "timestamp": datetime.now(timezone.utc).isoformat(),
    }


def _warmup_detector(
    detector: StreamingAnomalyDetector,
    n: int = 200,
    rng: np.random.Generator | None = None,
) -> None:
    """Feed n normal snapshots so the model trains."""
    if rng is None:
        rng = np.random.default_rng(42)
    for _ in range(n):
        snap = _make_snapshot(
            volume=1000.0 + rng.normal(0, 50),
            price=100.0 + rng.normal(0, 1),
            bid=99.9 + rng.normal(0, 0.5),
            ask=100.1 + rng.normal(0, 0.5),
        )
        detector.ingest(snap)


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestFeatureExtraction:
    """Feature vector shape and content."""

    def test_produces_correct_shape(self) -> None:
        det = StreamingAnomalyDetector()
        # Need at least one prior sample for z-scores
        det.ingest(_make_snapshot())
        features = det._extract_features(_make_snapshot())
        assert features.shape == (5,), f"Expected 5 features, got {features.shape}"

    def test_features_are_finite(self) -> None:
        det = StreamingAnomalyDetector()
        det.ingest(_make_snapshot())
        features = det._extract_features(_make_snapshot())
        assert np.all(np.isfinite(features))


class TestInsufficientData:
    """Detector must return None when not enough data."""

    def test_returns_none_below_100_samples(self) -> None:
        det = StreamingAnomalyDetector()
        for _ in range(99):
            result = det.ingest(_make_snapshot())
            assert result is None, "Should return None with < 100 samples"


class TestNormalData:
    """Normal data should not trigger alerts."""

    def test_normal_data_returns_none(self) -> None:
        det = StreamingAnomalyDetector()
        rng = np.random.default_rng(42)
        _warmup_detector(det, n=200, rng=rng)
        # Feed more normal data — should not trigger
        alerts = []
        for _ in range(50):
            snap = _make_snapshot(
                volume=1000.0 + rng.normal(0, 50),
                price=100.0 + rng.normal(0, 1),
                bid=99.9 + rng.normal(0, 0.5),
                ask=100.1 + rng.normal(0, 0.5),
            )
            result = det.ingest(snap)
            if result is not None:
                alerts.append(result)
        # At most 1-2 false positives with contamination=0.01
        assert len(alerts) <= 3, f"Too many false positives: {len(alerts)}"


class TestOutlierDetection:
    """Extreme outliers should trigger alerts after warmup."""

    def test_extreme_outlier_triggers_alert(self) -> None:
        det = StreamingAnomalyDetector()
        rng = np.random.default_rng(42)
        _warmup_detector(det, n=200, rng=rng)
        # Inject extreme outlier: volume 100x normal, huge spread
        outlier = _make_snapshot(volume=100_000.0, price=200.0, bid=50.0, ask=250.0)
        result = det.ingest(outlier)
        assert result is not None, "Extreme outlier should trigger alert"
        assert isinstance(result, AnomalyAlert)
        assert result.instrument_id == "ETH-USD"
        assert result.venue_id == "Binance"
        assert result.severity in ("info", "warning", "critical")


class TestSeverityThresholds:
    """Severity classification based on anomaly score."""

    def test_info_threshold(self) -> None:
        det = StreamingAnomalyDetector()
        assert det._determine_severity(-0.2) == "info"

    def test_warning_threshold(self) -> None:
        det = StreamingAnomalyDetector()
        assert det._determine_severity(-0.4) == "warning"

    def test_critical_threshold(self) -> None:
        det = StreamingAnomalyDetector()
        assert det._determine_severity(-0.8) == "critical"

    def test_positive_score_is_info(self) -> None:
        det = StreamingAnomalyDetector()
        assert det._determine_severity(0.1) == "info"


class TestRetrainInterval:
    """Retrain scheduling."""

    def test_initial_retrain_at_100_samples(self) -> None:
        det = StreamingAnomalyDetector()
        for _ in range(99):
            det.ingest(_make_snapshot())
        assert not det._trained
        det.ingest(_make_snapshot())
        assert det._trained

    def test_retrain_interval_respected(self) -> None:
        det = StreamingAnomalyDetector(retrain_interval_minutes=60)
        _warmup_detector(det, n=200)
        first_retrain = det._last_retrain
        assert first_retrain is not None
        # Ingesting more should NOT retrain immediately
        det.ingest(_make_snapshot())
        assert det._last_retrain == first_retrain
