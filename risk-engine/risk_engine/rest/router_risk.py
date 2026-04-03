"""FastAPI router exposing risk metrics and portfolio state via REST endpoints."""

from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal
from typing import Any

import numpy as np
import pandas as pd
import structlog
from fastapi import APIRouter, Depends

from risk_engine.concentration.analyzer import ConcentrationAnalyzer
from risk_engine.domain.portfolio import Portfolio
from risk_engine.domain.risk_result import VaRResult
from risk_engine.greeks.calculator import GreeksCalculator
from risk_engine.settlement.tracker import SettlementTracker
from risk_engine.var.historical import HistoricalVaR
from risk_engine.var.monte_carlo import DistributionParams, MonteCarloVaR
from risk_engine.var.parametric import ParametricVaR

logger = structlog.get_logger()

router = APIRouter(prefix="/api/v1", tags=["risk"])


# ---------------------------------------------------------------------------
# Dependency container
# ---------------------------------------------------------------------------


class RiskDependencies:
    """Holds shared state injected into route handlers via FastAPI dependencies.

    Instantiate once during application startup and call ``register(app)``
    to wire the dependency overrides, or use the ``get_*`` helpers directly.
    """

    def __init__(
        self,
        portfolio: Portfolio | None = None,
        historical_var: HistoricalVaR | None = None,
        parametric_var: ParametricVaR | None = None,
        settlement_tracker: SettlementTracker | None = None,
        returns_matrix: pd.DataFrame | None = None,
        monte_carlo_var: MonteCarloVaR | None = None,
        greeks_calculator: GreeksCalculator | None = None,
        concentration_analyzer: ConcentrationAnalyzer | None = None,
    ) -> None:
        self.portfolio = portfolio or Portfolio()
        self.historical_var = historical_var or HistoricalVaR()
        self.parametric_var = parametric_var or ParametricVaR()
        self.settlement_tracker = settlement_tracker or SettlementTracker()
        self.returns_matrix = returns_matrix if returns_matrix is not None else pd.DataFrame()
        self.monte_carlo_var = monte_carlo_var
        self.greeks_calculator = greeks_calculator
        self.concentration_analyzer = concentration_analyzer

    # Dependency callables -------------------------------------------------

    def get_portfolio(self) -> Portfolio:
        return self.portfolio

    def get_historical_var(self) -> HistoricalVaR:
        return self.historical_var

    def get_parametric_var(self) -> ParametricVaR:
        return self.parametric_var

    def get_settlement_tracker(self) -> SettlementTracker:
        return self.settlement_tracker

    def get_returns_matrix(self) -> pd.DataFrame:
        return self.returns_matrix

    def get_monte_carlo_var(self) -> MonteCarloVaR | None:
        return self.monte_carlo_var

    def get_greeks_calculator(self) -> GreeksCalculator | None:
        return self.greeks_calculator

    def get_concentration_analyzer(self) -> ConcentrationAnalyzer | None:
        return self.concentration_analyzer


# Module-level singleton — replaced by ``configure_dependencies``.
_deps = RiskDependencies()


def configure_dependencies(deps: RiskDependencies) -> None:
    """Replace the module-level dependency holder.

    Call this at startup (in ``main.py``) to inject real objects.
    """
    global _deps  # noqa: PLW0603
    _deps = deps


# FastAPI-compatible dependency functions (zero-arg callables)
def _get_portfolio() -> Portfolio:
    return _deps.get_portfolio()


def _get_historical_var() -> HistoricalVaR:
    return _deps.get_historical_var()


def _get_parametric_var() -> ParametricVaR:
    return _deps.get_parametric_var()


def _get_settlement_tracker() -> SettlementTracker:
    return _deps.get_settlement_tracker()


def _get_returns_matrix() -> pd.DataFrame:
    return _deps.get_returns_matrix()


def _get_monte_carlo_var() -> MonteCarloVaR | None:
    return _deps.get_monte_carlo_var()


def _get_greeks_calculator() -> GreeksCalculator | None:
    return _deps.get_greeks_calculator()


def _get_concentration_analyzer() -> ConcentrationAnalyzer | None:
    return _deps.get_concentration_analyzer()


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _safe_compute_var(
    engine: HistoricalVaR | ParametricVaR,
    portfolio: Portfolio,
    returns_matrix: pd.DataFrame,
) -> VaRResult | None:
    """Compute VaR, returning *None* when there is insufficient data."""
    if returns_matrix.empty or not portfolio.positions:
        return None
    try:
        return engine.compute(portfolio.positions, returns_matrix)
    except Exception:
        logger.exception("var_computation_failed", method=type(engine).__name__)
        return None


