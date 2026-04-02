"""Risk computation result domain models."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from decimal import Decimal


@dataclass
class RiskCheck:
    """Individual risk-limit check outcome."""

    name: str
    passed: bool
    message: str = ""
    threshold: str = ""
    actual: str = ""


@dataclass
class VaRResult:
    """Value-at-Risk computation output."""

    var_amount: Decimal
    cvar_amount: Decimal
    confidence: float
    horizon_days: int
    method: str  # "historical", "parametric", "monte_carlo"
    computed_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))
    distribution: list[float] | None = None


@dataclass
class RiskCheckResult:
    """Aggregated pre-trade risk check result."""

    approved: bool
    reject_reason: str = ""
    checks: list[RiskCheck] = field(default_factory=list)
    portfolio_var_before: Decimal = Decimal("0")
    portfolio_var_after: Decimal = Decimal("0")
    computed_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))
