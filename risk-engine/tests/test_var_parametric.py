"""Tests for Parametric VaR (variance-covariance method)."""

from __future__ import annotations

from decimal import Decimal

import numpy as np
import pandas as pd
import pytest
from scipy.stats import norm

from risk_engine.domain.position import Position
from risk_engine.domain.risk_result import VaRResult
from risk_engine.var.parametric import ParametricVaR
from risk_engine.var.historical import HistoricalVaR


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_position(
    instrument_id: str,
    quantity: Decimal,
    market_price: Decimal,
    asset_class: str = "crypto",
) -> Position:
    return Position(
        instrument_id=instrument_id,
        venue_id="TEST",
        quantity=quantity,
        average_cost=market_price,
        market_price=market_price,
        unrealized_pnl=Decimal("0"),
        realized_pnl=Decimal("0"),
        asset_class=asset_class,
        settlement_cycle="T0" if asset_class == "crypto" else "T2",
    )


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestParametricVaRBasic:
    """test_parametric_var_basic — compute parametric VaR, verify positive and reasonable."""

    def test_parametric_var_basic(
        self,
        sample_positions: dict[str, Position],
        sample_returns_matrix: pd.DataFrame,
    ) -> None:
        engine = ParametricVaR(confidence=0.99, use_shrinkage=True)
        result = engine.compute(sample_positions, sample_returns_matrix)

        assert isinstance(result, VaRResult)
        assert result.var_amount > Decimal("0")
        assert result.cvar_amount > Decimal("0")
        assert result.confidence == 0.99
        assert result.horizon_days == 1
        assert result.method == "parametric"
        # VaR should be a reasonable fraction of the total portfolio value
        total_value = sum(
            float(p.market_value) for p in sample_positions.values()
        )
        assert float(result.var_amount) < total_value  # VaR < 100% of portfolio


class TestParametricCloseToHistoricalForNormal:
    """For normally distributed returns, parametric and historical VaR should
    be relatively close (within 50% of each other)."""

    def test_parametric_close_to_historical_for_normal(self) -> None:
        # Generate normally distributed returns (large sample for convergence)
        rng = np.random.default_rng(seed=99)
        n_days = 5000
        vol = 0.02
        returns = pd.DataFrame({
            "ASSET_A": rng.normal(0, vol, n_days),
        })

        positions = {
            "ASSET_A": _make_position("ASSET_A", Decimal("100"), Decimal("100")),
        }

        parametric = ParametricVaR(confidence=0.99, use_shrinkage=False)
        historical = HistoricalVaR(window_days=n_days, confidence=0.99)

        p_result = parametric.compute(positions, returns)
        h_result = historical.compute(positions, returns)

        p_var = float(p_result.var_amount)
        h_var = float(h_result.var_amount)

        # They should be within 50% of each other
        ratio = p_var / h_var if h_var > 0 else float("inf")
        assert 0.5 < ratio < 1.5, (
            f"Parametric VaR ({p_var:.2f}) and Historical VaR ({h_var:.2f}) "
            f"diverged too much (ratio={ratio:.3f})"
        )


class TestSingleAssetPortfolio:
    """Single asset VaR = z * sigma * V (simple formula)."""

    def test_single_asset_portfolio(self) -> None:
        rng = np.random.default_rng(seed=123)
        n_days = 1000
        daily_vol = 0.03
        returns = pd.DataFrame({
            "TOKEN_X": rng.normal(0, daily_vol, n_days),
        })

        market_price = Decimal("500")
        quantity = Decimal("20")
        positions = {
            "TOKEN_X": _make_position("TOKEN_X", quantity, market_price),
        }

        confidence = 0.99
        engine = ParametricVaR(confidence=confidence, use_shrinkage=False)
        result = engine.compute(positions, returns)

        # Expected: z_alpha * sample_std * portfolio_value
        z_alpha = norm.ppf(confidence)
        sample_std = float(returns["TOKEN_X"].std(ddof=0))
        portfolio_value = float(quantity * market_price)
        expected_var = z_alpha * sample_std * portfolio_value

        actual_var = float(result.var_amount)
        # Allow 5% tolerance (sample covariance vs population)
        assert abs(actual_var - expected_var) / expected_var < 0.05, (
            f"Single-asset VaR ({actual_var:.2f}) != expected ({expected_var:.2f})"
        )


class TestZeroCorrelationPortfolio:
    """Uncorrelated assets: portfolio VaR < sum of individual VaRs
    (diversification benefit)."""

    def test_zero_correlation_portfolio(self) -> None:
        rng = np.random.default_rng(seed=456)
        n_days = 2000

        # Generate independent (uncorrelated) returns
        returns = pd.DataFrame({
            "ASSET_1": rng.normal(0, 0.02, n_days),
            "ASSET_2": rng.normal(0, 0.03, n_days),
        })

        positions = {
            "ASSET_1": _make_position("ASSET_1", Decimal("50"), Decimal("200")),
            "ASSET_2": _make_position("ASSET_2", Decimal("30"), Decimal("300")),
        }

        engine = ParametricVaR(confidence=0.99, use_shrinkage=False)

        # Portfolio VaR
        portfolio_result = engine.compute(positions, returns)

        # Individual VaRs
        var_1 = engine.compute(
            {"ASSET_1": positions["ASSET_1"]},
            returns[["ASSET_1"]],
        )
        var_2 = engine.compute(
            {"ASSET_2": positions["ASSET_2"]},
            returns[["ASSET_2"]],
        )

        sum_individual = float(var_1.var_amount) + float(var_2.var_amount)
        portfolio_var = float(portfolio_result.var_amount)

        assert portfolio_var < sum_individual, (
            f"Portfolio VaR ({portfolio_var:.2f}) should be less than "
            f"sum of individual VaRs ({sum_individual:.2f}) due to diversification"
        )


class TestEmptyPortfolioZeroVaR:
    """Empty positions returns zero VaR."""

    def test_empty_portfolio_zero_var(self, sample_returns_matrix: pd.DataFrame) -> None:
        engine = ParametricVaR(confidence=0.99)
        result = engine.compute({}, sample_returns_matrix)

        assert result.var_amount == Decimal("0")
        assert result.cvar_amount == Decimal("0")
        assert result.method == "parametric"
        assert result.confidence == 0.99


class TestCVaRGreaterThanVaR:
    """CVaR >= VaR always."""

    def test_cvar_greater_than_var(
        self,
        sample_positions: dict[str, Position],
        sample_returns_matrix: pd.DataFrame,
    ) -> None:
        engine = ParametricVaR(confidence=0.99)
        result = engine.compute(sample_positions, sample_returns_matrix)

        assert result.cvar_amount >= result.var_amount, (
            f"CVaR ({result.cvar_amount}) should be >= VaR ({result.var_amount})"
        )

    def test_cvar_greater_than_var_at_95(
        self,
        sample_positions: dict[str, Position],
        sample_returns_matrix: pd.DataFrame,
    ) -> None:
        engine = ParametricVaR(confidence=0.95)
        result = engine.compute(sample_positions, sample_returns_matrix)

        assert result.cvar_amount >= result.var_amount
