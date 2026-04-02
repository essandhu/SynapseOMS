"""Tests for the RebalancingAssistant Anthropic API integration."""

from __future__ import annotations

import json
from unittest.mock import MagicMock, patch

import pytest

from ai.rebalancing_assistant.types import ExtractedConstraints, RebalanceRequest


def _make_request() -> RebalanceRequest:
    return RebalanceRequest(
        user_input="Maximize Sharpe ratio, no more than 30% in any single name",
        portfolio_summary="60% equities, 40% bonds, NAV $1M",
        available_instruments="AAPL, MSFT, GOOG, BND, TLT",
    )


def _mock_response(text: str) -> MagicMock:
    """Build a mock Anthropic Messages response with the given text content."""
    block = MagicMock()
    block.text = text
    response = MagicMock()
    response.content = [block]
    return response


VALID_JSON = json.dumps(
    {
        "objective": "maximize_sharpe",
        "target_return": None,
        "risk_aversion": 0.8,
        "long_only": True,
        "max_single_weight": 0.30,
        "asset_class_bounds": None,
        "sector_limits": None,
        "target_volatility": None,
        "max_turnover_usd": None,
        "instruments_to_include": None,
        "instruments_to_exclude": None,
        "reasoning": "User wants max Sharpe with 30% cap per name.",
    }
)


@patch("ai.rebalancing_assistant.assistant.anthropic")
def test_prompt_includes_portfolio_context(mock_anthropic: MagicMock) -> None:
    """The prompt sent to the API must contain portfolio_summary and user_input."""
    mock_client = MagicMock()
    mock_anthropic.Anthropic.return_value = mock_client
    mock_client.messages.create.return_value = _mock_response(VALID_JSON)

    from ai.rebalancing_assistant.assistant import RebalancingAssistant

    assistant = RebalancingAssistant()
    request = _make_request()
    assistant.extract_constraints(request)

    call_kwargs = mock_client.messages.create.call_args
    prompt_content = call_kwargs.kwargs["messages"][0]["content"]
    assert request.portfolio_summary in prompt_content
    assert request.user_input in prompt_content


@patch("ai.rebalancing_assistant.assistant.anthropic")
def test_successful_json_parsing(mock_anthropic: MagicMock) -> None:
    """Valid JSON from the API should be parsed into ExtractedConstraints."""
    mock_client = MagicMock()
    mock_anthropic.Anthropic.return_value = mock_client
    mock_client.messages.create.return_value = _mock_response(VALID_JSON)

    from ai.rebalancing_assistant.assistant import RebalancingAssistant

    assistant = RebalancingAssistant()
    result = assistant.extract_constraints(_make_request())

    assert isinstance(result, ExtractedConstraints)
    assert result.objective == "maximize_sharpe"
    assert result.max_single_weight == 0.30
    assert result.risk_aversion == 0.8
    assert result.long_only is True
    assert "max Sharpe" in result.reasoning


@patch("ai.rebalancing_assistant.assistant.anthropic")
def test_json_parse_error_returns_defaults(mock_anthropic: MagicMock) -> None:
    """Non-JSON response should return conservative default constraints."""
    mock_client = MagicMock()
    mock_anthropic.Anthropic.return_value = mock_client
    mock_client.messages.create.return_value = _mock_response(
        "Sorry, I cannot parse that request."
    )

    from ai.rebalancing_assistant.assistant import RebalancingAssistant

    assistant = RebalancingAssistant()
    result = assistant.extract_constraints(_make_request())

    assert result.objective == "minimize_variance"
    assert result.risk_aversion == 1.0
    assert result.long_only is True
    assert result.target_return is None
    assert "Failed to parse AI response" in result.reasoning


def test_validation_catches_infeasible_bounds() -> None:
    """Constraints with min > max in asset_class_bounds should produce a warning."""
    from ai.rebalancing_assistant.assistant import RebalancingAssistant

    constraints = ExtractedConstraints(
        objective="minimize_variance",
        target_return=None,
        risk_aversion=1.0,
        long_only=True,
        max_single_weight=None,
        asset_class_bounds={"equity": [0.8, 0.2]},  # min > max
        sector_limits=None,
        target_volatility=None,
        max_turnover_usd=None,
        instruments_to_include=None,
        instruments_to_exclude=None,
        reasoning="Initial reasoning.",
    )
    request = _make_request()

    assistant = RebalancingAssistant.__new__(RebalancingAssistant)
    assistant._validate(constraints, request)

    assert "infeasible" in constraints.reasoning.lower()
    assert "equity" in constraints.reasoning
