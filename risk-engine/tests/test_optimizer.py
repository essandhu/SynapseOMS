"""Tests for the portfolio construction optimizer (P3-12)."""

from __future__ import annotations

from decimal import Decimal

import numpy as np
import pytest

from risk_engine.domain.position import Position
from risk_engine.optimizer.constraints import OptimizationConstraints
from risk_engine.optimizer.mean_variance import (
    OptimizationResult,
    PortfolioOptimizer,
    TradeAction,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_positions(
    n: int = 3,
    asset_classes: list[str] | None = None,
) -> list[Position]:
    """Create *n* dummy positions with equal market value."""
    ids = [f"ASSET-{i}" for i in range(n)]
    classes = asset_classes or ["equity"] * n
    return [
        Position(
            instrument_id=ids[i],
            venue_id="TEST",
            quantity=Decimal("100"),
            average_cost=Decimal("10.00"),
            market_price=Decimal("10.00"),
            unrealized_pnl=Decimal("0"),
            realized_pnl=Decimal("0"),
            asset_class=classes[i],
            settlement_cycle="T2",
        )
        for i in range(n)
    ]


def _simple_cov(n: int = 3) -> np.ndarray:
    """Return a simple diagonal covariance matrix."""
    return np.diag([0.04, 0.16, 0.25][:n])


def _simple_expected_returns(n: int = 3) -> np.ndarray:
    return np.array([0.10, 0.15, 0.20][:n])


# ---------------------------------------------------------------------------
# Test: unconstrained optimisation produces valid weights summing to 1
# ---------------------------------------------------------------------------


class TestUnconstrainedOptimization:
    def test_weights_sum_to_one(self) -> None:
        positions = _make_positions()
        opt = PortfolioOptimizer()
        result = opt.optimize(
            current_positions=positions,
            expected_returns=_simple_expected_returns(),
            covariance_matrix=_simple_cov(),
            constraints=OptimizationConstraints(
                risk_aversion=1.0, long_only=False
            ),
        )
        assert isinstance(result, OptimizationResult)
        assert abs(result.target_weights.sum() - 1.0) < 1e-6

    def test_expected_return_and_volatility_populated(self) -> None:
        positions = _make_positions()
        opt = PortfolioOptimizer()
        result = opt.optimize(
            current_positions=positions,
            expected_returns=_simple_expected_returns(),
            covariance_matrix=_simple_cov(),
            constraints=OptimizationConstraints(
                risk_aversion=1.0, long_only=False
            ),
        )
        assert result.expected_return > 0
        assert result.expected_volatility > 0
        assert isinstance(result.sharpe_ratio, float)

    def test_unconstrained_may_have_negative_weights(self) -> None:
        """Without long_only, the optimizer may short an asset."""
        positions = _make_positions()
        # Expected returns strongly favour asset 2; high risk_aversion
        # may still produce all positive, so use low risk_aversion and
        # skewed returns.
        mu = np.array([-0.10, 0.30, 0.30])
        opt = PortfolioOptimizer()
        result = opt.optimize(
            current_positions=positions,
            expected_returns=mu,
            covariance_matrix=_simple_cov(),
            constraints=OptimizationConstraints(
                risk_aversion=0.1, long_only=False
            ),
        )
        # With very negative expected return on asset 0, it should be shorted
        assert result.target_weights[0] < -0.01


# ---------------------------------------------------------------------------
# Test: long-only constraint produces no negative weights
# ---------------------------------------------------------------------------


class TestLongOnlyConstraint:
    def test_no_negative_weights(self) -> None:
        positions = _make_positions()
        mu = np.array([-0.10, 0.30, 0.30])
        opt = PortfolioOptimizer()
        result = opt.optimize(
            current_positions=positions,
            expected_returns=mu,
            covariance_matrix=_simple_cov(),
            constraints=OptimizationConstraints(
                risk_aversion=0.1, long_only=True
            ),
        )
        assert np.all(result.target_weights >= -1e-8)
        assert abs(result.target_weights.sum() - 1.0) < 1e-6


# ---------------------------------------------------------------------------
# Test: asset class bound respected
# ---------------------------------------------------------------------------


class TestAssetClassBounds:
    def test_crypto_upper_bound(self) -> None:
        """crypto assets should not exceed 30 % of the portfolio."""
        positions = _make_positions(
            n=3, asset_classes=["equity", "crypto", "crypto"]
        )
        # Returns strongly favour crypto to tempt the optimizer
        mu = np.array([0.05, 0.40, 0.40])
        opt = PortfolioOptimizer()
        result = opt.optimize(
            current_positions=positions,
            expected_returns=mu,
            covariance_matrix=_simple_cov(),
            constraints=OptimizationConstraints(
                risk_aversion=0.5,
                long_only=True,
                asset_class_bounds={"crypto": (0.0, 0.30)},
            ),
        )
        # crypto weight = sum of positions[1] + positions[2]
        crypto_weight = result.target_weights[1] + result.target_weights[2]
        assert crypto_weight <= 0.30 + 1e-6

    def test_asset_class_lower_bound(self) -> None:
        """Equity must be at least 50 %."""
        positions = _make_positions(
            n=3, asset_classes=["equity", "crypto", "crypto"]
        )
        mu = np.array([0.05, 0.40, 0.40])
        opt = PortfolioOptimizer()
        result = opt.optimize(
            current_positions=positions,
            expected_returns=mu,
            covariance_matrix=_simple_cov(),
            constraints=OptimizationConstraints(
                risk_aversion=0.5,
                long_only=True,
                asset_class_bounds={"equity": (0.50, 1.0)},
            ),
        )
        equity_weight = result.target_weights[0]
        assert equity_weight >= 0.50 - 1e-6


# ---------------------------------------------------------------------------
# Test: trade list correctly reflects weight diff
# ---------------------------------------------------------------------------


class TestTradeList:
    def test_trade_list_reflects_weight_diff(self) -> None:
        positions = _make_positions()
        # All positions start with equal weight (1/3 each)
        opt = PortfolioOptimizer()
        result = opt.optimize(
            current_positions=positions,
            expected_returns=np.array([0.05, 0.15, 0.30]),
            covariance_matrix=_simple_cov(),
            constraints=OptimizationConstraints(
                risk_aversion=1.0, long_only=True
            ),
        )
        current_weights = np.array([1 / 3, 1 / 3, 1 / 3])
        for trade in result.trades:
            idx = int(trade.instrument_id.split("-")[1])
            diff = result.target_weights[idx] - current_weights[idx]
            if diff > 0:
                assert trade.side == "buy"
            else:
                assert trade.side == "sell"
            assert trade.quantity > 0
            assert trade.estimated_cost > 0

    def test_no_trades_when_already_optimal(self) -> None:
        """If current weights match optimal, no trades should be generated."""
        positions = _make_positions(n=2)
        # Use equal expected returns + diagonal cov + equal current weights
        # With risk_aversion=1 and diagonal cov [0.04, 0.04],
        # the optimizer should target roughly 50/50
        mu = np.array([0.10, 0.10])
        cov = np.diag([0.04, 0.04])
        opt = PortfolioOptimizer()
        result = opt.optimize(
            current_positions=positions,
            expected_returns=mu,
            covariance_matrix=cov,
            constraints=OptimizationConstraints(
                risk_aversion=1.0, long_only=True
            ),
        )
        # Optimal should be ~50/50 which matches current weights
        assert len(result.trades) == 0 or all(
            t.quantity < Decimal("0.01") for t in result.trades
        )


# ---------------------------------------------------------------------------
# Test: infeasible constraints return clear error
# ---------------------------------------------------------------------------


class TestInfeasibleConstraints:
    def test_infeasible_raises_value_error(self) -> None:
        """Contradictory constraints should raise a ValueError."""
        positions = _make_positions(
            n=2, asset_classes=["equity", "crypto"]
        )
        mu = np.array([0.10, 0.20])
        cov = np.diag([0.04, 0.16])
        opt = PortfolioOptimizer()
        # equity >= 80 % AND crypto >= 80 % is impossible with sum == 1
        with pytest.raises(ValueError, match="infeasible|failed"):
            opt.optimize(
                current_positions=positions,
                expected_returns=mu,
                covariance_matrix=cov,
                constraints=OptimizationConstraints(
                    risk_aversion=1.0,
                    long_only=True,
                    asset_class_bounds={
                        "equity": (0.80, 1.0),
                        "crypto": (0.80, 1.0),
                    },
                ),
            )

    def test_empty_positions_raises(self) -> None:
        opt = PortfolioOptimizer()
        with pytest.raises(ValueError, match="empty"):
            opt.optimize(
                current_positions=[],
                expected_returns=np.array([]),
                covariance_matrix=np.array([[]]),
                constraints=OptimizationConstraints(),
            )


# ---------------------------------------------------------------------------
# Test: max_single_weight constraint
# ---------------------------------------------------------------------------


class TestMaxSingleWeight:
    def test_no_weight_exceeds_cap(self) -> None:
        positions = _make_positions()
        # Returns heavily favour asset 2 to tempt concentration
        mu = np.array([0.01, 0.01, 0.50])
        opt = PortfolioOptimizer()
        result = opt.optimize(
            current_positions=positions,
            expected_returns=mu,
            covariance_matrix=_simple_cov(),
            constraints=OptimizationConstraints(
                risk_aversion=0.5,
                long_only=True,
                max_single_weight=0.50,
            ),
        )
        assert np.all(result.target_weights <= 0.50 + 1e-6)


# ---------------------------------------------------------------------------
# Test: turnover constraint
# ---------------------------------------------------------------------------


class TestTurnoverConstraint:
    def test_turnover_respected(self) -> None:
        positions = _make_positions()
        mu = np.array([0.01, 0.01, 0.50])
        max_turnover = 0.3
        opt = PortfolioOptimizer()
        result = opt.optimize(
            current_positions=positions,
            expected_returns=mu,
            covariance_matrix=_simple_cov(),
            constraints=OptimizationConstraints(
                risk_aversion=0.5,
                long_only=True,
                max_turnover=max_turnover,
            ),
        )
        current_w = np.array([1 / 3, 1 / 3, 1 / 3])
        actual_turnover = np.abs(result.target_weights - current_w).sum()
        assert actual_turnover <= max_turnover + 1e-6
