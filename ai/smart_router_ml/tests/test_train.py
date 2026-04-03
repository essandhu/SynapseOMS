"""Unit tests for the XGBoost training script."""

from __future__ import annotations

import json
import os
from pathlib import Path

import numpy as np
import pandas as pd
import pytest
import xgboost as xgb

from smart_router_ml.train import (
    build_training_dataframe,
    compute_target,
    load_fills_from_csv,
    parse_args,
    train_model,
)
from smart_router_ml.features import FEATURE_COLUMNS


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

def _synthetic_fills(n: int = 200, seed: int = 42) -> pd.DataFrame:
    """Generate synthetic fill data with the expected columns."""
    rng = np.random.default_rng(seed)
    return pd.DataFrame(
        {
            "order_size_pct_adv": rng.uniform(0.01, 0.5, n),
            "spread_bps": rng.uniform(1, 50, n),
            "book_depth_at_price": rng.uniform(10, 500, n),
            "venue_fill_rate_30d": rng.uniform(0.5, 1.0, n),
            "venue_latency_p50": rng.uniform(1, 100, n),
            "cross_venue_price_diff": rng.uniform(-20, 20, n),
            "hour_of_day": rng.uniform(0, 24, n),
            "instrument_volatility": rng.uniform(0.01, 0.8, n),
            "maker_taker_fee": rng.uniform(0.0001, 0.003, n),
            "time_since_last_fill": rng.uniform(0, 3600, n),
            "slippage_bps": rng.uniform(0, 20, n),
            "fee_bps": rng.uniform(1, 10, n),
            "latency_penalty_bps": rng.uniform(0, 5, n),
        }
    )


@pytest.fixture
def synthetic_fills() -> pd.DataFrame:
    return _synthetic_fills()


# ---------------------------------------------------------------------------
# compute_target
# ---------------------------------------------------------------------------

class TestComputeTarget:
    def test_returns_correct_shape(self, synthetic_fills: pd.DataFrame) -> None:
        y = compute_target(synthetic_fills)
        assert y.shape == (len(synthetic_fills),)

    def test_values_are_negative(self, synthetic_fills: pd.DataFrame) -> None:
        """Target = -1 * (slippage + fee + latency) so all values should be <= 0."""
        y = compute_target(synthetic_fills)
        assert np.all(y <= 0)

    def test_known_values(self) -> None:
        df = pd.DataFrame(
            {
                "slippage_bps": [5.0],
                "fee_bps": [2.0],
                "latency_penalty_bps": [1.0],
            }
        )
        y = compute_target(df)
        assert y[0] == pytest.approx(-8.0)


# ---------------------------------------------------------------------------
# build_training_dataframe
# ---------------------------------------------------------------------------

class TestBuildTrainingDataframe:
    def test_feature_columns_match(self, synthetic_fills: pd.DataFrame) -> None:
        X, _ = build_training_dataframe(synthetic_fills)
        assert list(X.columns) == FEATURE_COLUMNS

    def test_shape(self, synthetic_fills: pd.DataFrame) -> None:
        X, y = build_training_dataframe(synthetic_fills)
        assert X.shape == (len(synthetic_fills), len(FEATURE_COLUMNS))
        assert y.shape == (len(synthetic_fills),)

    def test_cyclic_encoding_bounded(self, synthetic_fills: pd.DataFrame) -> None:
        X, _ = build_training_dataframe(synthetic_fills)
        assert X["hour_sin"].between(-1, 1).all()
        assert X["hour_cos"].between(-1, 1).all()


# ---------------------------------------------------------------------------
# train_model
# ---------------------------------------------------------------------------

