"""Portfolio domain model with thread-safe fill application."""

from __future__ import annotations

import threading
from dataclasses import dataclass, field
from datetime import datetime, timezone
from decimal import Decimal

from risk_engine.domain.position import Position


@dataclass
class Portfolio:
    """Aggregated portfolio state — positions, cash, and NAV."""

    positions: dict[str, Position] = field(default_factory=dict)
    nav: Decimal = Decimal("0")
    cash: Decimal = Decimal("100000")
    available_cash: Decimal = Decimal("100000")
    unsettled_cash: Decimal = Decimal("0")
    updated_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))
    _lock: threading.Lock = field(default_factory=threading.Lock, repr=False)

    def __post_init__(self) -> None:
        """Compute initial NAV so it reflects cash + positions on creation."""
        self._recompute_nav()

    # ------------------------------------------------------------------
    # Fill application
    # ------------------------------------------------------------------

    def apply_fill(
        self,
        instrument_id: str,
        venue_id: str,
        side: str,
        quantity: Decimal,
        price: Decimal,
        asset_class: str,
        settlement_cycle: str,
    ) -> None:
        """Apply a fill event to update positions and cash.

        Parameters
        ----------
        instrument_id:
            Unique identifier for the instrument (e.g. "AAPL", "BTC-USD").
        venue_id:
            The venue that executed the fill.
        side:
            "buy" or "sell".
        quantity:
            Unsigned quantity filled.
        price:
            Execution price per unit.
        asset_class:
            One of "equity", "crypto", "tokenized_security".
        settlement_cycle:
            "T0" (instant, crypto) or "T2" (equities).
        """
        with self._lock:
            fill_cost = quantity * price
            side_lower = side.lower()

            if instrument_id in self.positions:
                pos = self.positions[instrument_id]

                if side_lower == "buy":
                    # Weighted-average cost update
                    new_quantity = pos.quantity + quantity
                    if new_quantity != Decimal("0"):
                        pos.average_cost = (
                            (pos.average_cost * pos.quantity) + fill_cost
                        ) / new_quantity
                    pos.quantity = new_quantity

                elif side_lower == "sell":
                    if quantity > pos.quantity:
                        raise ValueError(
                            f"Cannot sell {quantity} of {instrument_id}: "
                            f"only {pos.quantity} held"
                        )
                    # Realized P&L on the sold portion
                    realized = (price - pos.average_cost) * quantity
                    pos.realized_pnl += realized
                    pos.quantity -= quantity

                # Recalculate unrealized P&L
                pos.market_price = price
                pos.unrealized_pnl = (price - pos.average_cost) * pos.quantity
                pos.updated_at = datetime.now(timezone.utc)

                # Remove flat positions
                if pos.quantity == Decimal("0"):
                    del self.positions[instrument_id]

            else:
                if side_lower == "sell":
                    raise ValueError(
                        f"Cannot sell {instrument_id}: no existing position"
                    )

                self.positions[instrument_id] = Position(
                    instrument_id=instrument_id,
                    venue_id=venue_id,
                    quantity=quantity,
                    average_cost=price,
                    market_price=price,
                    unrealized_pnl=Decimal("0"),
                    realized_pnl=Decimal("0"),
                    asset_class=asset_class,
                    settlement_cycle=settlement_cycle,
                )

            # Cash updates — depends on settlement cycle
            if side_lower == "buy":
                if settlement_cycle == "T0":
                    self.cash -= fill_cost
                    self.available_cash -= fill_cost
                else:
                    # T2: cash is committed but not yet settled
                    self.available_cash -= fill_cost
                    self.unsettled_cash += fill_cost
            elif side_lower == "sell":
                if settlement_cycle == "T0":
                    self.cash += fill_cost
                    self.available_cash += fill_cost
                else:
                    # T2: sell proceeds increase available_cash (mirrors the
                    # buy deduction) and settle against unsettled_cash.
                    self.available_cash += fill_cost
                    self.unsettled_cash -= fill_cost
                    self.cash += fill_cost

            self._recompute_nav()

    # ------------------------------------------------------------------
    # NAV
    # ------------------------------------------------------------------

    def compute_nav(self) -> Decimal:
        """Recompute NAV from positions + cash (acquires lock)."""
        with self._lock:
            return self._recompute_nav()

    def _recompute_nav(self) -> Decimal:
        """Internal NAV recomputation — caller must hold ``_lock``.

        NAV = available_cash + positions_market_value

        ``available_cash`` already reflects all committed transactions:
        reduced by buys (including unsettled T+2) and increased by sells.
        This avoids double-counting unsettled equity purchases where
        ``cash`` hasn't been debited yet but the position already exists.
        """
        positions_value = sum(
            (p.quantity * p.market_price for p in self.positions.values()),
            Decimal("0"),
        )
        self.nav = self.available_cash + positions_value
        self.updated_at = datetime.now(timezone.utc)
        return self.nav
