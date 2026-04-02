"""Feature engineering pipeline for the smart-router ML scorer.

Extracts and transforms the 10 raw venue features into an 11-column
numeric matrix suitable for XGBoost prediction.  The ``hour_of_day``
field is cyclically encoded as sin/cos, expanding 10 inputs to 11 columns.
"""

from __future__ import annotations

import math
from typing import Any

import numpy as np

# Ordered list of columns in the output matrix.  Consumers (model, tests)
# import this to know the column semantics.
FEATURE_COLUMNS: list[str] = [
    "order_size_pct_adv",
    "spread_bps",
    "book_depth_at_price",
    "venue_fill_rate_30d",
    "venue_latency_p50",
    "cross_venue_price_diff",
    "hour_sin",
    "hour_cos",
    "instrument_volatility",
    "maker_taker_fee",
    "time_since_last_fill",
]

# Raw input keys expected on each candidate dict (before encoding).
_RAW_KEYS: list[str] = [
    "order_size_pct_adv",
    "spread_bps",
    "book_depth_at_price",
    "venue_fill_rate_30d",
    "venue_latency_p50",
    "cross_venue_price_diff",
    "hour_of_day",
    "instrument_volatility",
    "maker_taker_fee",
    "time_since_last_fill",
]


def extract_features(candidates: list[dict[str, Any]]) -> np.ndarray:
    """Convert a list of candidate dicts into a (N, 11) float64 matrix.

    Parameters
    ----------
    candidates:
        Each dict must contain the 10 raw feature keys plus ``venue_id``.

    Returns
    -------
    np.ndarray of shape ``(len(candidates), 11)``

    Raises
    ------
    ValueError
        If *candidates* is empty.
    """
    if not candidates:
        raise ValueError("candidates must contain at least one candidate")

    rows: list[list[float]] = []
    for c in candidates:
        hour = float(c["hour_of_day"])
        hour_sin = math.sin(2.0 * math.pi * hour / 24.0)
        hour_cos = math.cos(2.0 * math.pi * hour / 24.0)

        row = [
            float(c["order_size_pct_adv"]),
            float(c["spread_bps"]),
            float(c["book_depth_at_price"]),
            float(c["venue_fill_rate_30d"]),
            float(c["venue_latency_p50"]),
            float(c["cross_venue_price_diff"]),
            hour_sin,
            hour_cos,
            float(c["instrument_volatility"]),
            float(c["maker_taker_fee"]),
            float(c["time_since_last_fill"]),
        ]
        rows.append(row)

    return np.array(rows, dtype=np.float64)
