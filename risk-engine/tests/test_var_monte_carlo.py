"""Tests for Monte Carlo VaR with correlated paths and fat-tailed distributions."""

from __future__ import annotations

from decimal import Decimal
from math import sqrt

import numpy as np
import pytest
from scipy.stats import norm

from risk_engine.domain.position import Position
from risk_engine.domain.risk_result import VaRResult
from risk_engine.var.monte_carlo import DistributionParams, MonteCarloVaR


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
# Tests — VaRResult structure and correctness
# ---------------------------------------------------------------------------


class TestMonteCarloVaRBasic:
    """Verify basic structure and sanity of the Monte Carlo VaR result."""

    def test_returns_var_result_with_correct_metadata(self) -> None:
        np.random.seed(42)
        positions = [_make_position("BTC-USD", Decimal("1"), Decimal("60000"), "crypto")]
        cov = np.array([[0.04**2]])
        mu = np.array([0.001])
        dist_params = [DistributionParams(family="normal")]

        engine = MonteCarloVaR(num_simulations=5_000, confidence=0.99)
        result = engine.compute(positions, cov, mu, dist_params)

        assert isinstance(result, VaRResult)
        assert result.method == "monte_carlo"
        assert result.confidence == 0.99
        assert result.horizon_days == 1
        assert result.var_amount > Decimal("0")
        assert result.cvar_amount >= result.var_amount
        assert result.computed_at is not None

    def test_var_is_positive_and_reasonable(self) -> None:
        np.random.seed(42)
        positions = [_make_position("AAPL", Decimal("100"), Decimal("180"), "equity")]
        cov = np.array([[0.015**2]])
        mu = np.array([0.0005])
        dist_params = [DistributionParams(family="normal")]

        engine = MonteCarloVaR(num_simulations=10_000, confidence=0.99)
        result = engine.compute(positions, cov, mu, dist_params)

        portfolio_value = float(Decimal("100") * Decimal("180"))
        assert float(result.var_amount) > 0
        assert float(result.var_amount) < portfolio_value  # VaR < 100% of portfolio


class TestDistributionArrayLength:
    """The returned distribution array must have length == num_simulations."""

    def test_distribution_length_equals_num_simulations(self) -> None:
        np.random.seed(42)
        num_sims = 7_500
        positions = [_make_position("ETH-USD", Decimal("5"), Decimal("3400"), "crypto")]
        cov = np.array([[0.05**2]])
        mu = np.array([0.0012])
        dist_params = [DistributionParams(family="normal")]

        engine = MonteCarloVaR(num_simulations=num_sims, confidence=0.99)
        result = engine.compute(positions, cov, mu, dist_params)

        assert result.distribution is not None
        assert len(result.distribution) == num_sims


class TestKnownCovarianceVaRRange:
    """Given a known covariance matrix, VaR should fall within an expected range."""

    def test_var_within_expected_range(self) -> None:
        np.random.seed(42)
        daily_vol = 0.02
        portfolio_value = 10_000.0
        positions = [_make_position("A", Decimal("100"), Decimal("100"), "equity")]
        cov = np.array([[daily_vol**2]])
        mu = np.array([0.0])
        dist_params = [DistributionParams(family="normal")]

        engine = MonteCarloVaR(num_simulations=50_000, confidence=0.99)
        result = engine.compute(positions, cov, mu, dist_params)

        # Analytical 99% VaR for normal: z * sigma * V
        z_99 = norm.ppf(0.99)
        expected_var = z_99 * daily_vol * portfolio_value

        actual_var = float(result.var_amount)
        # Monte Carlo should be within 10% of analytical for large N
        assert abs(actual_var - expected_var) / expected_var < 0.10, (
            f"MC VaR ({actual_var:.2f}) not within 10% of analytical ({expected_var:.2f})"
        )


class TestSinglePositionMatchesAnalytical:
    """Single normal-distributed position MC VaR should match parametric VaR."""

    def test_single_position_matches_analytical(self) -> None:
        np.random.seed(123)
        daily_vol = 0.03
        market_price = Decimal("500")
        quantity = Decimal("20")
        portfolio_value = float(quantity * market_price)

        positions = [_make_position("TOKEN_X", quantity, market_price, "crypto")]
        cov = np.array([[daily_vol**2]])
        mu = np.array([0.0])
        dist_params = [DistributionParams(family="normal")]

        engine = MonteCarloVaR(num_simulations=100_000, confidence=0.99)
        result = engine.compute(positions, cov, mu, dist_params)

        z_99 = norm.ppf(0.99)
        expected_var = z_99 * daily_vol * portfolio_value

        actual_var = float(result.var_amount)
        # 5% tolerance with 100K simulations
        assert abs(actual_var - expected_var) / expected_var < 0.05, (
            f"MC VaR ({actual_var:.2f}) not within 5% of analytical ({expected_var:.2f})"
        )


class TestStudentTFatterTails:
    """Student-t distribution should produce larger VaR than normal (fatter tails)."""

    def test_student_t_var_larger_than_normal(self) -> None:
        np.random.seed(42)
        positions = [_make_position("BTC-USD", Decimal("1"), Decimal("60000"), "crypto")]
        cov = np.array([[0.04**2]])
        mu = np.array([0.0])

        normal_params = [DistributionParams(family="normal")]
        student_t_params = [DistributionParams(family="student_t", df=3.0)]

        engine = MonteCarloVaR(num_simulations=100_000, confidence=0.99)

        np.random.seed(42)
        normal_result = engine.compute(positions, cov, mu, normal_params)

        np.random.seed(42)
        student_t_result = engine.compute(positions, cov, mu, student_t_params)

        normal_var = float(normal_result.var_amount)
        student_t_var = float(student_t_result.var_amount)

        assert student_t_var > normal_var, (
            f"Student-t VaR ({student_t_var:.2f}) should be > Normal VaR ({normal_var:.2f})"
        )


