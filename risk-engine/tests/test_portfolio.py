"""Tests for Portfolio domain model — NAV computation and fill application."""

from decimal import Decimal

import pytest

from risk_engine.domain.portfolio import Portfolio
from risk_engine.domain.position import Position


class TestNavComputation:
    """NAV = available_cash + positions_market_value."""

    def test_nav_empty_portfolio(self) -> None:
        """Empty portfolio NAV equals starting cash."""
        p = Portfolio(cash=Decimal("100000"), available_cash=Decimal("100000"))
        assert p.nav == Decimal("100000")

    def test_nav_with_positions_no_unsettled(self) -> None:
        """NAV = cash + positions when everything is settled (T0/crypto)."""
        p = Portfolio(
            cash=Decimal("90000"),
            available_cash=Decimal("90000"),
            positions={
                "BTC-USD": Position(
                    instrument_id="BTC-USD",
                    venue_id="binance",
                    quantity=Decimal("0.1"),
                    average_cost=Decimal("100000"),
                    market_price=Decimal("105000"),
                    unrealized_pnl=Decimal("500"),
                    realized_pnl=Decimal("0"),
                    asset_class="crypto",
                    settlement_cycle="T0",
                ),
            },
        )
        # NAV = 90000 + (0.1 * 105000) = 90000 + 10500 = 100500
        assert p.nav == Decimal("100500")

    def test_nav_after_t2_buy_no_double_count(self) -> None:
        """T+2 equity buy must NOT double-count the purchase amount.

        After buying 50 AAPL at $200 (T+2):
        - cash stays at $100K (not yet settled)
        - available_cash = $90K (committed)
        - unsettled_cash = $10K (pending debit)
        - position value = 50 * $200 = $10K

        NAV should be ~$100K, NOT $120K.
        """
        p = Portfolio(
            cash=Decimal("100000"),
            available_cash=Decimal("100000"),
        )
        p.apply_fill(
            instrument_id="AAPL",
            venue_id="alpaca",
            side="buy",
            quantity=Decimal("50"),
            price=Decimal("200"),
            asset_class="equity",
            settlement_cycle="T2",
        )
        # Position value: 50 * 200 = 10000
        # available_cash: 100000 - 10000 = 90000
        # NAV should be: 90000 + 10000 = 100000
        assert p.nav == Decimal("100000")

    def test_nav_after_t0_buy(self) -> None:
        """T0 (crypto) buy deducts cash immediately — NAV stays correct."""
        p = Portfolio(
            cash=Decimal("100000"),
            available_cash=Decimal("100000"),
        )
        p.apply_fill(
            instrument_id="BTC-USD",
            venue_id="binance",
            side="buy",
            quantity=Decimal("0.1"),
            price=Decimal("60000"),
            asset_class="crypto",
            settlement_cycle="T0",
        )
        # cash: 100000 - 6000 = 94000
        # position value: 0.1 * 60000 = 6000
        # NAV: 94000 + 6000 = 100000
        assert p.nav == Decimal("100000")

    def test_nav_mixed_t0_and_t2(self) -> None:
        """Mixed crypto (T0) and equity (T2) buys compute NAV correctly."""
        p = Portfolio(
            cash=Decimal("100000"),
            available_cash=Decimal("100000"),
        )
        # T0 crypto buy
        p.apply_fill(
            instrument_id="BTC-USD",
            venue_id="binance",
            side="buy",
            quantity=Decimal("0.1"),
            price=Decimal("60000"),
            asset_class="crypto",
            settlement_cycle="T0",
        )
        # T2 equity buy
        p.apply_fill(
            instrument_id="AAPL",
            venue_id="alpaca",
            side="buy",
            quantity=Decimal("50"),
            price=Decimal("200"),
            asset_class="equity",
            settlement_cycle="T2",
        )
        # cash: 94000 (only crypto deducted)
        # available_cash: 94000 - 10000 = 84000
        # positions: BTC(6000) + AAPL(10000) = 16000
        # NAV: 84000 + 16000 = 100000
        assert p.nav == Decimal("100000")