def _safe_compute_mc_var(
    engine: MonteCarloVaR,
    portfolio: Portfolio,
    returns_matrix: pd.DataFrame,
) -> VaRResult | None:
    """Compute Monte Carlo VaR, returning *None* on failure.

    Derives covariance matrix and expected returns from the returns matrix,
    and assigns distribution families based on asset class (Student-t for
    crypto, normal for everything else).
    """
    if returns_matrix.empty or not portfolio.positions:
        return None
    try:
        positions = list(portfolio.positions.values())
        instrument_ids = [p.instrument_id for p in positions]

        # Filter returns matrix to matching instruments
        available_cols = [c for c in instrument_ids if c in returns_matrix.columns]
        if not available_cols:
            return None

        sub_returns = returns_matrix[available_cols]
        cov_matrix = sub_returns.cov().values
        expected_rets = sub_returns.mean().values

        # Build distribution params based on asset class
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
        logger.exception("mc_var_computation_failed")
        return None


def _decimal_str(value: Decimal | None) -> str | None:
    """Format a Decimal as a string, or return None."""
    if value is None:
        return None
    return str(value)


# ---------------------------------------------------------------------------
# Risk endpoints
# ---------------------------------------------------------------------------


@router.get("/risk/var")
async def get_var(
    portfolio: Portfolio = Depends(_get_portfolio),
    historical_var: HistoricalVaR = Depends(_get_historical_var),
    parametric_var: ParametricVaR = Depends(_get_parametric_var),
    returns_matrix: pd.DataFrame = Depends(_get_returns_matrix),
    monte_carlo_var: MonteCarloVaR | None = Depends(_get_monte_carlo_var),
) -> dict[str, Any]:
    """Return current Value-at-Risk across methods."""
    hist_result = _safe_compute_var(historical_var, portfolio, returns_matrix)
    para_result = _safe_compute_var(parametric_var, portfolio, returns_matrix)

    # Monte Carlo VaR computation
    mc_result: VaRResult | None = None
    if monte_carlo_var is not None and not returns_matrix.empty and portfolio.positions:
        mc_result = _safe_compute_mc_var(monte_carlo_var, portfolio, returns_matrix)

    # Pick the best available CVaR (prefer historical)
    cvar: Decimal | None = None
    if hist_result is not None:
        cvar = hist_result.cvar_amount
    elif para_result is not None:
        cvar = para_result.cvar_amount
    elif mc_result is not None:
        cvar = mc_result.cvar_amount

    # Confidence / horizon from whichever engine is configured
    confidence = historical_var.confidence
    now = datetime.now(timezone.utc)

    return {
        "historicalVaR": _decimal_str(hist_result.var_amount) if hist_result else None,
        "parametricVaR": _decimal_str(para_result.var_amount) if para_result else None,
        "monteCarloVaR": _decimal_str(mc_result.var_amount) if mc_result else None,
        "cvar": _decimal_str(cvar),
        "confidence": confidence,
        "horizon": "1d",
        "computedAt": (hist_result.computed_at if hist_result else now).isoformat().replace("+00:00", "Z"),
        "monteCarloDistribution": mc_result.distribution if mc_result else None,
    }


@router.get("/risk/drawdown")
async def get_drawdown(
    portfolio: Portfolio = Depends(_get_portfolio),
    returns_matrix: pd.DataFrame = Depends(_get_returns_matrix),
) -> dict[str, Any]:
    """Return current and historical drawdown data."""
    nav = portfolio.compute_nav()

    # Build a simple drawdown history from the returns matrix if available.
    history: list[dict[str, Any]] = []
    peak = nav
    trough = nav
    current_dd = 0.0

    if not returns_matrix.empty:
        # Reconstruct a NAV series from portfolio returns
        # (simplified: use column means as a proxy for portfolio return per day)
        daily_returns = returns_matrix.mean(axis=1)
        nav_series: list[float] = []
        running_nav = float(nav) if nav > 0 else 100_000.0

        # Walk backwards to build the series, then reverse
        values = daily_returns.values
        for r in reversed(values):
            running_nav = running_nav / (1.0 + float(r)) if (1.0 + float(r)) != 0 else running_nav
            nav_series.append(running_nav)
        nav_series.reverse()
        nav_series.append(float(nav))  # current point

        dates = list(returns_matrix.index)

        running_peak = nav_series[0]
        for i, val in enumerate(nav_series):
            if val > running_peak:
                running_peak = val
            dd = (val - running_peak) / running_peak if running_peak > 0 else 0.0
            if i < len(dates):
                dt = dates[i]
                date_str = str(dt.date()) if hasattr(dt, "date") else str(dt)
                history.append({"date": date_str, "drawdown": round(dd, 6)})

        # Current drawdown from series
        if nav_series:
            series_peak = max(nav_series)
            series_trough = min(nav_series)
            peak = Decimal(str(round(series_peak, 2)))
            trough = Decimal(str(round(series_trough, 2)))
            current_dd = round(
                (float(nav) - series_peak) / series_peak if series_peak > 0 else 0.0,
                6,
            )

    return {
        "current": current_dd,
        "peak": str(peak),
        "trough": str(trough),
        "history": history,
    }