class TestCorrelatedMultiAssetPortfolio:
    """Multi-asset portfolio with correlation produces diversification benefit."""

    def test_diversification_benefit(self) -> None:
        np.random.seed(42)
        # Two assets with low correlation
        vol_a, vol_b = 0.02, 0.03
        corr = 0.2
        cov = np.array([
            [vol_a**2, corr * vol_a * vol_b],
            [corr * vol_a * vol_b, vol_b**2],
        ])
        mu = np.array([0.0, 0.0])

        positions = [
            _make_position("A", Decimal("100"), Decimal("100"), "equity"),
            _make_position("B", Decimal("50"), Decimal("200"), "crypto"),
        ]
        dist_params = [
            DistributionParams(family="normal"),
            DistributionParams(family="normal"),
        ]

        engine = MonteCarloVaR(num_simulations=50_000, confidence=0.99)

        # Portfolio VaR
        portfolio_result = engine.compute(positions, cov, mu, dist_params)

        # Individual VaRs
        np.random.seed(42)
        var_a = engine.compute(
            [positions[0]],
            np.array([[vol_a**2]]),
            np.array([0.0]),
            [dist_params[0]],
        )
        np.random.seed(42)
        var_b = engine.compute(
            [positions[1]],
            np.array([[vol_b**2]]),
            np.array([0.0]),
            [dist_params[1]],
        )

        sum_individual = float(var_a.var_amount) + float(var_b.var_amount)
        portfolio_var = float(portfolio_result.var_amount)

        assert portfolio_var < sum_individual, (
            f"Portfolio VaR ({portfolio_var:.2f}) should be < sum of individual "
            f"VaRs ({sum_individual:.2f}) due to diversification"
        )


class TestCVaRGreaterThanVaR:
    """CVaR (expected shortfall) should always be >= VaR."""

    def test_cvar_gte_var(self) -> None:
        np.random.seed(42)
        positions = [_make_position("BTC-USD", Decimal("1"), Decimal("60000"), "crypto")]
        cov = np.array([[0.04**2]])
        mu = np.array([0.001])
        dist_params = [DistributionParams(family="student_t", df=5.0)]

        engine = MonteCarloVaR(num_simulations=50_000, confidence=0.99)
        result = engine.compute(positions, cov, mu, dist_params)

        assert result.cvar_amount >= result.var_amount


class TestMultiDayHorizon:
    """Multi-day horizon should scale VaR appropriately (roughly sqrt(T) for normal)."""

    def test_multi_day_horizon_scaling(self) -> None:
        np.random.seed(42)
        positions = [_make_position("A", Decimal("100"), Decimal("100"), "equity")]
        cov = np.array([[0.02**2]])
        mu = np.array([0.0])
        dist_params = [DistributionParams(family="normal")]

        engine_1d = MonteCarloVaR(num_simulations=100_000, horizon_days=1, confidence=0.99)
        engine_10d = MonteCarloVaR(num_simulations=100_000, horizon_days=10, confidence=0.99)

        np.random.seed(42)
        result_1d = engine_1d.compute(positions, cov, mu, dist_params)
        np.random.seed(42)
        result_10d = engine_10d.compute(positions, cov, mu, dist_params)

        var_1d = float(result_1d.var_amount)
        var_10d = float(result_10d.var_amount)

        # 10-day VaR should be roughly sqrt(10) * 1-day VaR
        expected_ratio = sqrt(10)
        actual_ratio = var_10d / var_1d

        assert 0.7 * expected_ratio < actual_ratio < 1.3 * expected_ratio, (
            f"10d/1d VaR ratio ({actual_ratio:.2f}) not near sqrt(10) ({expected_ratio:.2f})"
        )


class TestMixedDistributions:
    """Portfolio with mixed normal (equity) and Student-t (crypto) distributions."""

    def test_mixed_distributions_compute(self) -> None:
        np.random.seed(42)
        positions = [
            _make_position("AAPL", Decimal("100"), Decimal("180"), "equity"),
            _make_position("BTC-USD", Decimal("0.5"), Decimal("65000"), "crypto"),
        ]
        cov = np.array([
            [0.015**2, 0.3 * 0.015 * 0.04],
            [0.3 * 0.015 * 0.04, 0.04**2],
        ])
        mu = np.array([0.0005, 0.001])
        dist_params = [
            DistributionParams(family="normal"),  # equity
            DistributionParams(family="student_t", df=5.0),  # crypto
        ]

        engine = MonteCarloVaR(num_simulations=10_000, confidence=0.99)
        result = engine.compute(positions, cov, mu, dist_params)

        assert isinstance(result, VaRResult)
        assert result.var_amount > Decimal("0")
        assert result.cvar_amount >= result.var_amount
        assert result.distribution is not None
        assert len(result.distribution) == 10_000


class TestDefaultParameters:
    """Verify default constructor parameters."""

    def test_defaults(self) -> None:
        engine = MonteCarloVaR()
        assert engine.num_simulations == 10_000
        assert engine.horizon_days == 1
        assert engine.confidence == 0.99


class TestDistributionParamsDataclass:
    """Verify DistributionParams defaults and construction."""

    def test_normal_defaults(self) -> None:
        p = DistributionParams(family="normal")
        assert p.family == "normal"
        assert p.df is None

    def test_student_t(self) -> None:
        p = DistributionParams(family="student_t", df=5.0)
        assert p.family == "student_t"
        assert p.df == 5.0
