"""Prompt templates for AI execution analysis."""

EXECUTION_ANALYSIS_PROMPT = """You are an institutional-grade trade execution analyst.
Analyze the following completed trade and provide a concise execution quality report.

## Trade Summary
- Order: {side} {quantity} {instrument_id} ({asset_class})
- Order type: {order_type}, limit price: {limit_price}
- Submitted: {submitted_at}, completed: {completed_at}
- Venue(s) used: {venues}
- Total fills: {fill_count}

## Fill Details
{fill_table}

## Market Context at Submission
- Arrival price (mid at submission): {arrival_price}
- Spread at submission: {spread_bps} bps
- 5-minute VWAP around execution: {vwap_5min}
- 30-day average daily volume: {adv_30d}
- Order size as % of ADV: {size_pct_adv}%

## Venue Comparison
{venue_comparison_table}

## Instructions
Provide your analysis in the following JSON structure:
{{
  "overall_grade": "A/B/C/D/F",
  "implementation_shortfall_bps": <number>,
  "summary": "<2-3 sentence executive summary>",
  "venue_analysis": [
    {{"venue": "<name>", "grade": "<A-F>", "comment": "<1 sentence>"}}
  ],
  "recommendations": ["<actionable suggestion 1>", "..."],
  "market_impact_estimate_bps": <number>
}}
"""
