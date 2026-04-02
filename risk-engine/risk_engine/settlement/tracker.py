"""Settlement tracker — monitors pending settlements and computes settlement risk."""

from __future__ import annotations

import threading
from dataclasses import dataclass
from datetime import date, timedelta
from decimal import Decimal

import structlog

logger = structlog.get_logger()


@dataclass
class PendingSettlement:
    """A single pending settlement record."""

    trade_date: date
    settlement_date: date
    instrument_id: str
    asset_class: str
    amount: Decimal  # Positive = cash incoming (sell), negative = cash outgoing (buy)
    status: str = "pending"  # "pending", "settled", "failed"


class SettlementTracker:
    """Tracks pending settlements and computes settlement risk.

    Settlement cycles:
    - T+0: crypto, tokenized_security — immediate, no pending record.
    - T+2: equity (and default) — settles two business days after trade date.
    """

    def __init__(self) -> None:
        self._pending: list[PendingSettlement] = []
        self._lock = threading.Lock()

    # ------------------------------------------------------------------
    # Recording fills
    # ------------------------------------------------------------------

    def record_fill(
        self,
        instrument_id: str,
        asset_class: str,
        side: str,
        quantity: Decimal,
        price: Decimal,
        trade_date: date | None = None,
    ) -> None:
        """Record a fill and create a pending settlement if needed.

        Parameters
        ----------
        instrument_id:
            Unique identifier for the instrument.
        asset_class:
            One of "equity", "crypto", "tokenized_security".
        side:
            "BUY" or "SELL".
        quantity:
            Unsigned quantity filled.
        price:
            Execution price per unit.
        trade_date:
            Trade date; defaults to today.
        """
        if trade_date is None:
            trade_date = date.today()

        notional = quantity * price
        amount = notional if side.upper() == "SELL" else -notional

        settlement_cycle = self._get_settlement_cycle(asset_class)

        if settlement_cycle == "T0":
            # Crypto / tokenized: immediately settled, no pending record needed
            logger.info(
                "immediate_settlement",
                instrument_id=instrument_id,
                amount=str(amount),
            )
            return

        settlement_date = self._add_business_days(
            trade_date, self._cycle_days(settlement_cycle)
        )

        with self._lock:
            self._pending.append(
                PendingSettlement(
                    trade_date=trade_date,
                    settlement_date=settlement_date,
                    instrument_id=instrument_id,
                    asset_class=asset_class,
                    amount=amount,
                )
            )

        logger.info(
            "settlement_recorded",
            instrument_id=instrument_id,
            trade_date=str(trade_date),
            settlement_date=str(settlement_date),
            amount=str(amount),
        )

    # ------------------------------------------------------------------
    # Settling matured records
    # ------------------------------------------------------------------

    def settle_matured(self, as_of: date | None = None) -> None:
        """Mark settlements as complete when settlement_date has arrived.

        Parameters
        ----------
        as_of:
            The date to evaluate against; defaults to today.
        """
        if as_of is None:
            as_of = date.today()

        with self._lock:
            for s in self._pending:
                if s.status == "pending" and s.settlement_date <= as_of:
                    s.status = "settled"
                    logger.info(
                        "settlement_completed",
                        instrument_id=s.instrument_id,
                        settlement_date=str(s.settlement_date),
                    )

    # ------------------------------------------------------------------
    # Risk computation
    # ------------------------------------------------------------------

    def compute_settlement_risk(self) -> dict:
        """Compute settlement risk metrics.

        Returns
        -------
        dict with keys:
            total_unsettled:  Sum of absolute values of all pending amounts.
            by_date:          Net amount per settlement date (str -> str).
            cash_committed:   Absolute value of buy-side cash still unsettled.
            pending_count:    Number of pending settlement records.
        """
        with self._lock:
            pending = [s for s in self._pending if s.status == "pending"]

        total_unsettled = sum(abs(s.amount) for s in pending)

        # Breakdown by settlement date
        by_date: dict[date, Decimal] = {}
        for s in pending:
            by_date[s.settlement_date] = (
                by_date.get(s.settlement_date, Decimal("0")) + s.amount
            )

        # Impact on available cash: sum of negative amounts (buy commitments)
        cash_impact = sum(s.amount for s in pending if s.amount < 0)

        return {
            "total_unsettled": total_unsettled,
            "by_date": {str(d): str(v) for d, v in sorted(by_date.items())},
            "cash_committed": abs(cash_impact),
            "pending_count": len(pending),
        }

    # ------------------------------------------------------------------
    # Settlement cycle helpers
    # ------------------------------------------------------------------

    def _get_settlement_cycle(self, asset_class: str) -> str:
        """Return the settlement cycle string for the given asset class."""
        cycles = {
            "equity": "T2",
            "crypto": "T0",
            "tokenized_security": "T0",
        }
        return cycles.get(asset_class, "T2")

    def _cycle_days(self, cycle: str) -> int:
        """Return the number of business days for a cycle label."""
        return {"T0": 0, "T1": 1, "T2": 2}.get(cycle, 2)

    @staticmethod
    def _add_business_days(start: date, days: int) -> date:
        """Add business days (skip weekends).

        Parameters
        ----------
        start:
            The starting date.
        days:
            Number of business days to add. If 0, returns start unchanged.
        """
        current = start
        added = 0
        while added < days:
            current += timedelta(days=1)
            if current.weekday() < 5:  # Monday=0 through Friday=4
                added += 1
        return current
