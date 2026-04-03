"""Market regime detection using rolling statistics."""
from __future__ import annotations

from enum import Enum
from dataclasses import dataclass

import pandas as pd
import numpy as np


class RegimeState(Enum):
    BULL = "bull"
    BEAR = "bear"
    CRISIS = "crisis"


@dataclass(frozen=True)
class RegimeConfig:
    """Thresholds for regime classification."""

    volatility_window: int = 21
    return_window: int = 21
    crisis_vol_threshold: float = 0.30  # annualized vol above this => CRISIS
    bear_return_threshold: float = -0.02  # rolling return below this => BEAR
    risk_multipliers: dict[str, float] | None = None

    @property
    def multipliers(self) -> dict[RegimeState, float]:
        if self.risk_multipliers:
            return {RegimeState(k): v for k, v in self.risk_multipliers.items()}
        return {
            RegimeState.BULL: 1.0,
            RegimeState.BEAR: 1.3,
            RegimeState.CRISIS: 1.8,
        }


class RegimeDetector:
    """Detects market regimes from return series using rolling volatility and returns."""

    def __init__(self, config: RegimeConfig | None = None) -> None:
        self.config = config or RegimeConfig()

    def detect_regime(self, returns: pd.Series) -> RegimeState:
        """Detect the current regime from a return series."""
        if returns.empty or len(returns) < 2:
            return RegimeState.BULL  # default for insufficient data

        history = self.regime_history(returns)
        return RegimeState(history.iloc[-1])

    def regime_history(self, returns: pd.Series) -> pd.Series:
        """Return regime labels for each observation."""
        if returns.empty:
            return pd.Series(dtype=str)

        n = len(returns)
        if n < 2:
            return pd.Series([RegimeState.BULL.value], index=returns.index)

        window = min(self.config.volatility_window, n)
        ret_window = min(self.config.return_window, n)

        # Rolling volatility (annualized)
        rolling_vol = returns.rolling(window=window, min_periods=2).std() * np.sqrt(252)

        # Rolling cumulative return
        rolling_ret = returns.rolling(window=ret_window, min_periods=2).sum()

        regimes = []
        for i in range(n):
            vol = rolling_vol.iloc[i]
            ret = rolling_ret.iloc[i]

            if pd.isna(vol) or pd.isna(ret):
                regimes.append(RegimeState.BULL.value)  # default for warmup
                continue

            if vol > self.config.crisis_vol_threshold:
                regimes.append(RegimeState.CRISIS.value)
            elif ret < self.config.bear_return_threshold:
                regimes.append(RegimeState.BEAR.value)
            else:
                regimes.append(RegimeState.BULL.value)

        return pd.Series(regimes, index=returns.index)

    def risk_multiplier(self, returns: pd.Series) -> float:
        """Return the risk multiplier for the current regime."""
        regime = self.detect_regime(returns)
        return self.config.multipliers[regime]
