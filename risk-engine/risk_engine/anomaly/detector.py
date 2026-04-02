"""Streaming Isolation Forest anomaly detector for market data."""

from __future__ import annotations

import uuid
from collections import deque
from dataclasses import dataclass, field
from datetime import datetime, timezone

import numpy as np
from sklearn.ensemble import IsolationForest


@dataclass
class AnomalyAlert:
    """A detected market anomaly."""

    id: str
    instrument_id: str
    venue_id: str
    anomaly_score: float
    severity: str  # "info" | "warning" | "critical"
    features: dict[str, float]
    description: str
    timestamp: datetime
    acknowledged: bool = False


_FEATURE_NAMES = [
    "volume_zscore",
    "price_return_zscore",
    "spread_zscore",
    "volume_price_corr",
    "cross_venue_divergence",
]


class StreamingAnomalyDetector:
    """Incrementally scores market snapshots via Isolation Forest."""

    def __init__(
        self,
        retrain_interval_minutes: int = 60,
        contamination: float = 0.01,
    ) -> None:
        self._model = IsolationForest(
            contamination=contamination, random_state=42,
        )
        self._retrain_minutes = retrain_interval_minutes
        self.feature_window: deque[np.ndarray] = deque(maxlen=10_000)
        self._trained: bool = False
        self._last_retrain: datetime | None = None
        # Rolling raw values for z-score computation
        self._volumes: deque[float] = deque(maxlen=10_000)
        self._prices: deque[float] = deque(maxlen=10_000)
        self._spreads: deque[float] = deque(maxlen=10_000)

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def ingest(self, snapshot: dict) -> AnomalyAlert | None:
        """Score a market snapshot; return alert if anomalous."""
        self._volumes.append(float(snapshot["volume"]))
        self._prices.append(float(snapshot["price"]))
        self._spreads.append(
            float(snapshot["ask"]) - float(snapshot["bid"]),
        )

        features = self._extract_features(snapshot)
        self.feature_window.append(features)

        if len(self.feature_window) < 100:
            return None

        if self._should_retrain():
            data = np.array(self.feature_window)
            self._model.fit(data)
            self._trained = True
            self._last_retrain = datetime.now(timezone.utc)

        if not self._trained:
            return None

        score = float(self._model.score_samples(features.reshape(1, -1))[0])
        prediction = int(self._model.predict(features.reshape(1, -1))[0])
        if prediction == 1:
            return None  # inlier

        severity = self._determine_severity(score)
        feat_dict = dict(zip(_FEATURE_NAMES, features.tolist(), strict=True))
        return AnomalyAlert(
            id=str(uuid.uuid4()),
            instrument_id=snapshot["instrument_id"],
            venue_id=snapshot["venue_id"],
            anomaly_score=score,
            severity=severity,
            features=feat_dict,
            description=self._describe_features(snapshot, features),
            timestamp=datetime.now(timezone.utc),
        )

    # ------------------------------------------------------------------
    # Feature engineering
    # ------------------------------------------------------------------

    def _extract_features(self, snapshot: dict) -> np.ndarray:
        """Produce a 5-element feature vector from snapshot + history."""
        vol = float(snapshot["volume"])
        price = float(snapshot["price"])
        spread = float(snapshot["ask"]) - float(snapshot["bid"])

        vol_z = self._zscore(vol, self._volumes)
        spread_z = self._zscore(spread, self._spreads)

        # Price return z-score
        if len(self._prices) >= 2:
            returns = np.diff(list(self._prices))
            curr_return = price - list(self._prices)[-2] if len(self._prices) >= 2 else 0.0
            mean_r, std_r = float(np.mean(returns)), float(np.std(returns))
            price_ret_z = (curr_return - mean_r) / std_r if std_r > 1e-12 else 0.0
        else:
            price_ret_z = 0.0

        # Volume / price rolling correlation
        if len(self._volumes) >= 10:
            recent_v = np.array(list(self._volumes)[-30:])
            recent_p = np.array(list(self._prices)[-30:])
            corr_matrix = np.corrcoef(recent_v, recent_p)
            vp_corr = float(corr_matrix[0, 1]) if np.isfinite(corr_matrix[0, 1]) else 0.0
        else:
            vp_corr = 0.0

        cross_venue_div = 0.0  # placeholder

        return np.array(
            [vol_z, price_ret_z, spread_z, vp_corr, cross_venue_div],
            dtype=np.float64,
        )

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    @staticmethod
    def _zscore(value: float, history: deque[float]) -> float:
        if len(history) < 2:
            return 0.0
        arr = np.array(history)
        mean, std = float(np.mean(arr)), float(np.std(arr))
        return (value - mean) / std if std > 1e-12 else 0.0

    def _should_retrain(self) -> bool:
        if not self._trained and len(self.feature_window) >= 100:
            return True
        if self._last_retrain is None:
            return False
        elapsed = (datetime.now(timezone.utc) - self._last_retrain).total_seconds()
        return elapsed >= self._retrain_minutes * 60

    @staticmethod
    def _determine_severity(score: float) -> str:
        if score < -0.7:
            return "critical"
        if score < -0.5:
            return "warning"
        if score < -0.3:
            return "info"
        return "info"

    @staticmethod
    def _describe_features(snapshot: dict, features: np.ndarray) -> str:
        inst = snapshot["instrument_id"]
        venue = snapshot["venue_id"]
        vol_z = features[0]
        parts = [f"{inst} on {venue}"]
        if abs(vol_z) > 1.0:
            parts.append(f"volume {abs(vol_z):.1f}x above rolling mean")
        return " ".join(parts)
