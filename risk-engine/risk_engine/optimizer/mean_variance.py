"""Mean-variance portfolio optimizer using cvxpy."""

from __future__ import annotations

from dataclasses import dataclass, field
from decimal import Decimal, ROUND_HALF_UP
from typing import Sequence

import cvxpy as cp
import numpy as np

from risk_engine.domain.position import Position
from risk_engine.optimizer.constraints import OptimizationConstraints


# ---------------------------------------------------------------------------
# Result dataclasses
# ---------------------------------------------------------------------------


@dataclass
class TradeAction:
    """A single buy / sell instruction derived from the weight diff."""

    instrument_id: str
    side: str  # "buy" or "sell"
    quantity: Decimal
    estimated_cost: Decimal


@dataclass
class OptimizationResult:
    """Output of the portfolio optimiser."""

    target_weights: np.ndarray
    trades: list[TradeAction]
    expected_return: float
    expected_volatility: float
    sharpe_ratio: float


# ---------------------------------------------------------------------------
# Optimizer
# ---------------------------------------------------------------------------


class PortfolioOptimizer:
    """Mean-variance optimizer backed by cvxpy / ECOS.

    Parameters
    ----------
    risk_free_rate:
        Annualised risk-free rate used for Sharpe ratio computation.
    """

    def __init__(self, risk_free_rate: float = 0.0) -> None:
        self.risk_free_rate = risk_free_rate

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def optimize(
        self,
        current_positions: Sequence[Position],
        expected_returns: np.ndarray,
        covariance_matrix: np.ndarray,
        constraints: OptimizationConstraints,
    ) -> OptimizationResult:
        """Run mean-variance optimisation and return target weights + trades.

        Parameters
        ----------
        current_positions:
            List of :class:`Position` objects representing the current
            portfolio.  The ordering must match the rows/columns of
            *expected_returns* and *covariance_matrix*.
        expected_returns:
            1-D array of annualised expected returns, length *n*.
        covariance_matrix:
            *n x n* annualised covariance matrix (must be PSD).
        constraints:
            :class:`OptimizationConstraints` governing the optimisation.

        Returns
        -------
        OptimizationResult
            Target weights, trade list, and portfolio statistics.

        Raises
        ------
        ValueError
            If the problem is infeasible or the solver fails.
        """
        n = len(current_positions)
        if n == 0:
            raise ValueError("current_positions must not be empty")

        # Ensure covariance matrix is symmetric PSD for the solver
        cov = np.array(covariance_matrix, dtype=np.float64)
        cov = (cov + cov.T) / 2  # enforce symmetry

        mu = np.array(expected_returns, dtype=np.float64).flatten()

        current_weights = self._compute_current_weights(current_positions)

        # --- cvxpy problem --------------------------------------------------
        w = cp.Variable(n)
        portfolio_return = mu @ w
        portfolio_risk = cp.quad_form(w, cov)

        objective = cp.Maximize(
            portfolio_return - constraints.risk_aversion * portfolio_risk
        )

        constraint_list: list[cp.Constraint] = [cp.sum(w) == 1]

        if constraints.long_only:
            constraint_list.append(w >= 0)

        if constraints.max_single_weight is not None:
            constraint_list.append(w <= constraints.max_single_weight)

        if constraints.target_volatility is not None:
            constraint_list.append(
                cp.sqrt(portfolio_risk) <= constraints.target_volatility
            )

        if constraints.max_turnover is not None:
            constraint_list.append(
                cp.norm(w - current_weights, 1) <= constraints.max_turnover
            )

        if constraints.asset_class_bounds is not None:
            for ac, (lo, hi) in constraints.asset_class_bounds.items():
                mask = self._asset_class_mask(current_positions, ac)
                constraint_list.append(w @ mask >= lo)
                constraint_list.append(w @ mask <= hi)

        problem = cp.Problem(objective, constraint_list)
        self._solve(problem)

        if problem.status in ("infeasible", "infeasible_inaccurate"):
            raise ValueError(
                f"Optimization infeasible (status={problem.status}). "
                "Check that the constraints are not contradictory."
            )
        if problem.status not in ("optimal", "optimal_inaccurate"):
            raise ValueError(
                f"Optimization failed with solver status: {problem.status}"
            )

        target_weights = np.array(w.value, dtype=np.float64).flatten()

        # Portfolio statistics
        exp_ret = float(mu @ target_weights)
        exp_vol = float(np.sqrt(target_weights @ cov @ target_weights))
        sharpe = (
            (exp_ret - self.risk_free_rate) / exp_vol if exp_vol > 1e-12 else 0.0
        )

        trades = self._compute_trades(
            current_positions, current_weights, target_weights
        )

        return OptimizationResult(
            target_weights=target_weights,
            trades=trades,
            expected_return=exp_ret,
            expected_volatility=exp_vol,
            sharpe_ratio=sharpe,
        )

    # ------------------------------------------------------------------
    # Solver selection
    # ------------------------------------------------------------------

    # Preferred solver order: ECOS (spec), then modern alternatives.
    _SOLVER_PREFERENCE = ["ECOS", "CLARABEL", "SCS"]

    @classmethod
    def _solve(cls, problem: cp.Problem) -> None:
        """Solve *problem* using the first available solver."""
        installed = cp.installed_solvers()
        for name in cls._SOLVER_PREFERENCE:
            if name in installed:
                problem.solve(solver=name)
                return
        # Last resort: let cvxpy pick
        problem.solve()

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    @staticmethod
    def _compute_current_weights(positions: Sequence[Position]) -> np.ndarray:
        """Derive weight vector from position market values."""
        market_values = np.array(
            [float(p.market_value) for p in positions], dtype=np.float64
        )
        total = market_values.sum()
        if total == 0:
            return np.zeros(len(positions), dtype=np.float64)
        return market_values / total

    @staticmethod
    def _asset_class_mask(
        positions: Sequence[Position], asset_class: str
    ) -> np.ndarray:
        """Return a boolean-like float mask (1.0 / 0.0) for *asset_class*."""
        return np.array(
            [1.0 if p.asset_class == asset_class else 0.0 for p in positions],
            dtype=np.float64,
        )

    @staticmethod
    def _compute_trades(
        positions: Sequence[Position],
        current_weights: np.ndarray,
        target_weights: np.ndarray,
        tolerance: float = 1e-6,
    ) -> list[TradeAction]:
        """Compute trade actions from the weight diff.

        For each instrument whose weight changes beyond *tolerance* we
        emit a buy or sell action.  Quantity is derived by mapping the
        weight delta back through the total portfolio market value and
        the per-asset market price.
        """
        total_value = sum(float(p.market_value) for p in positions)
        if total_value == 0:
            return []

        trades: list[TradeAction] = []
        for i, pos in enumerate(positions):
            diff = float(target_weights[i]) - float(current_weights[i])
            if abs(diff) < tolerance:
                continue

            notional = abs(diff) * total_value
            price = float(pos.market_price)
            if price == 0:
                continue

            qty = Decimal(str(notional / price)).quantize(
                Decimal("0.000001"), rounding=ROUND_HALF_UP
            )
            cost = Decimal(str(notional)).quantize(
                Decimal("0.01"), rounding=ROUND_HALF_UP
            )

            trades.append(
                TradeAction(
                    instrument_id=pos.instrument_id,
                    side="buy" if diff > 0 else "sell",
                    quantity=qty,
                    estimated_cost=cost,
                )
            )

        return trades
