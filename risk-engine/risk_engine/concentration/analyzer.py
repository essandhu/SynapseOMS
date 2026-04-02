"""Concentration risk analyzer.

Computes single-name, asset-class, and venue concentration metrics
for a portfolio, flags threshold breaches, and calculates the
Herfindahl-Hirschman Index (HHI).
"""

from __future__ import annotations

from dataclasses import dataclass, field
from decimal import Decimal

from risk_engine.domain.portfolio import Portfolio


@dataclass(frozen=True)
class ConcentrationResult:
    """Output of a concentration risk analysis."""

    single_name: dict[str, float]  # instrument_id -> % of NAV (0-100)
    by_asset_class: dict[str, float]  # asset_class -> % of NAV (0-100)
    by_venue: dict[str, float]  # venue_id -> % of NAV (0-100)
    warnings: list[str]  # threshold breach descriptions
    hhi: float  # Herfindahl-Hirschman Index (0-10000)


class ConcentrationAnalyzer:
    """Analyzes portfolio concentration and flags threshold breaches.

    Parameters
    ----------
    single_name_threshold:
        Maximum allowed weight for any single position, as a fraction
        (e.g. 0.25 = 25 %).
    asset_class_threshold:
        Maximum allowed weight for any single asset class, as a fraction
        (e.g. 0.50 = 50 %).
    """

    def __init__(
        self,
        single_name_threshold: float = 0.25,
        asset_class_threshold: float = 0.50,
    ) -> None:
        self.single_name_threshold = single_name_threshold
        self.asset_class_threshold = asset_class_threshold

    def analyze(self, portfolio: Portfolio) -> ConcentrationResult:
        """Run concentration analysis on *portfolio*.

        Returns a :class:`ConcentrationResult` with per-instrument,
        per-asset-class, and per-venue concentration percentages,
        a list of threshold-breach warnings, and the HHI.
        """
        nav = portfolio.nav
        if nav == Decimal("0"):
            return ConcentrationResult(
                single_name={},
                by_asset_class={},
                by_venue={},
                warnings=[],
                hhi=0.0,
            )

        # -- Single-name concentration --
        single_name: dict[str, float] = {}
        for inst_id, pos in portfolio.positions.items():
            pct = float(pos.market_value / nav * 100)
            single_name[inst_id] = round(pct, 4)

        # -- Asset-class concentration --
        ac_totals: dict[str, Decimal] = {}
        for pos in portfolio.positions.values():
            ac_totals[pos.asset_class] = ac_totals.get(
                pos.asset_class, Decimal("0")
            ) + pos.market_value
        by_asset_class: dict[str, float] = {
            ac: round(float(val / nav * 100), 4)
            for ac, val in ac_totals.items()
        }

        # -- Venue concentration --
        venue_totals: dict[str, Decimal] = {}
        for pos in portfolio.positions.values():
            venue_totals[pos.venue_id] = venue_totals.get(
                pos.venue_id, Decimal("0")
            ) + pos.market_value
        by_venue: dict[str, float] = {
            v: round(float(val / nav * 100), 4)
            for v, val in venue_totals.items()
        }

        # -- Warnings --
        warnings: list[str] = []
        sn_thresh_pct = self.single_name_threshold * 100
        ac_thresh_pct = self.asset_class_threshold * 100
        for inst_id, pct in single_name.items():
            if pct > sn_thresh_pct:
                warnings.append(
                    f"{inst_id} exceeds single-name threshold: "
                    f"{pct}% > {sn_thresh_pct}%"
                )
        for ac, pct in by_asset_class.items():
            if pct > ac_thresh_pct:
                warnings.append(
                    f"{ac} exceeds asset-class threshold: "
                    f"{pct}% > {ac_thresh_pct}%"
                )

        # -- HHI: sum of squared weight percentages --
        hhi = sum(w ** 2 for w in single_name.values())

        return ConcentrationResult(
            single_name=single_name,
            by_asset_class=by_asset_class,
            by_venue=by_venue,
            warnings=warnings,
            hhi=round(hhi, 4),
        )
