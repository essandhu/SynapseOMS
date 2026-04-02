"""Rolling and exponentially-weighted time-series statistics."""

from __future__ import annotations

import pandas as pd


def rolling_mean(series: pd.Series, window: int) -> pd.Series:
    """Compute a simple rolling mean over *window* periods.

    Parameters
    ----------
    series:
        Input time series.
    window:
        Number of observations in the rolling window.

    Returns
    -------
    pd.Series with the rolling mean; the first ``window - 1`` values will be
    NaN.
    """
    return series.rolling(window=window).mean()


def rolling_std(series: pd.Series, window: int) -> pd.Series:
    """Compute a simple rolling standard deviation over *window* periods.

    Parameters
    ----------
    series:
        Input time series.
    window:
        Number of observations in the rolling window.

    Returns
    -------
    pd.Series with the rolling standard deviation (ddof=1); the first
    ``window - 1`` values will be NaN.
    """
    return series.rolling(window=window).std(ddof=1)


def exponential_weighted_covariance(
    returns: pd.DataFrame,
    span: int = 60,
) -> pd.DataFrame:
    """Compute an exponentially-weighted covariance matrix.

    Uses the pandas ``ewm`` method with the given *span* parameter to weight
    recent observations more heavily.

    Parameters
    ----------
    returns:
        DataFrame of asset returns (columns = instruments, rows = dates).
    span:
        Decay span for the exponential weights.

    Returns
    -------
    pd.DataFrame — the EWM covariance matrix (N x N) where N is the number of
    columns in *returns*.
    """
    return returns.ewm(span=span).cov().iloc[-len(returns.columns):]
