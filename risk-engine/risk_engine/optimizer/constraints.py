"""Optimization constraints dataclass for portfolio construction."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class OptimizationConstraints:
    """Parameters and constraints that govern portfolio optimization.

    Attributes
    ----------
    risk_aversion:
        Controls the trade-off between expected return and risk.
        Higher values produce more conservative (lower-volatility) portfolios.
    long_only:
        If ``True``, all weights must be >= 0 (no short-selling).
    max_single_weight:
        Upper bound on any individual asset weight (e.g. 0.40 = 40 %).
    sector_limits:
        Per-sector upper bounds keyed by sector name.
    target_volatility:
        Maximum annualised portfolio volatility (sigma).
    max_turnover:
        Maximum L1-norm distance between current and target weights.
    asset_class_bounds:
        Per-asset-class (lo, hi) weight bounds keyed by class name.
        Example: ``{"crypto": (0.0, 0.30)}`` limits crypto to at most 30 %.
    """

    risk_aversion: float = 1.0
    long_only: bool = True
    max_single_weight: float | None = None
    sector_limits: dict[str, float] | None = None
    target_volatility: float | None = None
    max_turnover: float | None = None
    asset_class_bounds: dict[str, tuple[float, float]] | None = None
