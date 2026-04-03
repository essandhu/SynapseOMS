"""FastAPI router for what-if scenario analysis."""

from __future__ import annotations

import copy
from decimal import Decimal
from typing import Any

import pandas as pd
import structlog
from fastapi import APIRouter, Depends
from pydantic import BaseModel, field_validator

from risk_engine.domain.portfolio import Portfolio
from risk_engine.domain.position import Position
from risk_engine.domain.risk_result import VaRResult
from risk_engine.var.historical import HistoricalVaR
from risk_engine.var.monte_carlo import DistributionParams, MonteCarloVaR
from risk_engine.var.parametric import ParametricVaR

logger = structlog.get_logger()

router = APIRouter(prefix="/api/v1", tags=["scenario"])


# ---------------------------------------------------------------------------
# Request / response models
# ---------------------------------------------------------------------------


class HypotheticalPosition(BaseModel):
    instrument_id: str
    side: str
    quantity: str
    price: str

    @field_validator("side")
    @classmethod
    def validate_side(cls, v: str) -> str:
        if v.lower() not in ("buy", "sell"):
            raise ValueError("side must be 'buy' or 'sell'")
        return v.lower()


class ScenarioRequest(BaseModel):
    hypothetical_positions: list[HypotheticalPosition]

    @field_validator("hypothetical_positions")
    @classmethod
    def validate_not_empty(cls, v: list) -> list:
        if not v:
            raise ValueError("hypothetical_positions must not be empty")
        return v


# ---------------------------------------------------------------------------
# Dependency container
# ---------------------------------------------------------------------------


class ScenarioDependencies:
    def __init__(
        self,
        portfolio: Portfolio | None = None,
        historical_var: HistoricalVaR | None = None,
        parametric_var: ParametricVaR | None = None,
        monte_carlo_var: MonteCarloVaR | None = None,
        returns_matrix: pd.DataFrame | None = None,
    ) -> None:
        self.portfolio = portfolio or Portfolio()
        self.historical_var = historical_var or HistoricalVaR()
        self.parametric_var = parametric_var or ParametricVaR()
        self.monte_carlo_var = monte_carlo_var
        self.returns_matrix = (
            returns_matrix if returns_matrix is not None else pd.DataFrame()
        )


_deps = ScenarioDependencies()


def configure_dependencies(deps: ScenarioDependencies) -> None:
    global _deps  # noqa: PLW0603
    _deps = deps


def _get_deps() -> ScenarioDependencies:
    return _deps


# ---------------------------------------------------------------------------
# VaR computation helpers
# ---------------------------------------------------------------------------


def _compute_var(
    engine: HistoricalVaR | ParametricVaR,
    portfolio: Portfolio,
    returns_matrix: pd.DataFrame,
) -> VaRResult | None:
    if returns_matrix.empty or not portfolio.positions:
        return None
    try:
        return engine.compute(portfolio.positions, returns_matrix)
    except Exception:
        logger.exception("scenario_var_failed", method=type(engine).__name__)
        return None


def _compute_mc_var(
    engine: MonteCarloVaR,
    portfolio: Portfolio,
    returns_matrix: pd.DataFrame,
) -> VaRResult | None:
    if returns_matrix.empty or not portfolio.positions:
        return None
    try:
        positions = list(portfolio.positions.values())
        instrument_ids = [p.instrument_id for p in positions]
        available_cols = [c for c in instrument_ids if c in returns_matrix.columns]
        if not available_cols:
            return None

        sub_returns = returns_matrix[available_cols]
        cov_matrix = sub_returns.cov().values
        expected_rets = sub_returns.mean().values

        dist_params: list[DistributionParams] = []
        filtered_positions: list = []
        for p in positions:
            if p.instrument_id in available_cols:
                filtered_positions.append(p)
                if p.asset_class == "crypto":
                    dist_params.append(DistributionParams(family="student_t", df=4.0))
                else:
                    dist_params.append(DistributionParams(family="normal"))

        return engine.compute(
            positions=filtered_positions,
            covariance_matrix=cov_matrix,
            expected_returns=expected_rets,
            distribution_params=dist_params,
        )
    except Exception:
        logger.exception("scenario_mc_var_failed")
        return None


