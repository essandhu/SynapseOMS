"""FastAPI router for AI-powered endpoints (rebalancing + execution reports)."""

from __future__ import annotations

import os
import sys
from typing import Any

import structlog
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

# Ensure the repo root is importable so ai.* packages resolve.
_REPO_ROOT = os.path.join(os.path.dirname(__file__), "..", "..", "..")
if _REPO_ROOT not in sys.path:
    sys.path.insert(0, _REPO_ROOT)

from ai.execution_analyst.analyst import ExecutionAnalyst
from ai.execution_analyst.types import ExecutionReport, TradeContext
from ai.rebalancing_assistant.assistant import RebalancingAssistant
from ai.rebalancing_assistant.types import ExtractedConstraints, RebalanceRequest

from risk_engine.domain.portfolio import Portfolio
from risk_engine.optimizer.mean_variance import PortfolioOptimizer

import numpy as np

logger = structlog.get_logger()

router = APIRouter(prefix="/api/v1/ai", tags=["ai"])


# ---------------------------------------------------------------------------
# Dependency container
# ---------------------------------------------------------------------------


class AIDependencies:
    """Holds shared state for AI route handlers."""

    def __init__(
        self,
        execution_analyst: ExecutionAnalyst | None = None,
        rebalancing_assistant: RebalancingAssistant | None = None,
        optimizer: PortfolioOptimizer | None = None,
        portfolio: Portfolio | None = None,
        expected_returns: np.ndarray | None = None,
        covariance_matrix: np.ndarray | None = None,
    ) -> None:
        self.execution_analyst = execution_analyst
        self.rebalancing_assistant = rebalancing_assistant
        self.optimizer = optimizer
        self.portfolio = portfolio or Portfolio()
        self.expected_returns = expected_returns
        self.covariance_matrix = covariance_matrix
        self.execution_reports: list[dict] = []


_deps: AIDependencies | None = None


def configure_dependencies(deps: AIDependencies | None) -> None:
    """Replace the module-level dependency holder."""
    global _deps  # noqa: PLW0603
    _deps = deps


# ---------------------------------------------------------------------------
# Request models
# ---------------------------------------------------------------------------


class RebalanceBody(BaseModel):
    """JSON body for the rebalance endpoint."""

    prompt: str


class ExecutionReportBody(BaseModel):
    """JSON body for the execution-report endpoint."""

    side: str
    quantity: int
    instrument_id: str
    asset_class: str
    order_type: str
    limit_price: float
    submitted_at: str
    completed_at: str
    venues: str
    fill_count: int
    fill_table: str
    arrival_price: float
    spread_bps: float
    vwap_5min: float
    adv_30d: int
    size_pct_adv: float
    venue_comparison_table: str


# ---------------------------------------------------------------------------
# Endpoints
# ---------------------------------------------------------------------------


