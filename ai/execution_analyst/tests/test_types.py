"""Tests for execution analyst domain types."""

from __future__ import annotations

import re
from datetime import datetime, timezone

import pytest

from ai.execution_analyst.prompt_templates import EXECUTION_ANALYSIS_PROMPT
from ai.execution_analyst.types import ExecutionReport, TradeContext


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

def _sample_trade_context() -> TradeContext:
    return TradeContext(
        side="BUY",
        quantity=10_000,
        instrument_id="AAPL",
        asset_class="equity",
        order_type="limit",
        limit_price=185.50,
        submitted_at=datetime(2026, 4, 1, 14, 30, 0, tzinfo=timezone.utc),
        completed_at=datetime(2026, 4, 1, 14, 30, 12, tzinfo=timezone.utc),
        venues="NYSE,ARCA",
        fill_count=3,
        fill_table="| 1 | NYSE | 5000 | 185.48 |\n| 2 | ARCA | 3000 | 185.50 |",
        arrival_price=185.45,
        spread_bps=1.2,
        vwap_5min=185.47,
        adv_30d=50_000_000,
        size_pct_adv=0.02,
        venue_comparison_table="| NYSE | 5000 | 185.48 |\n| ARCA | 3000 | 185.50 |",
    )


def _sample_report_dict() -> dict:
    return {
        "overall_grade": "B",
        "implementation_shortfall_bps": 2.7,
        "summary": "Execution was solid with minimal slippage.",
        "venue_analysis": [
            {"venue": "NYSE", "grade": "A", "comment": "Best fill price."},
            {"venue": "ARCA", "grade": "B", "comment": "Acceptable."},
        ],
        "recommendations": ["Consider adding BATS as venue."],
        "market_impact_estimate_bps": 1.5,
    }


# ---------------------------------------------------------------------------
# TradeContext tests
# ---------------------------------------------------------------------------

class TestTradeContextToPromptVars:
    """to_prompt_vars() must return keys matching every placeholder in the prompt."""

    def test_returns_all_prompt_placeholders(self):
        ctx = _sample_trade_context()
        prompt_vars = ctx.to_prompt_vars()

        # Extract single-brace placeholders (not double-brace JSON)
        placeholders = set(re.findall(r"(?<!\{)\{(\w+)\}(?!\})", EXECUTION_ANALYSIS_PROMPT))

        missing = placeholders - set(prompt_vars.keys())
        assert not missing, f"Missing keys for prompt placeholders: {missing}"

    def test_values_are_non_none(self):
        ctx = _sample_trade_context()
        prompt_vars = ctx.to_prompt_vars()

        for key, value in prompt_vars.items():
            assert value is not None, f"Prompt var '{key}' is None"

    def test_prompt_formats_without_error(self):
        ctx = _sample_trade_context()
        prompt_vars = ctx.to_prompt_vars()

        rendered = EXECUTION_ANALYSIS_PROMPT.format(**prompt_vars)
        assert "BUY" in rendered
        assert "AAPL" in rendered


# ---------------------------------------------------------------------------
# ExecutionReport tests
# ---------------------------------------------------------------------------

class TestExecutionReportRoundTrip:
    """from_dict -> to_dict must round-trip cleanly."""

    def test_round_trip(self):
        original = _sample_report_dict()
        report = ExecutionReport.from_dict(original)
        result = report.to_dict()

        assert result == original

    def test_from_dict_sets_all_fields(self):
        data = _sample_report_dict()
        report = ExecutionReport.from_dict(data)

        assert report.overall_grade == "B"
        assert report.implementation_shortfall_bps == 2.7
        assert report.summary == "Execution was solid with minimal slippage."
        assert len(report.venue_analysis) == 2
        assert report.venue_analysis[0]["venue"] == "NYSE"
        assert report.recommendations == ["Consider adding BATS as venue."]
        assert report.market_impact_estimate_bps == 1.5

    def test_to_dict_returns_plain_dict(self):
        report = ExecutionReport.from_dict(_sample_report_dict())
        d = report.to_dict()

        assert isinstance(d, dict)
        assert set(d.keys()) == {
            "overall_grade",
            "implementation_shortfall_bps",
            "summary",
            "venue_analysis",
            "recommendations",
            "market_impact_estimate_bps",
        }
