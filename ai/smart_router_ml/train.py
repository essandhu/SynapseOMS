"""ML training script for the smart-order-router XGBoost model.

Loads historical fill data (from PostgreSQL or CSV), joins with market
context, computes execution-quality target, trains an XGBoost regressor,
and saves the model as native JSON for hot-swap by the scoring sidecar.

CLI usage::

    python -m smart_router_ml.train --csv fills.csv --output model.json
    python -m smart_router_ml.train --db-url postgresql://... --output model.json
"""

from __future__ import annotations

import argparse
import logging
import math
import sys
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd
import xgboost as xgb

from smart_router_ml.features import FEATURE_COLUMNS

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# SQL query for loading fills with market context from PostgreSQL
# ---------------------------------------------------------------------------
_FILLS_QUERY = """
SELECT
    order_size_pct_adv,
    spread_bps,
    book_depth_at_price,
    venue_fill_rate_30d,
    venue_latency_p50,
    cross_venue_price_diff,
    hour_of_day,
    instrument_volatility,
    maker_taker_fee,
    time_since_last_fill,
    slippage_bps,
    fee_bps,
    latency_penalty_bps
FROM fills_with_market_context
"""


# ---------------------------------------------------------------------------
# Data loading
# ---------------------------------------------------------------------------

def load_fills_from_csv(path: str) -> pd.DataFrame:
    """Load historical fills from a CSV file."""
    return pd.read_csv(path)


def load_fills_from_db(db_url: str, query: str | None = None) -> pd.DataFrame:
    """Load historical fills from a PostgreSQL database.

    Parameters
    ----------
    db_url:
        SQLAlchemy-compatible connection string.
    query:
        Optional custom SQL query.  Defaults to *_FILLS_QUERY*.
    """
    try:
        import sqlalchemy  # noqa: F401
    except ImportError:
        logger.error("sqlalchemy is required for DB loading: pip install sqlalchemy psycopg2-binary")
        raise

    from sqlalchemy import create_engine, text

    engine = create_engine(db_url)
    sql = query or _FILLS_QUERY
    with engine.connect() as conn:
        df = pd.read_sql(text(sql), conn)
    return df


# ---------------------------------------------------------------------------
# Feature / target construction
# ---------------------------------------------------------------------------

def compute_target(df: pd.DataFrame) -> np.ndarray:
    """Compute execution quality target variable.

    target = -1 * (slippage_bps + fee_bps + latency_penalty_bps)

    Higher values indicate better execution quality.
    """
    return -1.0 * (
        df["slippage_bps"].values
        + df["fee_bps"].values
        + df["latency_penalty_bps"].values
    )


def build_training_dataframe(
    df: pd.DataFrame,
) -> tuple[pd.DataFrame, np.ndarray]:
    """Build feature matrix and target from raw fill data.

    Applies cyclic hour encoding (sin/cos) consistent with
    ``features.extract_features``.

    Returns
    -------
    (X, y) where X is a DataFrame with FEATURE_COLUMNS and y is an ndarray.
    """
    hour = df["hour_of_day"].astype(float)
    hour_sin = np.sin(2.0 * np.pi * hour / 24.0)
    hour_cos = np.cos(2.0 * np.pi * hour / 24.0)

    X = pd.DataFrame(
        {
            "order_size_pct_adv": df["order_size_pct_adv"].astype(float),
            "spread_bps": df["spread_bps"].astype(float),
            "book_depth_at_price": df["book_depth_at_price"].astype(float),
            "venue_fill_rate_30d": df["venue_fill_rate_30d"].astype(float),
            "venue_latency_p50": df["venue_latency_p50"].astype(float),
            "cross_venue_price_diff": df["cross_venue_price_diff"].astype(float),
            "hour_sin": hour_sin.values,
            "hour_cos": hour_cos.values,
            "instrument_volatility": df["instrument_volatility"].astype(float),
            "maker_taker_fee": df["maker_taker_fee"].astype(float),
            "time_since_last_fill": df["time_since_last_fill"].astype(float),
        }
    )

    y = compute_target(df)
    return X, y


# ---------------------------------------------------------------------------
# Training
# ---------------------------------------------------------------------------

