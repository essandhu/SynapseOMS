"""Tests for market regime detection."""
from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from risk_engine.timeseries.regime import RegimeConfig, RegimeDetector, RegimeState


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _steady_returns(mean: float, std: float, n: int = 60) -> pd.Series:
    """Generate a series of returns with given mean and low noise."""
    rng = np.random.default_rng(42)
    return pd.Series(rng.normal(mean, std, n))


def _volatile_returns(n: int = 60) -> pd.Series:
    """Generate a highly volatile return series (annualized vol >> 30%)."""
    rng = np.random.default_rng(99)
    # Daily std ~0.04 => annualized ~0.04*sqrt(252) ~ 63%
    return pd.Series(rng.normal(0.0, 0.04, n))


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

class TestDetectRegime:
    def test_detect_regime_trending_up(self) -> None:
        """Steady positive returns with low vol should be classified as BULL."""
        returns = _steady_returns(mean=0.002, std=0.005, n=60)
        detector = RegimeDetector()
        assert detector.detect_regime(returns) == RegimeState.BULL

    def test_detect_regime_trending_down(self) -> None:
        """Steady negative returns should be classified as BEAR."""
        returns = _steady_returns(mean=-0.005, std=0.005, n=60)
        detector = RegimeDetector()
        assert detector.detect_regime(returns) == RegimeState.BEAR

    def test_detect_regime_high_volatility(self) -> None:
        """Large daily swings should be classified as CRISIS."""
        returns = _volatile_returns(n=60)
        detector = RegimeDetector()
        assert detector.detect_regime(returns) == RegimeState.CRISIS

    def test_regime_history_length_matches_input(self) -> None:
        """regime_history output length must equal input length."""
        returns = _steady_returns(mean=0.001, std=0.005, n=40)
        detector = RegimeDetector()
        history = detector.regime_history(returns)
        assert len(history) == len(returns)

    def test_regime_transition_bull_to_bear(self) -> None:
        """A series that starts positive then turns negative should transition."""
        rng = np.random.default_rng(7)
        bull_part = pd.Series(rng.normal(0.003, 0.003, 30))
        bear_part = pd.Series(rng.normal(-0.006, 0.003, 30))
        returns = pd.concat([bull_part, bear_part], ignore_index=True)

        detector = RegimeDetector()
        history = detector.regime_history(returns)

        # Early part should contain BULL, later part should contain BEAR
        early = set(history.iloc[:25])
        late = set(history.iloc[-10:])
        assert RegimeState.BULL.value in early
        assert RegimeState.BEAR.value in late

    def test_empty_series_returns_default(self) -> None:
        """Empty series should default to BULL."""
        detector = RegimeDetector()
        assert detector.detect_regime(pd.Series(dtype=float)) == RegimeState.BULL

    def test_short_series_returns_bull(self) -> None:
        """Single observation should default to BULL."""
        detector = RegimeDetector()
        assert detector.detect_regime(pd.Series([0.01])) == RegimeState.BULL

    def test_risk_multiplier_bull(self) -> None:
        """BULL regime should return multiplier 1.0."""
        returns = _steady_returns(mean=0.002, std=0.005, n=60)
        detector = RegimeDetector()
        assert detector.risk_multiplier(returns) == pytest.approx(1.0)

    def test_risk_multiplier_crisis(self) -> None:
        """CRISIS regime should return multiplier 1.8."""
        returns = _volatile_returns(n=60)
        detector = RegimeDetector()
        assert detector.risk_multiplier(returns) == pytest.approx(1.8)

    def test_custom_config(self) -> None:
        """Custom thresholds should be respected."""
        config = RegimeConfig(
            crisis_vol_threshold=0.10,  # very low threshold
            bear_return_threshold=-0.001,
            risk_multipliers={"bull": 1.0, "bear": 1.5, "crisis": 2.0},
        )
        detector = RegimeDetector(config)
        # With a very low crisis threshold, even moderate vol triggers CRISIS
        returns = _steady_returns(mean=0.0, std=0.01, n=60)
        regime = detector.detect_regime(returns)
        assert regime == RegimeState.CRISIS
        assert config.multipliers[RegimeState.CRISIS] == 2.0
        assert config.multipliers[RegimeState.BEAR] == 1.5
