"""Portfolio Greeks calculator.

Computes portfolio-level and per-instrument Greeks (delta, gamma, vega,
theta, rho) for a collection of positions.

Phase 3 scope: spot instruments (equity, crypto) use delta = market_value / NAV
(beta-adjusted when beta data is available).  Gamma, vega, theta, rho are zero
for non-option instruments.  Full Black-Scholes Greeks for options will be
added in Phase 4+.
"""

from __future__ import annotations

import math
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any

from risk_engine.domain.position import Position

# Spot asset classes that use the simple delta = market_value / NAV formula.
_SPOT_ASSET_CLASSES = frozenset({"equity", "crypto", "tokenized_security"})


@dataclass(frozen=True)
class Greeks:
    """Risk sensitivities for a single instrument or aggregate."""

    delta: float = 0.0
    gamma: float = 0.0
    vega: float = 0.0
    theta: float = 0.0
    rho: float = 0.0

    def __add__(self, other: Greeks) -> Greeks:
        if not isinstance(other, Greeks):
            return NotImplemented
        return Greeks(
            delta=self.delta + other.delta,
            gamma=self.gamma + other.gamma,
            vega=self.vega + other.vega,
            theta=self.theta + other.theta,
            rho=self.rho + other.rho,
        )


@dataclass(frozen=True)
class PortfolioGreeks:
    """Result of a portfolio-wide Greeks computation."""

    total: Greeks
    by_instrument: dict[str, Greeks]
    computed_at: datetime


class GreeksCalculator:
    """Compute Greeks for a portfolio of positions.

    For spot instruments (equity / crypto / tokenized_security):
        - delta = market_value / NAV  (portfolio weight)
        - If ``market_data`` provides a ``beta`` for the instrument, delta
          is further multiplied by that beta.
        - gamma, vega, theta, rho = 0

    For option instruments (Phase 4+):
        - Black-Scholes Greeks using volatility from ``market_data`` and
          the supplied ``risk_free_rate``.
    """

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def compute(
        self,
        positions: list[Position],
        *,
        nav: float,
        market_data: dict[str, Any] | None = None,
        risk_free_rate: float = 0.0,
    ) -> PortfolioGreeks:
        """Compute per-instrument and aggregate portfolio Greeks.

        Parameters
        ----------
        positions:
            List of :class:`Position` objects to evaluate.
        nav:
            Net Asset Value of the portfolio.  Used to normalise delta
            as a portfolio weight.  If zero, all deltas are set to 0.
        market_data:
            Optional mapping of ``instrument_id`` to a dict of market
            information.  Recognised keys per instrument:

            * ``beta`` (float) — equity beta; delta is multiplied by this.
            * ``volatility`` (float) — annualised implied vol (options).
            * ``time_to_expiry`` (float) — years to expiry (options).
            * ``strike`` (float) — option strike price.
            * ``option_type`` (str) — ``"call"`` or ``"put"``.
        risk_free_rate:
            Annualised risk-free interest rate (continuous compounding).
            Used for option Greeks computation.
        """
        if market_data is None:
            market_data = {}

        by_instrument: dict[str, Greeks] = {}

        for pos in positions:
            instrument_id = pos.instrument_id
            info = market_data.get(instrument_id, {})

            if pos.asset_class in _SPOT_ASSET_CLASSES:
                greeks = self._spot_greeks(pos, nav, info)
            elif pos.asset_class == "option":
                greeks = self._option_greeks(pos, nav, info, risk_free_rate)
            else:
                # Unknown asset class — treat as spot
                greeks = self._spot_greeks(pos, nav, info)

            by_instrument[instrument_id] = greeks

        total = Greeks()
        for g in by_instrument.values():
            total = total + g

        return PortfolioGreeks(
            total=total,
            by_instrument=by_instrument,
            computed_at=datetime.now(timezone.utc),
        )

    # ------------------------------------------------------------------
    # Spot Greeks
    # ------------------------------------------------------------------

    @staticmethod
    def _spot_greeks(
        pos: Position,
        nav: float,
        info: dict[str, Any],
    ) -> Greeks:
        """Delta = market_value / NAV, optionally beta-adjusted."""
        if nav == 0.0:
            return Greeks()

        market_value = float(pos.market_value)
        delta = market_value / nav

        beta = info.get("beta")
        if beta is not None:
            delta *= float(beta)

        return Greeks(delta=delta)

    # ------------------------------------------------------------------
    # Option Greeks (Black-Scholes) — Phase 4+
    # ------------------------------------------------------------------

    @staticmethod
    def _option_greeks(
        pos: Position,
        nav: float,
        info: dict[str, Any],
        risk_free_rate: float,
    ) -> Greeks:
        """Black-Scholes Greeks for a European option position.

        Required keys in *info*: ``volatility``, ``time_to_expiry``,
        ``strike``, ``option_type`` (``"call"`` | ``"put"``).

        Falls back to zero Greeks if any required field is missing.
        """
        try:
            sigma = float(info["volatility"])
            T = float(info["time_to_expiry"])       # noqa: N806 (math convention)
            K = float(info["strike"])                # noqa: N806
            option_type = info["option_type"]        # "call" or "put"
        except (KeyError, TypeError):
            return Greeks()

        S = float(pos.market_price)                  # noqa: N806
        qty = float(pos.quantity)
        r = risk_free_rate

        if T <= 0.0 or sigma <= 0.0 or S <= 0.0 or K <= 0.0:
            return Greeks()

        sqrt_T = math.sqrt(T)
        d1 = (math.log(S / K) + (r + 0.5 * sigma ** 2) * T) / (sigma * sqrt_T)
        d2 = d1 - sigma * sqrt_T

        # Standard normal PDF / CDF helpers
        nd1 = _norm_cdf(d1)
        nd2 = _norm_cdf(d2)
        npd1 = _norm_pdf(d1)

        if option_type == "call":
            per_unit_delta = nd1
        else:
            per_unit_delta = nd1 - 1.0

        per_unit_gamma = npd1 / (S * sigma * sqrt_T)
        per_unit_vega = S * npd1 * sqrt_T / 100.0  # per 1% vol move
        discount = math.exp(-r * T)

        if option_type == "call":
            per_unit_theta = (
                -(S * npd1 * sigma) / (2.0 * sqrt_T)
                - r * K * discount * nd2
            ) / 365.0  # per calendar day
            per_unit_rho = K * T * discount * nd2 / 100.0
        else:
            nd2_neg = _norm_cdf(-d2)
            per_unit_theta = (
                -(S * npd1 * sigma) / (2.0 * sqrt_T)
                + r * K * discount * nd2_neg
            ) / 365.0
            per_unit_rho = -K * T * discount * nd2_neg / 100.0

        return Greeks(
            delta=per_unit_delta * qty,
            gamma=per_unit_gamma * qty,
            vega=per_unit_vega * qty,
            theta=per_unit_theta * qty,
            rho=per_unit_rho * qty,
        )


# ---------------------------------------------------------------------------
# Standard normal helpers (avoid scipy dependency for this small need)
# ---------------------------------------------------------------------------

def _norm_cdf(x: float) -> float:
    """Cumulative distribution function of the standard normal."""
    return 0.5 * (1.0 + math.erf(x / math.sqrt(2.0)))


def _norm_pdf(x: float) -> float:
    """Probability density function of the standard normal."""
    return math.exp(-0.5 * x * x) / math.sqrt(2.0 * math.pi)