@router.post("/rebalance")
async def rebalance(body: RebalanceBody) -> dict[str, Any]:
    """Extract constraints from natural language and optionally run optimizer."""
    if _deps is None or _deps.rebalancing_assistant is None:
        raise HTTPException(
            status_code=503,
            detail="AI rebalancing not configured (ANTHROPIC_API_KEY not set?)",
        )

    portfolio = _deps.portfolio
    positions = list(portfolio.positions.values())

    # Build portfolio summary
    if not positions:
        portfolio_summary = "No positions"
    else:
        portfolio_summary = "\n".join(
            f"- {p.instrument_id}: qty={p.quantity}, market_value={p.market_value}"
            for p in positions
        )
    available = (
        ", ".join(p.instrument_id for p in positions) if positions else "None"
    )

    request = RebalanceRequest(
        user_input=body.prompt,
        portfolio_summary=portfolio_summary,
        available_instruments=available,
    )

    try:
        # extract_constraints is synchronous in the real RebalancingAssistant
        constraints = _deps.rebalancing_assistant.extract_constraints(request)
    except Exception as exc:
        raise HTTPException(
            status_code=503, detail=f"AI service unavailable: {exc}"
        ) from exc

    # Convert to optimizer constraints
    nav = (
        float(sum(getattr(p, "market_value", 0) for p in positions))
        if positions
        else 1.0
    )
    opt_constraints = constraints.to_optimization_constraints(nav=nav)

    # Run optimizer if configured
    if (
        _deps.optimizer is None
        or _deps.expected_returns is None
        or _deps.covariance_matrix is None
    ):
        return {
            "constraints": _constraints_to_dict(constraints),
            "optimization": None,
            "reasoning": constraints.reasoning,
        }

    try:
        result = _deps.optimizer.optimize(
            current_positions=positions,
            expected_returns=_deps.expected_returns,
            covariance_matrix=_deps.covariance_matrix,
            constraints=opt_constraints,
        )
    except ValueError as exc:
        raise HTTPException(
            status_code=422, detail=f"Infeasible constraints: {exc}"
        ) from exc

    instrument_ids = [p.instrument_id for p in positions]
    return {
        "constraints": _constraints_to_dict(constraints),
        "optimization": {
            "targetWeights": {
                i: round(float(w), 6)
                for i, w in zip(instrument_ids, result.target_weights)
            },
            "trades": [
                {
                    "instrumentId": t.instrument_id,
                    "side": t.side,
                    "quantity": str(t.quantity),
                    "estimatedCost": str(t.estimated_cost),
                }
                for t in result.trades
            ],
            "expectedReturn": round(result.expected_return, 6),
            "expectedVolatility": round(result.expected_volatility, 6),
            "sharpeRatio": round(result.sharpe_ratio, 6),
        },
        "reasoning": constraints.reasoning,
    }


@router.post("/execution-report")
async def create_execution_report(body: ExecutionReportBody) -> dict[str, Any]:
    """Analyse a completed trade execution via AI and store the report."""
    if _deps is None or _deps.execution_analyst is None:
        raise HTTPException(
            status_code=503, detail="AI execution analysis not configured"
        )

    from datetime import datetime

    trade_context = TradeContext(
        side=body.side,
        quantity=body.quantity,
        instrument_id=body.instrument_id,
        asset_class=body.asset_class,
        order_type=body.order_type,
        limit_price=body.limit_price,
        submitted_at=datetime.fromisoformat(body.submitted_at),
        completed_at=datetime.fromisoformat(body.completed_at),
        venues=body.venues,
        fill_count=body.fill_count,
        fill_table=body.fill_table,
        arrival_price=body.arrival_price,
        spread_bps=body.spread_bps,
        vwap_5min=body.vwap_5min,
        adv_30d=body.adv_30d,
        size_pct_adv=body.size_pct_adv,
        venue_comparison_table=body.venue_comparison_table,
    )

    try:
        report = await _deps.execution_analyst.analyze_execution(trade_context)
    except Exception as exc:
        raise HTTPException(
            status_code=503, detail=f"AI service unavailable: {exc}"
        ) from exc

    report_dict = report.to_dict()
    report_dict["orderId"] = body.instrument_id  # placeholder
    report_dict["analyzedAt"] = datetime.now().isoformat()
    _deps.execution_reports.insert(0, report_dict)
    return report_dict


@router.get("/execution-reports")
async def get_execution_reports(limit: int = 20) -> list[dict[str, Any]]:
    """Return the most recent execution reports."""
    if _deps is None:
        raise HTTPException(status_code=503, detail="AI not configured")
    return _deps.execution_reports[:limit]


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _constraints_to_dict(c: ExtractedConstraints) -> dict[str, Any]:
    """Serialize ExtractedConstraints to a camelCase dict for JSON output."""
    return {
        "objective": c.objective,
        "targetReturn": c.target_return,
        "riskAversion": c.risk_aversion,
        "longOnly": c.long_only,
        "maxSingleWeight": c.max_single_weight,
        "assetClassBounds": c.asset_class_bounds,
        "sectorLimits": c.sector_limits,
        "targetVolatility": c.target_volatility,
        "maxTurnoverUsd": c.max_turnover_usd,
        "instrumentsToInclude": c.instruments_to_include,
        "instrumentsToExclude": c.instruments_to_exclude,
        "reasoning": c.reasoning,
    }
