"""Anthropic API integration for AI execution analysis."""

import json
import time

import anthropic

from ai.execution_analyst.prompt_templates import EXECUTION_ANALYSIS_PROMPT
from ai.execution_analyst.types import ExecutionReport, TradeContext


class RateLimitExceeded(Exception):
    """Raised when the hourly analysis limit is exceeded."""


class ExecutionAnalyst:
    """Calls the Anthropic Messages API to produce execution quality reports."""

    def __init__(self) -> None:
        self.client = anthropic.Anthropic()  # Uses ANTHROPIC_API_KEY env var
        self.model = "claude-sonnet-4-6"
        self._call_timestamps: list[float] = []
        self._max_per_hour = 10

    async def analyze_execution(self, trade_context: TradeContext) -> ExecutionReport:
        """Analyse a completed trade and return a structured report.

        Enforces an hourly rate limit before calling the API.  If the
        response cannot be parsed as JSON, a fallback report is returned
        instead of raising.
        """
        self._enforce_rate_limit()

        prompt = EXECUTION_ANALYSIS_PROMPT.format(**trade_context.to_prompt_vars())

        response = self.client.messages.create(
            model=self.model,
            max_tokens=1024,
            messages=[{"role": "user", "content": prompt}],
        )

        self._call_timestamps.append(time.time())

        try:
            report_json = json.loads(response.content[0].text)
            return ExecutionReport.from_dict(report_json)
        except (json.JSONDecodeError, KeyError, IndexError) as exc:
            return ExecutionReport(
                overall_grade="N/A",
                implementation_shortfall_bps=0.0,
                summary=f"Failed to parse AI response: {exc}",
                venue_analysis=[],
                recommendations=[],
                market_impact_estimate_bps=0.0,
            )

    def _enforce_rate_limit(self) -> None:
        """Prune old timestamps and reject if hourly cap is reached."""
        now = time.time()
        cutoff = now - 3600
        self._call_timestamps = [t for t in self._call_timestamps if t > cutoff]
        if len(self._call_timestamps) >= self._max_per_hour:
            raise RateLimitExceeded(
                f"Max {self._max_per_hour} analyses per hour"
            )
