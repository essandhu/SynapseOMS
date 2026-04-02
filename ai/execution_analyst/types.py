"""Domain types for AI execution analysis."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime


@dataclass(frozen=True)
class TradeContext:
    """All context needed to render the execution analysis prompt."""

    side: str
    quantity: int
    instrument_id: str
    asset_class: str
    order_type: str
    limit_price: float
    submitted_at: datetime
    completed_at: datetime
    venues: str
    fill_count: int
    fill_table: str
    arrival_price: float
    spread_bps: float
    vwap_5min: float
    adv_30d: int
    size_pct_adv: float
    venue_comparison_table: str

    def to_prompt_vars(self) -> dict[str, object]:
        """Return a dict suitable for EXECUTION_ANALYSIS_PROMPT.format(**vars)."""
        return {
            "side": self.side,
            "quantity": self.quantity,
            "instrument_id": self.instrument_id,
            "asset_class": self.asset_class,
            "order_type": self.order_type,
            "limit_price": self.limit_price,
            "submitted_at": self.submitted_at,
            "completed_at": self.completed_at,
            "venues": self.venues,
            "fill_count": self.fill_count,
            "fill_table": self.fill_table,
            "arrival_price": self.arrival_price,
            "spread_bps": self.spread_bps,
            "vwap_5min": self.vwap_5min,
            "adv_30d": self.adv_30d,
            "size_pct_adv": self.size_pct_adv,
            "venue_comparison_table": self.venue_comparison_table,
        }


@dataclass(frozen=True)
class ExecutionReport:
    """Structured output from AI execution analysis."""

    overall_grade: str
    implementation_shortfall_bps: float
    summary: str
    venue_analysis: list[dict]
    recommendations: list[str]
    market_impact_estimate_bps: float

    def to_dict(self) -> dict:
        """Serialize to a plain dict for JSON output."""
        return {
            "overall_grade": self.overall_grade,
            "implementation_shortfall_bps": self.implementation_shortfall_bps,
            "summary": self.summary,
            "venue_analysis": self.venue_analysis,
            "recommendations": self.recommendations,
            "market_impact_estimate_bps": self.market_impact_estimate_bps,
        }

    @classmethod
    def from_dict(cls, data: dict) -> ExecutionReport:
        """Deserialize from a dict (e.g. parsed JSON from LLM)."""
        return cls(
            overall_grade=data["overall_grade"],
            implementation_shortfall_bps=data["implementation_shortfall_bps"],
            summary=data["summary"],
            venue_analysis=data["venue_analysis"],
            recommendations=data["recommendations"],
            market_impact_estimate_bps=data["market_impact_estimate_bps"],
        )
