"""Anthropic API integration for the AI rebalancing assistant."""

from __future__ import annotations

import json

import anthropic

from ai.rebalancing_assistant.prompt_templates import CONSTRAINT_EXTRACTION_PROMPT
from ai.rebalancing_assistant.types import ExtractedConstraints, RebalanceRequest


class RebalancingAssistant:
    """Extracts structured portfolio constraints from natural-language requests
    using the Anthropic Messages API."""

    def __init__(self) -> None:
        self.client = anthropic.Anthropic()  # Uses ANTHROPIC_API_KEY env var
        self.model = "claude-sonnet-4-6"

    def extract_constraints(self, request: RebalanceRequest) -> ExtractedConstraints:
        """Send the user request to Claude and parse structured constraints.

        Parameters
        ----------
        request:
            The rebalancing request containing user input and portfolio context.

        Returns
        -------
        ExtractedConstraints
            Parsed constraints, or conservative defaults if parsing fails.
        """
        prompt = CONSTRAINT_EXTRACTION_PROMPT.format(**request.to_prompt_vars())
        response = self.client.messages.create(
            model=self.model,
            max_tokens=1024,
            messages=[{"role": "user", "content": prompt}],
        )
        try:
            data = json.loads(response.content[0].text)
            constraints = ExtractedConstraints.from_dict(data)
            self._validate(constraints, request)
            return constraints
        except (json.JSONDecodeError, KeyError, IndexError) as exc:
            return ExtractedConstraints(
                objective="minimize_variance",
                target_return=None,
                risk_aversion=1.0,
                long_only=True,
                max_single_weight=None,
                asset_class_bounds=None,
                sector_limits=None,
                target_volatility=None,
                max_turnover_usd=None,
                instruments_to_include=None,
                instruments_to_exclude=None,
                reasoning=f"Failed to parse AI response: {exc}. Using conservative defaults.",
            )

    def _validate(
        self, constraints: ExtractedConstraints, request: RebalanceRequest
    ) -> None:
        """Validate extracted constraints for feasibility.

        Appends warning messages to ``constraints.reasoning`` for any issues
        found (e.g. infeasible bounds, unknown instruments).
        """
        if constraints.asset_class_bounds:
            for cls, bounds in constraints.asset_class_bounds.items():
                if bounds[0] > bounds[1]:
                    constraints.reasoning += (
                        f" Warning: {cls} bounds [{bounds[0]}, {bounds[1]}]"
                        f" are infeasible (min > max)."
                    )

        # Validate instruments exist in available list
        available = request.available_instruments.lower()
        if constraints.instruments_to_include:
            for inst in constraints.instruments_to_include:
                if inst.lower() not in available:
                    constraints.reasoning += (
                        f" Warning: {inst} not found in available instruments."
                    )
