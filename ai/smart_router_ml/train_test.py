"""Tests for the ML training script (P3-07).

Uses synthetic fill data so no real DB or CSV is needed.
"""

from __future__ import annotations

import json
import math
import tempfile
from pathlib import Path

import numpy as np
import pandas as pd
import pytest
import xgboost as xgb

from smart_router_ml.train import (
    build_training_dataframe,
    compute_target,
    load_fills_from_csv,
    train_model,
    FEATURE_COLUMNS,
)


# ---------------------------------------------------------------------------
# Helpers – synthetic data generation
# ---------------------------------------------------------------------------

def _make_synthetic_fills(n: int = 200, seed: int = 42) -> pd.DataFrame:
    """Generate *n* synthetic fill rows with all required columns."""
    rng = np.random.default_rng(seed)
    return pd.DataFrame(
        {
            "order_size_pct_adv": rng.uniform(0.01, 0.5, n),
            "spread_bps": rng.uniform(0.5, 10.0, n),
            "book_depth_at_price": rng.uniform(100, 50_000, n),
            "venue_fill_rate_30d": rng.uniform(0.4, 0.99, n),
            "venue_latency_p50": rng.uniform(0.5, 50.0, n),
            "cross_venue_price_diff": rng.uniform(-5.0, 5.0, n),
            "hour_of_day": rng.uniform(0, 24, n),
            "instrument_volatility": rng.uniform(0.005, 0.08, n),
            "maker_taker_fee": rng.uniform(-0.5, 1.5, n),
            "time_since_last_fill": rng.uniform(0.0, 300.0, n),
            # columns needed for target computation
            "slippage_bps": rng.uniform(-1.0, 5.0, n),
            "fee_bps": rng.uniform(0.1, 2.0, n),
            "latency_penalty_bps": rng.uniform(0.0, 3.0, n),
        }
    )


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestComputeTarget:
    """Target = -1 * (slippage_bps + fee_bps + latency_penalty_bps)."""

    def test_basic(self):
        df = pd.DataFrame(
            {
                "slippage_bps": [1.0, 2.0],
                "fee_bps": [0.5, 1.0],
                "latency_penalty_bps": [0.2, 0.3],
            }
        )
        result = compute_target(df)
        np.testing.assert_allclose(result, [-1.7, -3.3])

    def test_shape(self):
        df = _make_synthetic_fills(50)
        result = compute_target(df)
        assert result.shape == (50,)


class TestBuildTrainingDataframe:
    """Feature matrix + target from raw fills."""

    def test_columns(self):
        df = _make_synthetic_fills(30)
        X, y = build_training_dataframe(df)
        assert list(X.columns) == FEATURE_COLUMNS
        assert X.shape == (30, len(FEATURE_COLUMNS))
        assert y.shape == (30,)

    def test_hour_encoding(self):
        """hour_of_day=6 should give sin/cos of 2*pi*6/24."""
        df = _make_synthetic_fills(1)
        df.loc[0, "hour_of_day"] = 6.0
        X, _ = build_training_dataframe(df)
        expected_sin = math.sin(2 * math.pi * 6.0 / 24.0)
        expected_cos = math.cos(2 * math.pi * 6.0 / 24.0)
        assert abs(X["hour_sin"].iloc[0] - expected_sin) < 1e-9
        assert abs(X["hour_cos"].iloc[0] - expected_cos) < 1e-9


class TestLoadFillsFromCSV:
    """CSV loading round-trip."""

    def test_round_trip(self, tmp_path: Path):
        df = _make_synthetic_fills(10)
        csv_path = tmp_path / "fills.csv"
        df.to_csv(csv_path, index=False)

        loaded = load_fills_from_csv(str(csv_path))
        assert loaded.shape == (10, df.shape[1])
        pd.testing.assert_frame_equal(loaded, df)


class TestTrainModel:
    """End-to-end training produces a valid XGBoost model."""

    def test_trains_without_error(self, tmp_path: Path):
        df = _make_synthetic_fills(200)
        output_path = tmp_path / "model.json"
        metrics = train_model(
            df,
            output_path=str(output_path),
            n_estimators=20,
            max_depth=3,
            learning_rate=0.1,
        )
        assert output_path.exists()
        assert "rmse" in metrics
        assert metrics["rmse"] >= 0.0

    def test_feature_importance_non_empty(self, tmp_path: Path):
        df = _make_synthetic_fills(200)
        output_path = tmp_path / "model.json"
        metrics = train_model(df, output_path=str(output_path), n_estimators=20)
        assert "feature_importance" in metrics
        assert len(metrics["feature_importance"]) > 0

    def test_saved_model_is_valid_xgboost(self, tmp_path: Path):
        df = _make_synthetic_fills(200)
        output_path = tmp_path / "model.json"
        train_model(df, output_path=str(output_path), n_estimators=20)

        # Reload and predict
        booster = xgb.Booster()
        booster.load_model(str(output_path))
        X, _ = build_training_dataframe(df.head(5))
        dmat = xgb.DMatrix(X.values, feature_names=FEATURE_COLUMNS)
        preds = booster.predict(dmat)
        assert preds.shape == (5,)

    def test_model_json_parseable(self, tmp_path: Path):
        """The .json model file should be loadable by XGBoost (native JSON format)."""
        df = _make_synthetic_fills(100)
        output_path = tmp_path / "model.json"
        train_model(df, output_path=str(output_path), n_estimators=10)
        # XGBoost JSON models start with a JSON structure
        content = output_path.read_text()
        parsed = json.loads(content)
        assert "learner" in parsed

    def test_custom_hyperparams(self, tmp_path: Path):
        df = _make_synthetic_fills(150)
        output_path = tmp_path / "model.json"
        metrics = train_model(
            df,
            output_path=str(output_path),
            n_estimators=50,
            max_depth=5,
            learning_rate=0.05,
        )
        assert output_path.exists()
        assert metrics["rmse"] >= 0.0
