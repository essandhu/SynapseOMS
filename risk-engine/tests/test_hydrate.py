"""Tests for portfolio and settlement hydration from PostgreSQL."""

from datetime import datetime, timezone
from decimal import Decimal
from unittest.mock import AsyncMock, patch

import pytest

from risk_engine.db.hydrate import hydrate_portfolio, hydrate_settlements
from risk_engine.domain.portfolio import Portfolio
from risk_engine.settlement.tracker import SettlementTracker


def _make_row(**kwargs):
    """Create a dict that behaves like an asyncpg Record for column access."""
    defaults = {
        "instrument_id": "AAPL",
        "venue_id": "sim-exchange",
        "quantity": Decimal("50"),
        "average_cost": Decimal("185"),
        "market_price": Decimal("190"),
        "unrealized_pnl": Decimal("250"),
        "realized_pnl": Decimal("0"),
        "asset_class": "equity",
    }
    defaults.update(kwargs)
    return defaults


class TestHydratePortfolio:
    """Portfolio hydration from database rows."""

    @pytest.mark.asyncio
    async def test_hydrates_equity_position(self) -> None:
        """Equity (T2) position deducts available_cash but not cash."""
        p = Portfolio()
        rows = [_make_row(asset_class="equity")]

        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=rows)
        mock_conn.close = AsyncMock()

        with patch("risk_engine.db.hydrate.asyncpg") as mock_asyncpg:
            mock_asyncpg.connect = AsyncMock(return_value=mock_conn)
            await hydrate_portfolio(p, "postgresql://test")

        assert "AAPL" in p.positions
        assert p.positions["AAPL"].quantity == Decimal("50")
        # T2 equity: cash unchanged, available_cash reduced by cost basis
        cost_basis = Decimal("50") * Decimal("185")  # 9250
        assert p.cash == Decimal("100000")
        assert p.available_cash == Decimal("100000") - cost_basis
        # NAV = available_cash + position_market_value
        pos_value = Decimal("50") * Decimal("190")  # 9500
        assert p.nav == p.available_cash + pos_value

    @pytest.mark.asyncio
    async def test_hydrates_crypto_position(self) -> None:
        """Crypto (T0) position deducts both cash and available_cash."""
        p = Portfolio()
        rows = [_make_row(
            instrument_id="BTC-USD",
            quantity=Decimal("0.1"),
            average_cost=Decimal("65000"),
            market_price=Decimal("66000"),
            asset_class="crypto",
        )]

        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=rows)
        mock_conn.close = AsyncMock()

        with patch("risk_engine.db.hydrate.asyncpg") as mock_asyncpg:
            mock_asyncpg.connect = AsyncMock(return_value=mock_conn)
            await hydrate_portfolio(p, "postgresql://test")

        cost_basis = Decimal("0.1") * Decimal("65000")  # 6500
        assert p.cash == Decimal("100000") - cost_basis
        assert p.available_cash == Decimal("100000") - cost_basis

    @pytest.mark.asyncio
    async def test_hydrates_mixed_positions(self) -> None:
        """Mixed T0 + T2 positions compute correct cash balances."""
        p = Portfolio()
        rows = [
            _make_row(
                instrument_id="AAPL",
                quantity=Decimal("100"),
                average_cost=Decimal("150"),
                market_price=Decimal("155"),
                asset_class="equity",
            ),
            _make_row(
                instrument_id="BTC-USD",
                quantity=Decimal("1"),
                average_cost=Decimal("60000"),
                market_price=Decimal("62000"),
                asset_class="crypto",
            ),
        ]

        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=rows)
        mock_conn.close = AsyncMock()

        with patch("risk_engine.db.hydrate.asyncpg") as mock_asyncpg:
            mock_asyncpg.connect = AsyncMock(return_value=mock_conn)
            await hydrate_portfolio(p, "postgresql://test")

        assert len(p.positions) == 2
        # T2 equity: cash unchanged, available_cash -= 15000
        # T0 crypto: cash -= 60000, available_cash -= 60000
        assert p.cash == Decimal("100000") - Decimal("60000")  # 40000
        assert p.available_cash == Decimal("100000") - Decimal("15000") - Decimal("60000")  # 25000
        # NAV = 25000 + (100*155) + (1*62000) = 25000 + 15500 + 62000 = 102500
        assert p.nav == Decimal("102500")

    @pytest.mark.asyncio
    async def test_skips_when_no_postgres_url(self) -> None:
        """Hydration is a no-op when POSTGRES_URL is empty."""
        p = Portfolio()
        await hydrate_portfolio(p, "")
        assert len(p.positions) == 0
        assert p.nav == Decimal("100000")

    @pytest.mark.asyncio
    async def test_handles_db_connection_failure(self) -> None:
        """Hydration gracefully handles connection failures."""
        p = Portfolio()

        with patch("risk_engine.db.hydrate.asyncpg") as mock_asyncpg:
            mock_asyncpg.connect = AsyncMock(side_effect=ConnectionError("refused"))
            await hydrate_portfolio(p, "postgresql://bad-host")

        # Portfolio stays at defaults
        assert len(p.positions) == 0
        assert p.nav == Decimal("100000")

    @pytest.mark.asyncio
    async def test_handles_empty_positions_table(self) -> None:
        """No error when positions table is empty."""
        p = Portfolio()

        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        mock_conn.close = AsyncMock()

        with patch("risk_engine.db.hydrate.asyncpg") as mock_asyncpg:
            mock_asyncpg.connect = AsyncMock(return_value=mock_conn)
            await hydrate_portfolio(p, "postgresql://test")

        assert len(p.positions) == 0
        assert p.nav == Decimal("100000")


