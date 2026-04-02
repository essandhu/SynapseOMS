"""Position domain model."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from decimal import Decimal


@dataclass
class Position:
    """Represents a single instrument position within a portfolio."""

    instrument_id: str
    venue_id: str
    quantity: Decimal
    average_cost: Decimal
    market_price: Decimal
    unrealized_pnl: Decimal
    realized_pnl: Decimal
    asset_class: str  # "equity", "crypto", "tokenized_security"
    settlement_cycle: str  # "T0", "T2"
    quote_currency: str = "USD"
    updated_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))

    # ------------------------------------------------------------------
    # Derived helpers
    # ------------------------------------------------------------------

    @property
    def market_value(self) -> Decimal:
        """Current notional value of the position."""
        return self.quantity * self.market_price

    @property
    def cost_basis(self) -> Decimal:
        """Total cost basis of the position."""
        return self.quantity * self.average_cost

    def update_market_price(self, price: Decimal) -> None:
        """Refresh market price and recalculate unrealized P&L."""
        self.market_price = price
        self.unrealized_pnl = (price - self.average_cost) * self.quantity
        self.updated_at = datetime.now(timezone.utc)