@router.get("/risk/settlement")
async def get_settlement(
    settlement_tracker: SettlementTracker = Depends(_get_settlement_tracker),
) -> dict[str, Any]:
    """Return settlement risk timeline."""
    risk = settlement_tracker.compute_settlement_risk()

    # Build detailed entries from the tracker's internal pending list
    entries: list[dict[str, Any]] = []
    with settlement_tracker._lock:
        for s in settlement_tracker._pending:
            if s.status != "pending":
                continue
            entries.append({
                "date": str(s.settlement_date),
                "amount": str(abs(s.amount)),
                "instrumentId": s.instrument_id,
                "assetClass": s.asset_class,
            })

    return {
        "totalUnsettled": str(risk["total_unsettled"]),
        "entries": entries,
    }


@router.get("/risk/greeks")
async def get_greeks(
    portfolio: Portfolio = Depends(_get_portfolio),
    greeks_calculator: GreeksCalculator | None = Depends(_get_greeks_calculator),
) -> dict[str, Any]:
    """Return portfolio Greeks (total and per-instrument)."""
    if greeks_calculator is None:
        return {
            "error": "Greeks calculator not configured",
            "total": None,
            "byInstrument": {},
            "computedAt": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        }

    positions = list(portfolio.positions.values())
    nav = float(portfolio.nav)

    result = greeks_calculator.compute(positions, nav=nav)

    def _greeks_dict(g: Any) -> dict[str, float]:
        return {
            "delta": round(g.delta, 6),
            "gamma": round(g.gamma, 6),
            "vega": round(g.vega, 6),
            "theta": round(g.theta, 6),
            "rho": round(g.rho, 6),
        }

    return {
        "total": _greeks_dict(result.total),
        "byInstrument": {
            inst_id: _greeks_dict(g)
            for inst_id, g in result.by_instrument.items()
        },
        "computedAt": result.computed_at.isoformat().replace("+00:00", "Z"),
    }


@router.get("/risk/concentration")
async def get_concentration(
    portfolio: Portfolio = Depends(_get_portfolio),
    concentration_analyzer: ConcentrationAnalyzer | None = Depends(_get_concentration_analyzer),
) -> dict[str, Any]:
    """Return concentration risk breakdown."""
    if concentration_analyzer is None:
        return {
            "error": "Concentration analyzer not configured",
            "singleName": {},
            "byAssetClass": {},
            "byVenue": {},
            "hhi": 0.0,
            "warnings": [],
        }

    result = concentration_analyzer.analyze(portfolio)

    return {
        "singleName": result.single_name,
        "byAssetClass": result.by_asset_class,
        "byVenue": result.by_venue,
        "hhi": result.hhi,
        "warnings": result.warnings,
    }


# ---------------------------------------------------------------------------
# Portfolio endpoints
# ---------------------------------------------------------------------------


@router.get("/portfolio")
async def get_portfolio(
    portfolio: Portfolio = Depends(_get_portfolio),
) -> dict[str, Any]:
    """Return current portfolio state."""
    total_pnl = Decimal("0")
    with portfolio._lock:
        for pos in portfolio.positions.values():
            total_pnl += pos.unrealized_pnl + pos.realized_pnl
        position_count = len(portfolio.positions)

        return {
            "totalNav": str(portfolio.nav),
            "totalPnl": str(total_pnl),
            "dailyPnl": str(total_pnl),
            "positionCount": position_count,
        }


@router.get("/portfolio/exposure")
async def get_exposure(
    portfolio: Portfolio = Depends(_get_portfolio),
) -> dict[str, Any]:
    """Return exposure breakdown by asset class and venue."""
    by_asset_class: dict[str, Decimal] = {}
    by_venue: dict[str, Decimal] = {}

    with portfolio._lock:
        for pos in portfolio.positions.values():
            mv = pos.market_value
            by_asset_class[pos.asset_class] = (
                by_asset_class.get(pos.asset_class, Decimal("0")) + mv
            )
            by_venue[pos.venue_id] = (
                by_venue.get(pos.venue_id, Decimal("0")) + mv
            )

    total = sum(abs(v) for v in by_asset_class.values()) or Decimal("1")
    venue_total = sum(abs(v) for v in by_venue.values()) or Decimal("1")

    return {
        "byAssetClass": [
            {
                "assetClass": k,
                "notional": str(v),
                "percentage": float(abs(v) / total * 100),
            }
            for k, v in by_asset_class.items()
        ],
        "byVenue": [
            {
                "venueId": k,
                "notional": str(v),
                "percentage": float(abs(v) / venue_total * 100),
            }
            for k, v in by_venue.items()
        ],
    }