class TestHydrateSettlements:
    """Settlement tracker hydration from fills table."""

    @pytest.mark.asyncio
    async def test_hydrates_equity_fills_as_pending(self) -> None:
        """T+2 equity fills are loaded as pending settlements."""
        tracker = SettlementTracker()
        rows = [
            {
                "instrument_id": "AAPL",
                "asset_class": "equity",
                "side": "buy",
                "quantity": Decimal("50"),
                "price": Decimal("185"),
                "timestamp": datetime(2026, 4, 4, 18, 0, 0, tzinfo=timezone.utc),
            },
            {
                "instrument_id": "MSFT",
                "asset_class": "equity",
                "side": "buy",
                "quantity": Decimal("30"),
                "price": Decimal("420"),
                "timestamp": datetime(2026, 4, 4, 18, 0, 0, tzinfo=timezone.utc),
            },
        ]

        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=rows)
        mock_conn.close = AsyncMock()

        with patch("risk_engine.db.hydrate.asyncpg") as mock_asyncpg:
            mock_asyncpg.connect = AsyncMock(return_value=mock_conn)
            await hydrate_settlements(tracker, "postgresql://test")

        risk = tracker.compute_settlement_risk()
        assert risk["pending_count"] == 2
        # AAPL: 50*185=9250, MSFT: 30*420=12600, total=21850
        assert risk["total_unsettled"] == Decimal("21850")

    @pytest.mark.asyncio
    async def test_skips_when_no_postgres_url(self) -> None:
        """No-op when POSTGRES_URL is empty."""
        tracker = SettlementTracker()
        await hydrate_settlements(tracker, "")
        assert tracker.compute_settlement_risk()["pending_count"] == 0

    @pytest.mark.asyncio
    async def test_handles_empty_fills(self) -> None:
        """No error when fills table is empty."""
        tracker = SettlementTracker()

        mock_conn = AsyncMock()
        mock_conn.fetch = AsyncMock(return_value=[])
        mock_conn.close = AsyncMock()

        with patch("risk_engine.db.hydrate.asyncpg") as mock_asyncpg:
            mock_asyncpg.connect = AsyncMock(return_value=mock_conn)
            await hydrate_settlements(tracker, "postgresql://test")

        assert tracker.compute_settlement_risk()["pending_count"] == 0
