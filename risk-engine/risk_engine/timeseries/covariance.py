"""Covariance matrix estimators for portfolio risk analysis."""

from __future__ import annotations

import numpy as np
import pandas as pd
from sklearn.covariance import LedoitWolf


def sample_covariance(returns: pd.DataFrame) -> np.ndarray:
    """Compute the standard sample covariance matrix.

    Parameters
    ----------
    returns:
        DataFrame of asset returns (columns = instruments, rows = dates).

    Returns
    -------
    np.ndarray — (N x N) sample covariance matrix.
    """
    return np.array(returns.cov())


def ledoit_wolf_shrinkage(returns: pd.DataFrame) -> np.ndarray:
    """Compute the Ledoit-Wolf shrinkage covariance estimator.

    Uses ``sklearn.covariance.LedoitWolf`` to produce a regularised
    covariance matrix that is better conditioned than the raw sample
    covariance, especially when the number of observations is not much
    larger than the number of assets.

    Parameters
    ----------
    returns:
        DataFrame of asset returns (columns = instruments, rows = dates).

    Returns
    -------
    np.ndarray — (N x N) shrinkage-estimated covariance matrix.
    """
    lw = LedoitWolf()
    lw.fit(returns.dropna())
    return lw.covariance_
