"""Parametric Value-at-Risk (variance-covariance method).

Assumes normally distributed returns to compute VaR analytically via
the portfolio's covariance matrix. Computationally fast (<1ms) — suitable
for real-time pre-trade risk checks — but less accurate for fat-tailed
distributions (e.g., crypto).
"""

from __future__ import annotations

from decimal import Decimal
from math import sqrt

import time as _time

import numpy as np
import pandas as pd
from scipy.stats import norm

from risk_engine.domain.position import Position
from risk_engine.domain.risk_result import VaRResult
from risk_engine.metrics import var_computation_seconds
from risk_engine.timeseries.covariance import ledoit_wolf_shrinkage, sample_covariance


class ParametricVaR:
    """Parametric VaR using the variance-covariance method.

    Parameters
    ----------
    confidence:
        Confidence level (e.g. 0.99 for 99% VaR).
    use_shrinkage:
        If ``True``, use Ledoit-Wolf shrinkage for the covariance
        estimator; otherwise use the raw sample covariance.
    """

    def __init__(self, confidence: float = 0.99, use_shrinkage: bool = True) -> None:
        self.confidence = confidence
        self.use_shrinkage = use_shrinkage

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def compute(
        self,
        positions: dict[str, Position],
        returns_matrix: pd.DataFrame,
        base_currency: str = "USD",
    ) -> VaRResult:
        """Compute Parametric VaR and CVaR for the given portfolio.

        Steps
        -----
        1. Filter ``returns_matrix`` to only instruments in *positions*.
        2. Compute covariance matrix (Ledoit-Wolf shrinkage or sample).
        3. Compute position weights.
        4. Portfolio variance = w' * Sigma * w.
        5. Portfolio std = sqrt(variance).
        6. VaR  = z_{alpha} * portfolio_std * portfolio_value.
        7. CVaR = portfolio_value * portfolio_std * phi(z_{alpha}) / (1 - alpha).
        """
        _start = _time.monotonic()
        try:
            return self._compute_inner(positions, returns_matrix, base_currency)
        finally:
            var_computation_seconds.labels(method="parametric").observe(
                _time.monotonic() - _start
            )

    def _compute_inner(
        self,
        positions: dict[str, Position],
        returns_matrix: pd.DataFrame,
        base_currency: str = "USD",
    ) -> VaRResult:
        # Handle empty portfolio
        if not positions:
            return VaRResult(
                var_amount=Decimal("0"),
                cvar_amount=Decimal("0"),
                confidence=self.confidence,
                horizon_days=1,
                method="parametric",
            )

        # 1. Filter returns to instruments present in both positions and data
        instrument_ids = [
            iid for iid in positions if iid in returns_matrix.columns
        ]
        if not instrument_ids:
            return VaRResult(
                var_amount=Decimal("0"),
                cvar_amount=Decimal("0"),
                confidence=self.confidence,
                horizon_days=1,
                method="parametric",
            )

        filtered_returns = returns_matrix[instrument_ids].dropna()

        # 2. Covariance matrix
        if self.use_shrinkage:
            cov_matrix = ledoit_wolf_shrinkage(filtered_returns)
        else:
            cov_matrix = sample_covariance(filtered_returns)

        # 3. Position weights
        market_values = np.array(
            [float(positions[iid].market_value) for iid in instrument_ids]
        )
        total_value = float(market_values.sum())
        if total_value == 0.0:
            return VaRResult(
                var_amount=Decimal("0"),
                cvar_amount=Decimal("0"),
                confidence=self.confidence,
                horizon_days=1,
                method="parametric",
            )

        weights = market_values / total_value

        # 4. Portfolio variance: w' * Sigma * w
        portfolio_variance = float(weights @ cov_matrix @ weights)

        # 5. Portfolio standard deviation
        portfolio_std = sqrt(max(portfolio_variance, 0.0))

        # 6. VaR = z_alpha * sigma_p * V
        z_alpha = norm.ppf(self.confidence)
        var_amount = z_alpha * portfolio_std * total_value

        # 7. CVaR = V * sigma_p * phi(z_alpha) / (1 - alpha)
        phi_z = norm.pdf(z_alpha)
        cvar_amount = total_value * portfolio_std * phi_z / (1.0 - self.confidence)

        return VaRResult(
            var_amount=Decimal(str(round(var_amount, 10))),
            cvar_amount=Decimal(str(round(cvar_amount, 10))),
            confidence=self.confidence,
            horizon_days=1,
            method="parametric",
        )
