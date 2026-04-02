"""Historical Value-at-Risk (VaR) computation for cross-asset, mixed-calendar portfolios."""

from __future__ import annotations

from decimal import Decimal

import numpy as np
import pandas as pd

from risk_engine.domain.position import Position
from risk_engine.domain.risk_result import VaRResult


class HistoricalVaR:
    """Compute Historical VaR across equity and crypto positions with
    mixed-calendar alignment.

    Crypto trades 24/7 while equities only trade on business days.
    This class handles the calendar mismatch by forward-filling equity
    returns on non-trading days before computing portfolio-level returns.
    """

    def __init__(self, window_days: int = 252, confidence: float = 0.99) -> None:
        self.window_days = window_days
        self.confidence = confidence

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def compute(
        self,
        positions: dict[str, Position],
        returns_matrix: pd.DataFrame,
        base_currency: str = "USD",
    ) -> VaRResult:
        """Compute Historical VaR and CVaR for the given portfolio.

        Steps
        -----
        1. Align returns to common dates (forward-fill equity on
           weekends / holidays to match crypto's 24/7 calendar).
        2. Compute portfolio weights from position market values.
        3. Compute portfolio returns: sum(weight_i * return_i) per date.
        4. Sort portfolio returns.
        5. VaR  = -percentile(portfolio_returns, 1 - confidence)
        6. CVaR = -mean(portfolio_returns below VaR threshold)
        """
        # Handle empty portfolio
        if not positions:
            return VaRResult(
                var_amount=Decimal("0"),
                cvar_amount=Decimal("0"),
                confidence=self.confidence,
                horizon_days=1,
                method="historical",
            )

        # 1. Align returns ------------------------------------------------
        aligned = self._align_returns(positions, returns_matrix)

        # 2. Portfolio weights ---------------------------------------------
        instrument_ids = [iid for iid in positions if iid in aligned.columns]
        total_value = sum(
            float(positions[iid].market_value) for iid in instrument_ids
        )
        weights = np.array(
            [float(positions[iid].market_value) / total_value for iid in instrument_ids]
        )

        # 3. Portfolio returns ---------------------------------------------
        port_returns: np.ndarray = aligned[instrument_ids].values @ weights

        # 4 & 5. VaR ------------------------------------------------------
        alpha = (1.0 - self.confidence) * 100.0  # e.g. 1.0 for 99% confidence
        var_pct = -float(np.percentile(port_returns, alpha))

        # 6. CVaR ----------------------------------------------------------
        threshold = -var_pct  # losses beyond this point
        tail = port_returns[port_returns <= threshold]
        if len(tail) > 0:
            cvar_pct = -float(np.mean(tail))
        else:
            cvar_pct = var_pct

        var_amount = Decimal(str(var_pct * total_value))
        cvar_amount = Decimal(str(cvar_pct * total_value))

        return VaRResult(
            var_amount=var_amount,
            cvar_amount=cvar_amount,
            confidence=self.confidence,
            horizon_days=1,
            method="historical",
            distribution=[float(r) for r in port_returns],
        )

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _align_returns(
        self,
        positions: dict[str, Position],
        returns_matrix: pd.DataFrame,
    ) -> pd.DataFrame:
        """Align a mixed-calendar returns matrix.

        1. Start with the union of all dates in the returns matrix.
        2. Forward-fill equity columns on weekends / holidays.
        3. Crypto columns are expected to have values every day.
        4. Trim to the most recent ``window_days`` rows.
        5. Drop any remaining leading NaN rows.
        """
        df = returns_matrix.copy()

        # Identify equity vs crypto columns based on position metadata
        equity_cols = [
            iid for iid, pos in positions.items()
            if pos.asset_class == "equity" and iid in df.columns
        ]

        # Forward-fill equity columns (NaN on weekends → 0 return)
        if equity_cols:
            df[equity_cols] = df[equity_cols].ffill().fillna(0.0)

        # Trim to window
        df = df.iloc[-self.window_days:]

        # Drop leading rows that still have NaN (e.g. before first data)
        df = df.dropna()

        return df
