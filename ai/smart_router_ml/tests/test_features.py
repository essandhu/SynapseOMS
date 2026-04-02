"""Tests for the feature engineering pipeline."""

import math

import numpy as np
import pytest

from smart_router_ml.features import extract_features, FEATURE_COLUMNS


def _make_candidate(overrides: dict | None = None) -> dict:
    base = {
        "venue_id": "venue_a",
        "order_size_pct_adv": 0.05,
        "spread_bps": 1.2,
        "book_depth_at_price": 50000.0,
        "venue_fill_rate_30d": 0.92,
        "venue_latency_p50": 0.5,
        "cross_venue_price_diff": 0.3,
        "hour_of_day": 14.0,
        "instrument_volatility": 0.02,
        "maker_taker_fee": 0.8,
        "time_since_last_fill": 120.0,
    }
    if overrides:
        base.update(overrides)
    return base


class TestExtractFeatures:
    """Tests for extract_features producing correct shape and values."""

    def test_single_candidate_produces_correct_shape(self):
        candidates = [_make_candidate()]
        matrix = extract_features(candidates)
        # 1 row, 11 columns (hour_of_day becomes sin + cos = 2 cols, other 9 stay)
        assert matrix.shape == (1, 11)

    def test_multiple_candidates_produces_correct_rows(self):
        candidates = [_make_candidate(), _make_candidate({"venue_id": "venue_b"})]
        matrix = extract_features(candidates)
        assert matrix.shape == (2, 11)

    def test_hour_of_day_sin_cos_encoding(self):
        candidates = [_make_candidate({"hour_of_day": 6.0})]
        matrix = extract_features(candidates)
        col_names = FEATURE_COLUMNS
        sin_idx = col_names.index("hour_sin")
        cos_idx = col_names.index("hour_cos")
        expected_sin = math.sin(2 * math.pi * 6.0 / 24.0)
        expected_cos = math.cos(2 * math.pi * 6.0 / 24.0)
        assert pytest.approx(matrix[0, sin_idx], abs=1e-6) == expected_sin
        assert pytest.approx(matrix[0, cos_idx], abs=1e-6) == expected_cos

    def test_feature_columns_list_matches_output_width(self):
        candidates = [_make_candidate()]
        matrix = extract_features(candidates)
        assert len(FEATURE_COLUMNS) == matrix.shape[1]

    def test_numeric_values_preserved(self):
        candidates = [_make_candidate({"spread_bps": 3.5})]
        matrix = extract_features(candidates)
        spread_idx = FEATURE_COLUMNS.index("spread_bps")
        assert pytest.approx(matrix[0, spread_idx]) == 3.5

    def test_empty_candidates_raises(self):
        with pytest.raises(ValueError, match="at least one candidate"):
            extract_features([])
