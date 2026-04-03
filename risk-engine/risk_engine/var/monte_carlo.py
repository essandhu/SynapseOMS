"""Monte Carlo Value-at-Risk with correlated paths and fat-tailed distributions.

Generates correlated random returns via Cholesky decomposition of the
covariance matrix.  Per-instrument distributions can be either normal
(for equities) or Student-t (for crypto assets with fat tails).  Returns
the full simulated P&L distribution array for frontend histogram rendering.
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal
from math import sqrt

import time as _time

import numpy as np
from scipy.stats import t as student_t

from risk_engine.domain.position import Position
from risk_engine.domain.risk_result import VaRResult
from risk_engine.metrics import var_computation_seconds


# ---------------------------------------------------------------------------
# Distribution configuration
# ---------------------------------------------------------------------------


@dataclass
class DistributionParams:
    """Per-instrument distribution specification.

    Parameters
    ----------
    family:
        ``"normal"`` for Gaussian returns (equities) or ``"student_t"``
        for fat-tailed returns (crypto).
    df:
        Degrees of freedom for the Student-t distribution.  Required when
        ``family == "student_t"``.  Typical crypto values: 3-5.
    """

    family: str  # "normal" or "student_t"
    df: float | None = None  # degrees of freedom for Student-t


# ---------------------------------------------------------------------------
# Monte Carlo VaR engine
# ---------------------------------------------------------------------------


class MonteCarloVaR:
    """Monte Carlo VaR with correlated paths and configurable per-instrument
    distributions.

    Parameters
    ----------
    num_simulations:
        Number of Monte Carlo simulation paths (default 10,000).
    horizon_days:
        Risk horizon in trading days (default 1).
    confidence:
        Confidence level for VaR (default 0.99 = 99%).
    """

    def __init__(
        self,
        num_simulations: int = 10_000,
        horizon_days: int = 1,
        confidence: float = 0.99,
    ) -> None:
        self.num_simulations = num_simulations
        self.horizon_days = horizon_days
        self.confidence = confidence

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def compute(
        self,
        positions: list[Position],
        covariance_matrix: np.ndarray,
        expected_returns: np.ndarray,
        distribution_params: list[DistributionParams],
    ) -> VaRResult:
        """Compute Monte Carlo VaR and CVaR.

        Parameters
        ----------
        positions:
            List of ``Position`` objects.  Order must align with rows/columns
            of *covariance_matrix*, *expected_returns*, and *distribution_params*.
        covariance_matrix:
            ``(n, n)`` daily covariance matrix of instrument returns.
        expected_returns:
            ``(n,)`` vector of expected daily returns per instrument.
        distribution_params:
            One ``DistributionParams`` per instrument specifying the marginal
            distribution family (normal or Student-t).

        Returns
        -------
        VaRResult
            With ``var_amount``, ``cvar_amount``, and the full ``distribution``
            array of simulated portfolio P&L values.
        """
        _start = _time.monotonic()
        try:
            return self._compute_inner(
                positions, covariance_matrix, expected_returns, distribution_params
            )
        finally:
            var_computation_seconds.labels(method="monte_carlo").observe(
                _time.monotonic() - _start
            )

    def _compute_inner(
        self,
        positions: list[Position],
        covariance_matrix: np.ndarray,
        expected_returns: np.ndarray,
        distribution_params: list[DistributionParams],
    ) -> VaRResult:
        n_instruments = len(positions)
        n_sims = self.num_simulations

        # Market values per instrument
        market_values = np.array(
            [float(p.market_value) for p in positions], dtype=np.float64
        )

        # 1. Cholesky decomposition of covariance matrix --------------------
        chol = np.linalg.cholesky(covariance_matrix)

        # 2. Generate per-instrument random draws ---------------------------
        #    Shape: (n_sims, n_instruments)
        raw = np.empty((n_sims, n_instruments), dtype=np.float64)
        for i, dp in enumerate(distribution_params):
            if dp.family == "student_t":
                if dp.df is None:
                    raise ValueError(
                        f"Student-t distribution requires 'df' parameter "
                        f"(instrument index {i})"
                    )
                # Student-t draws scaled to unit variance:
                # Var(t_df) = df / (df - 2), so divide by sqrt(df / (df - 2))
                # to get unit-variance draws.
                raw_t = student_t.rvs(df=dp.df, size=n_sims)
                scale = sqrt(dp.df / (dp.df - 2)) if dp.df > 2 else 1.0
                raw[:, i] = raw_t / scale
            else:
                # Normal (standard Gaussian)
                raw[:, i] = np.random.standard_normal(n_sims)

        # 3. Induce correlation via Cholesky factor -------------------------
        #    correlated = raw @ L^T   (each row is a correlated sample)
        correlated_returns = raw @ chol.T

        # 4. Add expected returns and scale for horizon ---------------------
        #    For multi-day horizon, scale mean by T and vol by sqrt(T).
        horizon_factor = sqrt(self.horizon_days)
        mean_factor = self.horizon_days

        # Simulated returns: mean * T + correlated_vol * sqrt(T)
        simulated_returns = (
            expected_returns * mean_factor + correlated_returns * horizon_factor
        )

        # 5. Compute portfolio P&L for each simulation ---------------------
        #    P&L_i = sum_j(market_value_j * simulated_return_j_i)
        simulated_pnl = simulated_returns @ market_values

        # 6. VaR = -percentile(simulated_pnl, 1 - confidence) ---------------
        alpha_pct = (1.0 - self.confidence) * 100.0
        var_amount = -float(np.percentile(simulated_pnl, alpha_pct))

        # 7. CVaR = -mean(simulated_pnl below -VaR threshold) ---------------
        threshold = -var_amount
        tail = simulated_pnl[simulated_pnl <= threshold]
        if len(tail) > 0:
            cvar_amount = -float(np.mean(tail))
        else:
            cvar_amount = var_amount

        return VaRResult(
            var_amount=Decimal(str(round(var_amount, 10))),
            cvar_amount=Decimal(str(round(cvar_amount, 10))),
            confidence=self.confidence,
            horizon_days=self.horizon_days,
            method="monte_carlo",
            distribution=[float(x) for x in simulated_pnl],
        )