def train_model(
    df: pd.DataFrame,
    *,
    output_path: str = "model.json",
    n_estimators: int = 100,
    max_depth: int = 6,
    learning_rate: float = 0.1,
    test_size: float = 0.2,
    random_state: int = 42,
) -> dict[str, Any]:
    """Train an XGBoost model on fill data and save to disk.

    Parameters
    ----------
    df:
        DataFrame of historical fills with feature and target columns.
    output_path:
        Path to save the trained model (XGBoost native JSON format).
    n_estimators:
        Number of boosting rounds.
    max_depth:
        Maximum tree depth.
    learning_rate:
        Boosting learning rate (eta).
    test_size:
        Fraction of data to hold out for evaluation.
    random_state:
        Random seed for reproducibility.

    Returns
    -------
    Dict with ``rmse`` and ``feature_importance`` keys.
    """
    X, y = build_training_dataframe(df)

    # Train/test split
    n = len(X)
    rng = np.random.default_rng(random_state)
    indices = rng.permutation(n)
    split = int(n * (1.0 - test_size))
    train_idx, test_idx = indices[:split], indices[split:]

    X_train, X_test = X.iloc[train_idx], X.iloc[test_idx]
    y_train, y_test = y[train_idx], y[test_idx]

    dtrain = xgb.DMatrix(X_train.values, label=y_train, feature_names=FEATURE_COLUMNS)
    dtest = xgb.DMatrix(X_test.values, label=y_test, feature_names=FEATURE_COLUMNS)

    params = {
        "max_depth": max_depth,
        "learning_rate": learning_rate,
        "objective": "reg:squarederror",
        "eval_metric": "rmse",
        "verbosity": 0,
    }

    evals_result: dict[str, Any] = {}
    booster = xgb.train(
        params,
        dtrain,
        num_boost_round=n_estimators,
        evals=[(dtrain, "train"), (dtest, "test")],
        evals_result=evals_result,
        verbose_eval=False,
    )

    # Save model
    Path(output_path).parent.mkdir(parents=True, exist_ok=True)
    booster.save_model(output_path)
    logger.info("Model saved to %s", output_path)

    # Compute final RMSE on test set
    preds = booster.predict(dtest)
    rmse = float(np.sqrt(np.mean((preds - y_test) ** 2)))
    logger.info("Test RMSE: %.6f", rmse)

    # Feature importance
    importance = booster.get_score(importance_type="weight")
    # Sort by importance descending
    sorted_importance = sorted(importance.items(), key=lambda kv: kv[1], reverse=True)

    logger.info("Feature importance ranking:")
    for fname, score in sorted_importance:
        logger.info("  %s: %.1f", fname, score)

    return {
        "rmse": rmse,
        "feature_importance": sorted_importance,
        "train_rmse_history": evals_result.get("train", {}).get("rmse", []),
        "test_rmse_history": evals_result.get("test", {}).get("rmse", []),
    }


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        prog="smart_router_ml.train",
        description="Train XGBoost model for smart order routing",
    )
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--csv", help="Path to CSV file of historical fills")
    source.add_argument("--db-url", help="PostgreSQL connection string")

    parser.add_argument("--query", default=None, help="Custom SQL query (with --db-url)")
    parser.add_argument("--output", default="model.json", help="Output model path (default: model.json)")
    parser.add_argument("--n-estimators", type=int, default=100, help="Number of boosting rounds")
    parser.add_argument("--max-depth", type=int, default=6, help="Maximum tree depth")
    parser.add_argument("--learning-rate", type=float, default=0.1, help="Learning rate (eta)")
    parser.add_argument("--test-size", type=float, default=0.2, help="Holdout fraction for evaluation")
    parser.add_argument("--random-state", type=int, default=42, help="Random seed")
    parser.add_argument("--verbose", action="store_true", help="Enable verbose logging")

    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> None:
    args = parse_args(argv)

    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )

    # Load data
    if args.csv:
        logger.info("Loading fills from CSV: %s", args.csv)
        df = load_fills_from_csv(args.csv)
    else:
        logger.info("Loading fills from DB: %s", args.db_url)
        df = load_fills_from_db(args.db_url, query=args.query)

    logger.info("Loaded %d fills", len(df))

    if len(df) < 10:
        logger.error("Too few fills (%d) to train a meaningful model", len(df))
        sys.exit(1)

    metrics = train_model(
        df,
        output_path=args.output,
        n_estimators=args.n_estimators,
        max_depth=args.max_depth,
        learning_rate=args.learning_rate,
        test_size=args.test_size,
        random_state=args.random_state,
    )

    logger.info("Training complete — RMSE: %.6f", metrics["rmse"])


if __name__ == "__main__":
    main()
