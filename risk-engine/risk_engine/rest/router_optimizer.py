"""FastAPI router for portfolio optimization endpoints."""

from __future__ import annotations

from typing import Any

import numpy as np
import structlog
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from risk_engine.domain.portfolio import Portfolio
from risk_engine.optimizer.constraints import OptimizationConstraints
from risk_engine.optimizer.mean_variance import PortfolioOptimizer

logger = structlog.get_logger()

router = APIRouter(prefix="/api/v1/optimizer", tags=["optimizer"])


# ---------------------------------------------------------------------------
# Dependency container
# ---------------------------------------------------------------------------


class OptimizerDependencies:
    """Holds shared state for optimizer route handlers."""

    def __init__(
        self,
        portfolio: Portfolio | None = None,
        optimizer: PortfolioOptimizer | None = None,
        expected_returns: np.ndarray | None = None,
        covariance_matrix: np.ndarray | None = None,
    ) -> None:
        self.portfolio = portfolio or Portfolio()
        self.optimizer = optimizer
        self.expected_returns = expected_returns
        self.covariance_matrix = covariance_matrix


_deps: OptimizerDependencies | None = None


def configure_dependencies(deps: OptimizerDependencies) -> None:
    """Replace the module-level dependency holder."""
    global _deps  # noqa: PLW0603
    _deps = deps


# ---------------------------------------------------------------------------
# Request / response models
# ---------------------------------------------------------------------------


class ConstraintsRequest(BaseModel):
    """JSON body for the optimize endpoint."""

    risk_aversion: float = Field(default=1.0, ge=0.0, description="Risk aversion parameter")
    long_only: bool = Field(default=True, description="Long-only constraint")
    max_single_weight: float | None = Field(
        default=None, ge=0.0, le=1.0, description="Max weight per asset (0-1)"
    )
    sector_limits: dict[str, float] | None = Field(default=None)
    target_volatility: float | None = Field(
        default=None, ge=0.0, description="Max annualised portfolio volatility"
    )
    max_turnover: float | None = Field(
        default=None, ge=0.0, le=2.0, description="Max L1 turnover"
    )
    asset_class_bounds: dict[str, tuple[float, float]] | None = Field(default=None)


# ---------------------------------------------------------------------------
# Endpoints
# ---------------------------------------------------------------------------


@router.post("/optimize")
async def optimize(body: ConstraintsRequest) -> dict[str, Any]:
    """Run portfolio optimization with the given constraints.

    Returns target weights, trade actions, and portfolio statistics.
    """
    if _deps is None or _deps.optimizer is None:
        raise HTTPException(status_code=503, detail="Optimizer not configured")

    portfolio = _deps.portfolio
    positions = list(portfolio.positions.values())

    if not positions:
        raise HTTPException(status_code=422, detail="Portfolio has no positions to optimize")

    if _deps.expected_returns is None or _deps.covariance_matrix is None:
        raise HTTPException(
            status_code=503, detail="Expected returns or covariance matrix not available"
        )

    constraints = OptimizationConstraints(
        risk_aversion=body.risk_aversion,
        long_only=body.long_only,
        max_single_weight=body.max_single_weight,
        sector_limits=body.sector_limits,
        target_volatility=body.target_volatility,
        max_turnover=body.max_turnover,
        asset_class_bounds=body.asset_class_bounds,
    )

    try:
        result = _deps.optimizer.optimize(
            current_positions=positions,
            expected_returns=_deps.expected_returns,
            covariance_matrix=_deps.covariance_matrix,
            constraints=constraints,
        )
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc

    instrument_ids = [p.instrument_id for p in positions]
    target_weights_dict = {
        inst_id: round(float(w), 6)
        for inst_id, w in zip(instrument_ids, result.target_weights)
    }

    trades_list = [
        {
            "instrumentId": t.instrument_id,
            "side": t.side,
            "quantity": str(t.quantity),
            "estimatedCost": str(t.estimated_cost),
        }
        for t in result.trades
    ]

    return {
        "targetWeights": target_weights_dict,
        "trades": trades_list,
        "expectedReturn": round(result.expected_return, 6),
        "expectedVolatility": round(result.expected_volatility, 6),
        "sharpeRatio": round(result.sharpe_ratio, 6),
    }
