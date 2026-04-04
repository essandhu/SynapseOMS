"""Hydrate portfolio and settlement tracker from PostgreSQL on startup.

When the risk engine restarts, its in-memory portfolio is empty.  The Kafka
consumer only processes *new* events (committed offset is at the end), so
historical positions are lost.  This module queries the positions and fills
tables to reconstruct portfolio and settlement state so the dashboard shows
accurate data immediately.
"""

from __future__ import annotations

from datetime import date
from decimal import Decimal

import asyncpg
import structlog

from risk_engine.domain.portfolio import Portfolio
from risk_engine.domain.position import Position
from risk_engine.settlement.tracker import SettlementTracker

logger = structlog.get_logger(__name__)

# Map asset_class to settlement cycle
_SETTLEMENT_CYCLES: dict[str, str] = {
    "equity": "T2",
    "crypto": "T0",
    "tokenized_security": "T0",
    "future": "T0",
    "option": "T0",
}


async def hydrate_portfolio(portfolio: Portfolio, postgres_url: str) -> None:
    """Load positions from PostgreSQL and update the portfolio in-place.

    Computes ``available_cash`` by deducting the cost basis of each position
    from the default starting cash, mirroring the logic in ``apply_fill``.
    """
    if not postgres_url:
        logger.warning("portfolio_hydration_skipped", reason="POSTGRES_URL not set")
        return

    try:
        conn = await asyncpg.connect(postgres_url)
    except Exception:
        logger.exception("portfolio_hydration_db_connect_failed")
        return

    try:
        rows = await conn.fetch(
            """
            SELECT p.instrument_id, p.venue_id, p.quantity, p.average_cost,
                   p.market_price, p.unrealized_pnl, p.realized_pnl,
                   p.asset_class
            FROM positions p
            WHERE p.quantity > 0
            """
        )

        if not rows:
            logger.info("portfolio_hydration_no_positions")
            return

        with portfolio._lock:
            for row in rows:
                asset_class = row["asset_class"] or "crypto"
                settlement_cycle = _SETTLEMENT_CYCLES.get(asset_class, "T0")
                cost_basis = Decimal(str(row["quantity"])) * Decimal(str(row["average_cost"]))

                portfolio.positions[row["instrument_id"]] = Position(
                    instrument_id=row["instrument_id"],
                    venue_id=row["venue_id"],
                    quantity=Decimal(str(row["quantity"])),
                    average_cost=Decimal(str(row["average_cost"])),
                    market_price=Decimal(str(row["market_price"])),
                    unrealized_pnl=Decimal(str(row["unrealized_pnl"] or 0)),
                    realized_pnl=Decimal(str(row["realized_pnl"] or 0)),
                    asset_class=asset_class,
                    settlement_cycle=settlement_cycle,
                )

                # Deduct cost basis from cash/available_cash to match fill logic
                if settlement_cycle == "T0":
                    portfolio.cash -= cost_basis
                    portfolio.available_cash -= cost_basis
                else:
                    # T2: cash not yet settled, but available_cash is committed
                    portfolio.available_cash -= cost_basis
                    portfolio.unsettled_cash += cost_basis

            portfolio._recompute_nav()

        logger.info(
            "portfolio_hydrated",
            positions=len(rows),
            nav=str(portfolio.nav),
            cash=str(portfolio.cash),
            available_cash=str(portfolio.available_cash),
        )
    except Exception:
        logger.exception("portfolio_hydration_failed")
    finally:
        await conn.close()


async def hydrate_settlements(
    tracker: SettlementTracker, postgres_url: str
) -> None:
    """Load unsettled T+2 fills from PostgreSQL into the SettlementTracker.

    Only equity fills (T+2) whose settlement date hasn't passed are loaded.
    Crypto/tokenized fills settle immediately (T+0) and are skipped.
    """
    if not postgres_url:
        return

    try:
        conn = await asyncpg.connect(postgres_url)
    except Exception:
        logger.exception("settlement_hydration_db_connect_failed")
        return

    try:
        rows = await conn.fetch(
            """
            SELECT o.instrument_id, o.asset_class, o.side,
                   f.quantity, f.price, f.timestamp
            FROM fills f
            JOIN orders o ON o.id = f.order_id
            WHERE o.asset_class = 'equity'
            """
        )

        loaded = 0
        for row in rows:
            trade_date = row["timestamp"].date()
            tracker.record_fill(
                instrument_id=row["instrument_id"],
                asset_class=row["asset_class"],
                side=row["side"],
                quantity=Decimal(str(row["quantity"])),
                price=Decimal(str(row["price"])),
                trade_date=trade_date,
            )
            loaded += 1

        # Remove already-settled records
        tracker.settle_matured()

        logger.info("settlements_hydrated", fills_loaded=loaded)
    except Exception:
        logger.exception("settlement_hydration_failed")
    finally:
        await conn.close()
