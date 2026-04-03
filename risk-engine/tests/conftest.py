"""Shared pytest fixtures for the risk-engine test suite."""

from __future__ import annotations

import sys
from pathlib import Path

# Add project root (parent of both risk-engine/ and ai/) so that
# ``from ai.execution_analyst.types import ...`` resolves when pytest
# runs from the risk-engine/ directory.
sys.path.insert(0, str(Path(__file__).resolve().parent.parent.parent))

from decimal import Decimal

import numpy as np
import pandas as pd
import pytest

from risk_engine.domain.position import Position
from risk_engine.domain.portfolio import Portfolio


# ---------------------------------------------------------------------------
# Sample positions
# ---------------------------------------------------------------------------

@pytest.fixture()
def sample_positions() -> dict[str, Position]:
    """Three representative positions spanning equity and crypto."""
    return {
        "AAPL": Position(
            instrument_id="AAPL",
            venue_id="NYSE",
            quantity=Decimal("100"),
            average_cost=Decimal("175.00"),
            market_price=Decimal("180.00"),
            unrealized_pnl=Decimal("500.00"),
            realized_pnl=Decimal("0"),
            asset_class="equity",
            settlement_cycle="T2",
        ),
        "BTC-USD": Position(
            instrument_id="BTC-USD",
            venue_id="COINBASE",
            quantity=Decimal("0.5"),
            average_cost=Decimal("62000.00"),
            market_price=Decimal("65000.00"),
            unrealized_pnl=Decimal("1500.00"),
            realized_pnl=Decimal("0"),
            asset_class="crypto",
            settlement_cycle="T0",
        ),
        "ETH-USD": Position(
            instrument_id="ETH-USD",
            venue_id="COINBASE",
            quantity=Decimal("5"),
            average_cost=Decimal("3200.00"),
            market_price=Decimal("3400.00"),
            unrealized_pnl=Decimal("1000.00"),
            realized_pnl=Decimal("0"),
            asset_class="crypto",
            settlement_cycle="T0",
        ),
    }


# ---------------------------------------------------------------------------
# Sample returns matrix
# ---------------------------------------------------------------------------

@pytest.fixture()
def sample_returns_matrix() -> pd.DataFrame:
    """252 trading days of synthetic daily returns for three instruments.

    Returns are drawn from realistic distributions:
    - AAPL:    mean ~0.05% daily, vol ~1.5%
    - BTC-USD: mean ~0.10% daily, vol ~4.0%
    - ETH-USD: mean ~0.12% daily, vol ~5.0%

    A fixed random seed ensures deterministic tests.
    """
    rng = np.random.default_rng(seed=42)
    n_days = 252

    # Correlated returns via Cholesky decomposition
    correlation = np.array([
        [1.0,  0.3,  0.35],
        [0.3,  1.0,  0.85],
        [0.35, 0.85, 1.0],
    ])
    vols = np.array([0.015, 0.04, 0.05])
    means = np.array([0.0005, 0.001, 0.0012])

    cov = np.outer(vols, vols) * correlation
    cholesky = np.linalg.cholesky(cov)

    uncorrelated = rng.standard_normal((n_days, 3))
    correlated = uncorrelated @ cholesky.T + means

    return pd.DataFrame(
        correlated,
        columns=["AAPL", "BTC-USD", "ETH-USD"],
    )


# ---------------------------------------------------------------------------
# Sample covariance matrix
# ---------------------------------------------------------------------------

@pytest.fixture()
def sample_covariance_matrix() -> np.ndarray:
    """3x3 annualised covariance matrix for AAPL, BTC-USD, ETH-USD."""
    correlation = np.array([
        [1.0,  0.3,  0.35],
        [0.3,  1.0,  0.85],
        [0.35, 0.85, 1.0],
    ])
    annual_vols = np.array([0.24, 0.64, 0.80])  # ~1.5%, 4%, 5% daily * sqrt(252)
    return np.outer(annual_vols, annual_vols) * correlation


# ---------------------------------------------------------------------------
# Sample portfolio
# ---------------------------------------------------------------------------

@pytest.fixture()
def sample_portfolio(sample_positions: dict[str, Position]) -> Portfolio:
    """Portfolio seeded with the three sample positions and $100K starting cash."""
    portfolio = Portfolio(
        positions=dict(sample_positions),
        cash=Decimal("100000"),
        available_cash=Decimal("100000"),
    )
    portfolio.compute_nav()
    return portfolio