def _var_str(result: VaRResult | None) -> str | None:
    return str(result.var_amount) if result else None


def _build_projected_portfolio(
    current: Portfolio, hypothetical: list[HypotheticalPosition]
) -> Portfolio:
    """Clone the current portfolio and add hypothetical positions."""
    projected = Portfolio(
        cash=current.cash,
        available_cash=current.available_cash,
        unsettled_cash=current.unsettled_cash,
    )
    # Deep copy existing positions
    for pid, pos in current.positions.items():
        projected.positions[pid] = Position(
            instrument_id=pos.instrument_id,
            venue_id=pos.venue_id,
            quantity=pos.quantity,
            average_cost=pos.average_cost,
            market_price=pos.market_price,
            unrealized_pnl=pos.unrealized_pnl,
            realized_pnl=pos.realized_pnl,
            asset_class=pos.asset_class,
            settlement_cycle=pos.settlement_cycle,
        )

    for hp in hypothetical:
        qty = Decimal(hp.quantity)
        if hp.side == "sell":
            qty = -qty
        price = Decimal(hp.price)
        key = hp.instrument_id

        if key in projected.positions:
            existing = projected.positions[key]
            new_qty = existing.quantity + qty
            projected.positions[key] = Position(
                instrument_id=key,
                venue_id=existing.venue_id,
                quantity=new_qty,
                average_cost=existing.average_cost,
                market_price=price,
                unrealized_pnl=(price - existing.average_cost) * new_qty,
                realized_pnl=existing.realized_pnl,
                asset_class=existing.asset_class,
                settlement_cycle=existing.settlement_cycle,
            )
        else:
            projected.positions[key] = Position(
                instrument_id=key,
                venue_id="hypothetical",
                quantity=qty,
                average_cost=price,
                market_price=price,
                unrealized_pnl=Decimal("0"),
                realized_pnl=Decimal("0"),
                asset_class="equity",
                settlement_cycle="T2",
            )

    projected.compute_nav()
    return projected


# ---------------------------------------------------------------------------
# Endpoint
# ---------------------------------------------------------------------------


@router.post("/risk/scenario")
async def post_scenario(
    request: ScenarioRequest,
    deps: ScenarioDependencies = Depends(_get_deps),
) -> dict[str, Any]:
    """Compute projected VaR for portfolio with hypothetical positions added."""
    portfolio = deps.portfolio
    returns_matrix = deps.returns_matrix

    # Current VaR
    curr_hist = _compute_var(deps.historical_var, portfolio, returns_matrix)
    curr_para = _compute_var(deps.parametric_var, portfolio, returns_matrix)
    curr_mc = (
        _compute_mc_var(deps.monte_carlo_var, portfolio, returns_matrix)
        if deps.monte_carlo_var
        else None
    )

    # Projected VaR
    projected = _build_projected_portfolio(
        portfolio, request.hypothetical_positions
    )
    proj_hist = _compute_var(deps.historical_var, projected, returns_matrix)
    proj_para = _compute_var(deps.parametric_var, projected, returns_matrix)
    proj_mc = (
        _compute_mc_var(deps.monte_carlo_var, projected, returns_matrix)
        if deps.monte_carlo_var
        else None
    )

    def _delta(a: VaRResult | None, b: VaRResult | None) -> str | None:
        if a and b:
            return str(a.var_amount - b.var_amount)
        return None

    return {
        "current_var": {
            "historical": _var_str(curr_hist),
            "parametric": _var_str(curr_para),
            "monte_carlo": _var_str(curr_mc),
        },
        "projected_var": {
            "historical": _var_str(proj_hist),
            "parametric": _var_str(proj_para),
            "monte_carlo": _var_str(proj_mc),
        },
        "var_delta": {
            "historical": _delta(proj_hist, curr_hist),
            "parametric": _delta(proj_para, curr_para),
            "monte_carlo": _delta(proj_mc, curr_mc),
        },
        "hypothetical_positions_added": len(request.hypothetical_positions),
    }
