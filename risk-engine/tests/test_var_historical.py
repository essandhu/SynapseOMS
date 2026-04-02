"""Tests for Historical VaR computation."""

from __future__ import annotations

from decimal import Decimal

import numpy as np
import pandas as pd
import pytest

from risk_engine.domain.position import Position
from risk_engine.var.historical import HistoricalVaR


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_position(
    instrument_id: str,
    quantity: Decimal,
    market_price: Decimal,
    asset_class: str = "equity",
    settlement_cycle: str = "T2",
    venue_id: str = "TEST",
) -> Position:
    return Position(
        instrument_id=instrument_id,
        venue_id=venue_id,
        quantity=quantity,
        average_cost=market_price,
        market_price=market_price,
        unrealized_pnl=Decimal("0"),
        realized_pnl=Decimal("0"),
        asset_class=asset_class,
        settlement_cycle=settlement_cycle,
    )


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestBasicVaRComputation:
    """test_basic_var_computation — compute VaR on sample returns, verify
    it's positive and within reasonable bounds."""

    def test_basic_var_computation(
        self,
        sample_positions: dict[str, Position],
        sample_returns_matrix: pd.DataFrame,
    ) -> None:
        engine = HistoricalVaR(window_days=252, confidence=0.99)
        result = engine.compute(sample_positions, sample_returns_matrix)

        assert result.var_amount > 0
        assert result.cvar_amount > 0
        assert result.confidence == 0.99
        assert result.horizon_days == 1
        assert result.method == "historical"
        # VaR should be a reasonable fraction of total portfolio value
        # (not more than 100% of portfolio value for a 1-day horizon)
        total_value = sum(
            float(p.market_value) for p in sample_positions.values()
        )
        assert float(result.var_amount) < total_value


class TestCVaRGreaterThanOrEqualVaR:
    """test_cvar_greater_than_or_equal_var — CVaR (expected shortfall)
    >= VaR always."""

    def test_cvar_greater_than_or_equal_var(
        self,
        sample_positions: dict[str, Position],
        sample_returns_matrix: pd.DataFrame,
    ) -> None:
        engine = HistoricalVaR(window_days=252, confidence=0.99)
        result = engine.compute(sample_positions, sample_returns_matrix)

        assert result.cvar_amount >= result.var_amount


class TestCryptoHigherVaRThanEquity:
    """test_crypto_higher_var_than_equity — portfolio of only crypto has
    higher VaR than only equity at same notional."""

    def test_crypto_higher_var_than_equity(
        self,
        sample_returns_matrix: pd.DataFrame,
    ) -> None:
        notional = Decimal("100000")

        equity_positions = {
            "AAPL": _make_position(
                "AAPL",
                quantity=notional / Decimal("180"),
                market_price=Decimal("180"),
                asset_class="equity",
            ),
        }

        crypto_positions = {
            "BTC-USD": _make_position(
                "BTC-USD",
                quantity=notional / Decimal("65000"),
                market_price=Decimal("65000"),
                asset_class="crypto",
                settlement_cycle="T0",
                venue_id="COINBASE",
            ),
        }

        engine = HistoricalVaR(window_days=252, confidence=0.99)
        equity_result = engine.compute(equity_positions, sample_returns_matrix)
        crypto_result = engine.compute(crypto_positions, sample_returns_matrix)

        assert crypto_result.var_amount > equity_result.var_amount


class TestEmptyPortfolioZeroVaR:
    """test_empty_portfolio_zero_var — empty positions dict returns VaR of 0."""

    def test_empty_portfolio_zero_var(
        self,
        sample_returns_matrix: pd.DataFrame,
    ) -> None:
        engine = HistoricalVaR(window_days=252, confidence=0.99)
        result = engine.compute({}, sample_returns_matrix)

        assert result.var_amount == Decimal("0")
        assert result.cvar_amount == Decimal("0")


class TestSingleInstrumentVaR:
    """test_single_instrument_var — VaR with single instrument matches
    simple percentile calculation."""

    def test_single_instrument_var(
        self,
        sample_returns_matrix: pd.DataFrame,
    ) -> None:
        price = Decimal("180")
        qty = Decimal("100")
        positions = {
            "AAPL": _make_position(
                "AAPL",
                quantity=qty,
                market_price=price,
            ),
        }

        engine = HistoricalVaR(window_days=252, confidence=0.99)
        result = engine.compute(positions, sample_returns_matrix)

        # Manual calculation: single instrument → weight=1.0, portfolio
        # returns == instrument returns
        aapl_returns = sample_returns_matrix["AAPL"].values[-252:]
        expected_var = -float(np.percentile(aapl_returns, 1.0))
        portfolio_value = float(qty * price)
        expected_var_amount = expected_var * portfolio_value

        assert float(result.var_amount) == pytest.approx(
            expected_var_amount, rel=1e-6
        )


class TestMixedCalendarAlignment:
    """test_mixed_calendar_alignment — equity returns with gaps (weekends)
    properly aligned with daily crypto returns."""

    def test_mixed_calendar_alignment(self) -> None:
        rng = np.random.default_rng(seed=99)

        # Create 30 calendar days of dates
        all_dates = pd.date_range("2025-01-01", periods=30, freq="D")
        # Crypto returns exist for every day
        crypto_returns = rng.normal(0.001, 0.04, size=30)

        # Equity returns only on weekdays
        weekday_mask = all_dates.weekday < 5
        equity_dates = all_dates[weekday_mask]
        equity_returns_raw = rng.normal(0.0005, 0.015, size=len(equity_dates))

        # Build a returns matrix with NaN for equity on weekends
        df = pd.DataFrame(index=all_dates)
        df["BTC-USD"] = crypto_returns
        df["AAPL"] = np.nan
        df.loc[equity_dates, "AAPL"] = equity_returns_raw

        positions = {
            "AAPL": _make_position(
                "AAPL",
                quantity=Decimal("100"),
                market_price=Decimal("180"),
                asset_class="equity",
            ),
            "BTC-USD": _make_position(
                "BTC-USD",
                quantity=Decimal("0.5"),
                market_price=Decimal("65000"),
                asset_class="crypto",
                settlement_cycle="T0",
                venue_id="COINBASE",
            ),
        }

        engine = HistoricalVaR(window_days=30, confidence=0.99)
        result = engine.compute(positions, df)

        # Should produce a valid result without errors
        assert result.var_amount >= 0
        assert result.cvar_amount >= 0
        # The distribution should have been computed on all 30 dates
        # (equity forward-filled on weekends)
        assert result.distribution is not None
        assert len(result.distribution) == 30
