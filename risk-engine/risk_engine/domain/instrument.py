"""Instrument domain model."""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal


@dataclass
class Instrument:
    """Static reference data for a tradeable instrument."""

    id: str
    symbol: str
    name: str
    asset_class: str  # "equity", "crypto", "tokenized_security"
    quote_currency: str
    base_currency: str
    settlement_cycle: str  # "T0", "T2"
    tick_size: Decimal = Decimal("0.01")
    lot_size: Decimal = Decimal("1")
