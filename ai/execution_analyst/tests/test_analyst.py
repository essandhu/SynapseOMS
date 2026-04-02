"""Tests for ExecutionAnalyst — Anthropic API integration with mocked client."""

import json
import sys
import os
import time
import pytest
from unittest.mock import MagicMock, patch
from datetime import datetime

# Fix imports
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', '..', '..'))

from ai.execution_analyst.types import TradeContext, ExecutionReport


def _make_trade_context() -> TradeContext:
    """Helper to create a sample TradeContext for tests."""
    return TradeContext(
        side="BUY",
        quantity=5000,
        instrument_id="AAPL",
        asset_class="equity",
        order_type="limit",
        limit_price=185.50,
        submitted_at=datetime(2025, 6, 1, 10, 0, 0),
        completed_at=datetime(2025, 6, 1, 10, 2, 30),
        venues="NYSE, ARCA, BATS",
        fill_count=3,
        fill_table="| Venue | Qty | Price |\n| NYSE | 2000 | 185.45 |",
        arrival_price=185.40,
        spread_bps=1.2,
        vwap_5min=185.48,
        adv_30d=50_000_000,
        size_pct_adv=0.01,
        venue_comparison_table="| Venue | Fill Rate |\n| NYSE | 40% |",
    )


VALID_REPORT_JSON = {
    "overall_grade": "B",
    "implementation_shortfall_bps": 2.5,
    "summary": "Execution was efficient with minimal market impact.",
    "venue_analysis": [
        {"venue": "NYSE", "grade": "A", "comment": "Best fill rate."}
    ],
    "recommendations": ["Consider increasing ARCA allocation."],
    "market_impact_estimate_bps": 1.8,
}


@patch("ai.execution_analyst.analyst.anthropic")
def test_prompt_is_formatted_correctly(mock_anthropic: MagicMock) -> None:
    """The prompt sent to the API must contain key trade fields."""
    mock_client = MagicMock()
    mock_anthropic.Anthropic.return_value = mock_client

    mock_response = MagicMock()
    mock_response.content = [MagicMock(text=json.dumps(VALID_REPORT_JSON))]
    mock_client.messages.create.return_value = mock_response

    from ai.execution_analyst.analyst import ExecutionAnalyst

    analyst = ExecutionAnalyst()
    ctx = _make_trade_context()

    import asyncio
    asyncio.run(analyst.analyze_execution(ctx))

    call_kwargs = mock_client.messages.create.call_args
    prompt_text = call_kwargs.kwargs["messages"][0]["content"]

    assert "AAPL" in prompt_text, "instrument_id missing from prompt"
    assert "BUY" in prompt_text, "side missing from prompt"
    assert "5000" in prompt_text, "quantity missing from prompt"


@patch("ai.execution_analyst.analyst.anthropic")
def test_successful_response_parsing(mock_anthropic: MagicMock) -> None:
    """Valid JSON from the API should parse into a correct ExecutionReport."""
    mock_client = MagicMock()
    mock_anthropic.Anthropic.return_value = mock_client

    mock_response = MagicMock()
    mock_response.content = [MagicMock(text=json.dumps(VALID_REPORT_JSON))]
    mock_client.messages.create.return_value = mock_response

    from ai.execution_analyst.analyst import ExecutionAnalyst

    analyst = ExecutionAnalyst()
    ctx = _make_trade_context()

    import asyncio
    report = asyncio.run(analyst.analyze_execution(ctx))

    assert isinstance(report, ExecutionReport)
    assert report.overall_grade == "B"
    assert report.implementation_shortfall_bps == 2.5
    assert report.market_impact_estimate_bps == 1.8
    assert len(report.venue_analysis) == 1
    assert report.recommendations == ["Consider increasing ARCA allocation."]


@patch("ai.execution_analyst.analyst.anthropic")
def test_json_parse_error_returns_fallback(mock_anthropic: MagicMock) -> None:
    """Non-JSON API response should produce a fallback report with grade N/A."""
    mock_client = MagicMock()
    mock_anthropic.Anthropic.return_value = mock_client

    mock_response = MagicMock()
    mock_response.content = [MagicMock(text="This is not valid JSON at all.")]
    mock_client.messages.create.return_value = mock_response

    from ai.execution_analyst.analyst import ExecutionAnalyst

    analyst = ExecutionAnalyst()
    ctx = _make_trade_context()

    import asyncio
    report = asyncio.run(analyst.analyze_execution(ctx))

    assert report.overall_grade == "N/A"
    assert report.implementation_shortfall_bps == 0.0
    assert "Failed to parse AI response" in report.summary
    assert report.venue_analysis == []
    assert report.recommendations == []


@patch("ai.execution_analyst.analyst.anthropic")
def test_rate_limiting_enforced(mock_anthropic: MagicMock) -> None:
    """The 11th call within an hour should raise RateLimitExceeded."""
    mock_client = MagicMock()
    mock_anthropic.Anthropic.return_value = mock_client

    mock_response = MagicMock()
    mock_response.content = [MagicMock(text=json.dumps(VALID_REPORT_JSON))]
    mock_client.messages.create.return_value = mock_response

    from ai.execution_analyst.analyst import ExecutionAnalyst, RateLimitExceeded

    analyst = ExecutionAnalyst()
    ctx = _make_trade_context()

    import asyncio

    for _ in range(10):
        asyncio.run(analyst.analyze_execution(ctx))

    with pytest.raises(RateLimitExceeded):
        asyncio.run(analyst.analyze_execution(ctx))


@patch("ai.execution_analyst.analyst.anthropic")
def test_rate_limit_resets_after_hour(mock_anthropic: MagicMock) -> None:
    """Timestamps older than 1 hour should be pruned, allowing new calls."""
    mock_client = MagicMock()
    mock_anthropic.Anthropic.return_value = mock_client

    mock_response = MagicMock()
    mock_response.content = [MagicMock(text=json.dumps(VALID_REPORT_JSON))]
    mock_client.messages.create.return_value = mock_response

    from ai.execution_analyst.analyst import ExecutionAnalyst

    analyst = ExecutionAnalyst()

    # Simulate 10 calls that happened over an hour ago
    old_time = time.time() - 3700  # 3700 seconds ago (> 1 hour)
    analyst._call_timestamps = [old_time + i for i in range(10)]

    ctx = _make_trade_context()

    import asyncio
    report = asyncio.run(analyst.analyze_execution(ctx))

    # Should succeed since old timestamps are pruned
    assert report.overall_grade == "B"
    # Only the new call timestamp should remain
    assert len(analyst._call_timestamps) == 1