class TestTrainModel:
    def test_synthetic_data_trains_without_error(
        self, synthetic_fills: pd.DataFrame, tmp_path: Path
    ) -> None:
        output = str(tmp_path / "model.json")
        metrics = train_model(
            synthetic_fills,
            output_path=output,
            n_estimators=10,
            max_depth=3,
            test_size=0.2,
        )
        assert "rmse" in metrics
        assert isinstance(metrics["rmse"], float)
        assert metrics["rmse"] >= 0

    def test_feature_importance_non_empty(
        self, synthetic_fills: pd.DataFrame, tmp_path: Path
    ) -> None:
        output = str(tmp_path / "model.json")
        metrics = train_model(
            synthetic_fills,
            output_path=output,
            n_estimators=10,
            max_depth=3,
        )
        assert len(metrics["feature_importance"]) > 0

    def test_model_file_is_valid_xgboost(
        self, synthetic_fills: pd.DataFrame, tmp_path: Path
    ) -> None:
        output = str(tmp_path / "model.json")
        train_model(
            synthetic_fills,
            output_path=output,
            n_estimators=10,
            max_depth=3,
        )
        assert Path(output).exists()
        # Verify it's loadable as an XGBoost model
        booster = xgb.Booster()
        booster.load_model(output)
        assert booster.num_features() == len(FEATURE_COLUMNS)

    def test_model_file_is_json_format(
        self, synthetic_fills: pd.DataFrame, tmp_path: Path
    ) -> None:
        output = str(tmp_path / "model.json")
        train_model(
            synthetic_fills,
            output_path=output,
            n_estimators=10,
            max_depth=3,
        )
        # XGBoost native JSON format should be parseable as JSON
        with open(output) as f:
            data = json.load(f)
        assert "learner" in data

    def test_creates_parent_directories(
        self, synthetic_fills: pd.DataFrame, tmp_path: Path
    ) -> None:
        output = str(tmp_path / "nested" / "dir" / "model.json")
        train_model(
            synthetic_fills,
            output_path=output,
            n_estimators=10,
            max_depth=3,
        )
        assert Path(output).exists()

    def test_rmse_history_returned(
        self, synthetic_fills: pd.DataFrame, tmp_path: Path
    ) -> None:
        output = str(tmp_path / "model.json")
        metrics = train_model(
            synthetic_fills,
            output_path=output,
            n_estimators=10,
            max_depth=3,
        )
        assert len(metrics["train_rmse_history"]) == 10
        assert len(metrics["test_rmse_history"]) == 10


# ---------------------------------------------------------------------------
# load_fills_from_csv
# ---------------------------------------------------------------------------

class TestLoadFillsFromCSV:
    def test_loads_csv_correctly(self, tmp_path: Path) -> None:
        df = _synthetic_fills(n=20)
        csv_path = str(tmp_path / "fills.csv")
        df.to_csv(csv_path, index=False)

        loaded = load_fills_from_csv(csv_path)
        assert len(loaded) == 20
        assert set(df.columns) == set(loaded.columns)


# ---------------------------------------------------------------------------
# parse_args
# ---------------------------------------------------------------------------

class TestParseArgs:
    def test_csv_source(self) -> None:
        args = parse_args(["--csv", "data.csv", "--output", "out.json"])
        assert args.csv == "data.csv"
        assert args.output == "out.json"
        assert args.db_url is None

    def test_db_source(self) -> None:
        args = parse_args(["--db-url", "postgresql://localhost/fills"])
        assert args.db_url == "postgresql://localhost/fills"
        assert args.csv is None

    def test_hyperparameters(self) -> None:
        args = parse_args([
            "--csv", "data.csv",
            "--n-estimators", "200",
            "--max-depth", "8",
            "--learning-rate", "0.05",
            "--test-size", "0.3",
            "--random-state", "99",
        ])
        assert args.n_estimators == 200
        assert args.max_depth == 8
        assert args.learning_rate == 0.05
        assert args.test_size == 0.3
        assert args.random_state == 99

    def test_csv_and_db_mutually_exclusive(self) -> None:
        with pytest.raises(SystemExit):
            parse_args(["--csv", "data.csv", "--db-url", "postgresql://localhost"])
