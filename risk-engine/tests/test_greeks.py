"""Unit tests for Portfolio Greeks Calculator."""

from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from risk_engine.domain.position import Position
from risk_engine.greeks.calculator import Greeks, GreeksCalculator, PortfolioGreeks


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _pos(
    instrument_id: str = "AAPL",
    quantity: str = "100",
    market_price: str = "150.00",
    asset_class: str = "equity",
) -> Position:
    qty = Decimal(quantity)
    mp = Decimal(market_price)
    ac = Decimal("140.00")
    return Position(
        instrument_id=instrument_id,
        venue_id="NYSE",
        quantity=qty,
        average_cost=ac,
        market_price=mp,
        unrealized_pnl=(mp - ac) * qty,
        realized_pnl=Decimal("0"),
        asset_class=asset_class,
        settlement_cycle="T2" if asset_class == "equity" else "T0",
    )


# ---------------------------------------------------------------------------
# Greeks dataclass
# ---------------------------------------------------------------------------

class TestGreeksDataclass:
    def test_defaults_are_zero(self) -> None:
        g = Greeks()
        assert g.delta == 0.0
        assert g.gamma == 0.0
        assert g.vega == 0.0
        assert g.theta == 0.0
        assert g.rho == 0.0

    def test_add_two_greeks(self) -> None:
        a = Greeks(delta=1.0, gamma=0.1, vega=0.5, theta=-0.2, rho=0.3)
        b = Greeks(delta=0.5, gamma=0.05, vega=0.25, theta=-0.1, rho=0.15)
        result = a + b
        assert result.delta == pytest.approx(1.5)
        assert result.gamma == pytest.approx(0.15)
        assert result.vega == pytest.approx(0.75)
        assert result.theta == pytest.approx(-0.3)
        assert result.rho == pytest.approx(0.45)


# ---------------------------------------------------------------------------
# PortfolioGreeks dataclass
# ---------------------------------------------------------------------------

class TestPortfolioGreeksDataclass:
    def test_fields_present(self) -> None:
        pg = PortfolioGreeks(
            total=Greeks(),
            by_instrument={},
            computed_at=datetime.now(timezone.utc),
        )
        assert isinstance(pg.total, Greeks)
        assert pg.by_instrument == {}
        assert isinstance(pg.computed_at, datetime)


# ---------------------------------------------------------------------------
# GreeksCalculator — spot instruments
# ---------------------------------------------------------------------------

class TestGreeksCalculatorSpot:
    """Tests for equity and crypto (non-option) positions."""

    def test_long_equity_positive_delta(self) -> None:
        pos = _pos(quantity="100", market_price="150.00")
        nav = 100_000.0
        calc = GreeksCalculator()
        result = calc.compute([pos], nav=nav)

        instrument_greeks = result.by_instrument["AAPL"]
        assert instrument_greeks.delta > 0.0
        expected_delta = float(pos.market_value) / nav
        assert instrument_greeks.delta == pytest.approx(expected_delta)

    def test_short_equity_negative_delta(self) -> None:
        pos = _pos(quantity="-50", market_price="200.00")
        nav = 100_000.0
        calc = GreeksCalculator()
        result = calc.compute([pos], nav=nav)

        instrument_greeks = result.by_instrument["AAPL"]
        assert instrument_greeks.delta < 0.0
        expected_delta = float(pos.market_value) / nav  # negative qty -> negative mv
        assert instrument_greeks.delta == pytest.approx(expected_delta)

    def test_spot_gamma_vega_theta_rho_are_zero(self) -> None:
        pos = _pos()
        calc = GreeksCalculator()
        result = calc.compute([pos], nav=100_000.0)

        g = result.by_instrument["AAPL"]
        assert g.gamma == 0.0
        assert g.vega == 0.0
        assert g.theta == 0.0
        assert g.rho == 0.0

    def test_crypto_positive_delta(self) -> None:
        pos = _pos(
            instrument_id="BTC-USD",
            quantity="0.5",
            market_price="65000.00",
            asset_class="crypto",
        )
        nav = 100_000.0
        calc = GreeksCalculator()
        result = calc.compute([pos], nav=nav)

        g = result.by_instrument["BTC-USD"]
        expected = float(Decimal("0.5") * Decimal("65000.00")) / nav
        assert g.delta == pytest.approx(expected)

    def test_portfolio_delta_sums_correctly(self) -> None:
        positions = [
            _pos(instrument_id="AAPL", quantity="100", market_price="150.00"),
            _pos(instrument_id="MSFT", quantity="50", market_price="400.00"),
            _pos(
                instrument_id="BTC-USD",
                quantity="0.5",
                market_price="60000.00",
                asset_class="crypto",
            ),
        ]
        nav = 200_000.0
        calc = GreeksCalculator()
        result = calc.compute(positions, nav=nav)

        expected_total = sum(
            float(p.market_value) / nav for p in positions
        )
        assert result.total.delta == pytest.approx(expected_total)

    def test_zero_position_all_greeks_zero(self) -> None:
        pos = _pos(quantity="0", market_price="150.00")
        calc = GreeksCalculator()
        result = calc.compute([pos], nav=100_000.0)

        g = result.by_instrument["AAPL"]
        assert g.delta == 0.0
        assert g.gamma == 0.0
        assert g.vega == 0.0
        assert g.theta == 0.0
        assert g.rho == 0.0

    def test_zero_nav_all_deltas_zero(self) -> None:
        pos = _pos(quantity="100", market_price="150.00")
        calc = GreeksCalculator()
        result = calc.compute([pos], nav=0.0)

        assert result.by_instrument["AAPL"].delta == 0.0
        assert result.total.delta == 0.0

    def test_empty_positions_returns_zero_greeks(self) -> None:
        calc = GreeksCalculator()
        result = calc.compute([], nav=100_000.0)

        assert result.total.delta == 0.0
        assert result.by_instrument == {}

    def test_computed_at_is_recent(self) -> None:
        calc = GreeksCalculator()
        before = datetime.now(timezone.utc)
        result = calc.compute([_pos()], nav=100_000.0)
        after = datetime.now(timezone.utc)

        assert before <= result.computed_at <= after

    def test_market_data_dict_with_beta(self) -> None:
        """When market_data provides beta, delta is beta-adjusted."""
        pos = _pos(instrument_id="AAPL", quantity="100", market_price="150.00")
        nav = 100_000.0
        market_data = {"AAPL": {"beta": 1.2}}
        calc = GreeksCalculator()
        result = calc.compute([pos], nav=nav, market_data=market_data)

        raw_delta = float(pos.market_value) / nav
        expected = raw_delta * 1.2
        assert result.by_instrument["AAPL"].delta == pytest.approx(expected)

    def test_risk_free_rate_stored(self) -> None:
        """risk_free_rate is accepted (used for options in Phase 4)."""
        calc = GreeksCalculator()
        # Should not raise
        result = calc.compute([_pos()], nav=100_000.0, risk_free_rate=0.05)
        assert isinstance(result, PortfolioGreeks)
