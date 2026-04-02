"""Data types for the AI rebalancing assistant."""

from __future__ import annotations

import os
import sys
from dataclasses import dataclass

# Ensure risk-engine is importable.
_RISK_ENGINE_PATH = os.path.join(os.path.dirname(__file__), "..", "..", "risk-engine")
if _RISK_ENGINE_PATH not in sys.path:
    sys.path.insert(0, _RISK_ENGINE_PATH)

from risk_engine.optimizer.constraints import OptimizationConstraints


@dataclass
class RebalanceRequest:
    """User-facing request for the rebalancing assistant.

    Attributes
    ----------
    user_input:
        Free-text description of the rebalancing goal.
    portfolio_summary:
        Human-readable summary of the current portfolio.
    available_instruments:
        Comma-separated or descriptive list of tradeable instruments.
    """

    user_input: str
    portfolio_summary: str
    available_instruments: str

    def to_prompt_vars(self) -> dict[str, str]:
        """Return a dict suitable for formatting prompt templates."""
        return {
            "user_input": self.user_input,
            "portfolio_summary": self.portfolio_summary,
            "available_instruments": self.available_instruments,
        }


@dataclass
class ExtractedConstraints:
    """Structured constraints extracted by the LLM from a user request.

    Attributes
    ----------
    objective:
        One of ``"maximize_sharpe"``, ``"minimize_variance"``, ``"target_return"``.
    target_return:
        Desired portfolio return (annualised), or ``None``.
    risk_aversion:
        Risk-aversion coefficient (higher = more conservative).
    long_only:
        Whether short-selling is prohibited.
    max_single_weight:
        Maximum weight for any single instrument, or ``None``.
    asset_class_bounds:
        Per-asset-class ``[min, max]`` weight bounds, or ``None``.
    sector_limits:
        Per-sector upper-bound weights, or ``None``.
    target_volatility:
        Maximum annualised volatility, or ``None``.
    max_turnover_usd:
        Maximum turnover in USD terms, or ``None``.
    instruments_to_include:
        Instruments the user explicitly wants in the portfolio, or ``None``.
    instruments_to_exclude:
        Instruments the user explicitly wants excluded, or ``None``.
    reasoning:
        LLM explanation of how the request was interpreted.
    """

    objective: str
    target_return: float | None
    risk_aversion: float
    long_only: bool
    max_single_weight: float | None
    asset_class_bounds: dict | None
    sector_limits: dict | None
    target_volatility: float | None
    max_turnover_usd: float | None
    instruments_to_include: list | None
    instruments_to_exclude: list | None
    reasoning: str

    @classmethod
    def from_dict(cls, data: dict) -> ExtractedConstraints:
        """Create an instance from a dictionary (e.g. parsed LLM JSON)."""
        return cls(
            objective=data["objective"],
            target_return=data.get("target_return"),
            risk_aversion=data.get("risk_aversion", 1.0),
            long_only=data.get("long_only", True),
            max_single_weight=data.get("max_single_weight"),
            asset_class_bounds=data.get("asset_class_bounds"),
            sector_limits=data.get("sector_limits"),
            target_volatility=data.get("target_volatility"),
            max_turnover_usd=data.get("max_turnover_usd"),
            instruments_to_include=data.get("instruments_to_include"),
            instruments_to_exclude=data.get("instruments_to_exclude"),
            reasoning=data.get("reasoning", ""),
        )

    def to_optimization_constraints(self, nav: float) -> OptimizationConstraints:
        """Convert to the risk-engine ``OptimizationConstraints`` dataclass.

        Parameters
        ----------
        nav:
            Current net asset value in USD.  Used to convert
            ``max_turnover_usd`` to a weight-based ``max_turnover``.
        """
        max_turnover: float | None = None
        if self.max_turnover_usd is not None:
            max_turnover = self.max_turnover_usd / nav

        acb: dict[str, tuple[float, float]] | None = None
        if self.asset_class_bounds is not None:
            acb = {
                k: (v[0], v[1]) for k, v in self.asset_class_bounds.items()
            }

        return OptimizationConstraints(
            risk_aversion=self.risk_aversion,
            long_only=self.long_only,
            max_single_weight=self.max_single_weight,
            sector_limits=self.sector_limits,
            target_volatility=self.target_volatility,
            max_turnover=max_turnover,
            asset_class_bounds=acb,
        )
