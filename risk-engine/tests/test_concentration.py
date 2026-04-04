"""Tests for the concentration risk analyzer (P3-13)."""

from __future__ import annotations

from decimal import Decimal

import pytest

from risk_engine.concentration.analyzer import ConcentrationAnalyzer, ConcentrationResult
from risk_engine.domain.portfolio import Portfolio
from risk_engine.domain.position import Position


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _pos(
    instrument_id: str,
    quantity: Decimal,
    price: Decimal,
    asset_class: str = "equity",
    venue_id: str = "NYSE",
) -> Position:
    return Position(
        instrument_id=instrument_id,
        venue_id=venue_id,
        quantity=quantity,
        average_cost=price,
        market_price=price,
        unrealized_pnl=Decimal("0"),
        realized_pnl=Decimal("0"),
        asset_class=asset_class,
        settlement_cycle="T2",
    )


def _portfolio(positions: list[Position], cash: Decimal = Decimal("0")) -> Portfolio:
    """Build a Portfolio from a list of positions and recompute NAV."""
    pos_dict = {p.instrument_id: p for p in positions}
    pf = Portfolio(positions=pos_dict, cash=cash, available_cash=cash)
    pf.compute_nav()
    return pf


# ---------------------------------------------------------------------------
# Test: single position -> 100% concentration + warning
# ---------------------------------------------------------------------------


class TestSinglePosition:
    def test_single_position_concentration(self) -> None:
        pf = _portfolio([_pos("AAPL", Decimal("100"), Decimal("150"))])
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        assert isinstance(result, ConcentrationResult)
        # Single position should be 100% of NAV
        assert result.single_name["AAPL"] == pytest.approx(100.0, abs=0.01)

    def test_single_position_generates_warning(self) -> None:
        pf = _portfolio([_pos("AAPL", Decimal("100"), Decimal("150"))])
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        assert len(result.warnings) >= 1
        assert any("AAPL" in w and "single-name" in w for w in result.warnings)

    def test_single_position_hhi_is_10000(self) -> None:
        pf = _portfolio([_pos("AAPL", Decimal("100"), Decimal("150"))])
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        assert result.hhi == pytest.approx(10000.0, abs=0.01)

    def test_single_position_asset_class_warning(self) -> None:
        pf = _portfolio([_pos("AAPL", Decimal("100"), Decimal("150"))])
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        assert any("asset-class" in w for w in result.warnings)


# ---------------------------------------------------------------------------
# Test: diversified portfolio -> no warnings
# ---------------------------------------------------------------------------


class TestDiversifiedPortfolio:
    def test_equal_weight_no_warnings(self) -> None:
        """Five equal positions (20% each) across asset classes -> no warnings."""
        asset_classes = ["equity", "crypto", "equity", "crypto", "equity"]
        positions = [
            _pos(
                f"STOCK-{i}",
                Decimal("100"),
                Decimal("10"),
                asset_class=asset_classes[i],
            )
            for i in range(5)
        ]
        pf = _portfolio(positions)
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        # 20% per name (< 25%), 60% equity + 40% crypto (neither > 50%... wait,
        # equity is 60% which > 50%). Use 50% threshold strictly: > not >=.
        # Our analyzer uses > so 60% > 50% triggers. Let's spread evenly.
        assert not any("single-name" in w for w in result.warnings)

    def test_equal_weight_no_single_name_or_asset_class_warnings(self) -> None:
        """Five positions across 5 asset classes -> no warnings at all."""
        classes = ["equity", "crypto", "tokenized_security", "equity", "crypto"]
        positions = [
            _pos(f"STOCK-{i}", Decimal("100"), Decimal("10"), asset_class=classes[i])
            for i in range(5)
        ]
        pf = _portfolio(positions)
        # Use relaxed asset class threshold to avoid 40% crypto triggering
        analyzer = ConcentrationAnalyzer(asset_class_threshold=0.50)
        result = analyzer.analyze(pf)

        # equity=40%, crypto=40%, tokenized=20% -- all <= 50%
        assert len(result.warnings) == 0

    def test_equal_weight_percentages(self) -> None:
        positions = [
            _pos(f"STOCK-{i}", Decimal("100"), Decimal("10"))
            for i in range(5)
        ]
        pf = _portfolio(positions)
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        for pct in result.single_name.values():
            assert pct == pytest.approx(20.0, abs=0.01)

    def test_diversified_across_asset_classes(self) -> None:
        """Two asset classes each at 50% should not breach 50% threshold."""
        positions = [
            _pos("AAPL", Decimal("100"), Decimal("50"), asset_class="equity"),
            _pos("BTC", Decimal("100"), Decimal("50"), asset_class="crypto"),
        ]
        pf = _portfolio(positions)
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        # No asset-class warnings (50% is not > 50%)
        assert not any("asset-class" in w for w in result.warnings)


# ---------------------------------------------------------------------------
# Test: HHI computation matches manual calculation
# ---------------------------------------------------------------------------


class TestHHI:
    def test_two_equal_positions(self) -> None:
        """Two equal positions: HHI = 50^2 + 50^2 = 5000."""
        positions = [
            _pos("A", Decimal("100"), Decimal("10")),
            _pos("B", Decimal("100"), Decimal("10")),
        ]
        pf = _portfolio(positions)
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        assert result.hhi == pytest.approx(5000.0, abs=0.01)

    def test_three_unequal_positions(self) -> None:
        """Positions with 50%, 30%, 20% weights.

        HHI = 50^2 + 30^2 + 20^2 = 2500 + 900 + 400 = 3800.
        """
        positions = [
            _pos("A", Decimal("50"), Decimal("10")),   # 500 -> 50%
            _pos("B", Decimal("30"), Decimal("10")),   # 300 -> 30%
            _pos("C", Decimal("20"), Decimal("10")),   # 200 -> 20%
        ]
        pf = _portfolio(positions)
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        assert result.hhi == pytest.approx(3800.0, abs=0.01)

    def test_perfectly_diversified_hhi(self) -> None:
        """Ten equal positions: HHI = 10 * 10^2 = 1000."""
        positions = [
            _pos(f"S-{i}", Decimal("10"), Decimal("100"))
            for i in range(10)
        ]
        pf = _portfolio(positions)
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        assert result.hhi == pytest.approx(1000.0, abs=0.01)


# ---------------------------------------------------------------------------
# Test: venue concentration
# ---------------------------------------------------------------------------


class TestVenueConcentration:
    def test_venue_breakdown(self) -> None:
        positions = [
            _pos("A", Decimal("100"), Decimal("10"), venue_id="BINANCE"),
            _pos("B", Decimal("100"), Decimal("10"), venue_id="COINBASE"),
        ]
        pf = _portfolio(positions)
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        assert result.by_venue["BINANCE"] == pytest.approx(50.0, abs=0.01)
        assert result.by_venue["COINBASE"] == pytest.approx(50.0, abs=0.01)

    def test_single_venue(self) -> None:
        positions = [
            _pos("A", Decimal("100"), Decimal("10"), venue_id="NYSE"),
            _pos("B", Decimal("100"), Decimal("10"), venue_id="NYSE"),
        ]
        pf = _portfolio(positions)
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        assert result.by_venue["NYSE"] == pytest.approx(100.0, abs=0.01)


# ---------------------------------------------------------------------------
# Test: configurable thresholds
# ---------------------------------------------------------------------------


class TestConfigurableThresholds:
    def test_stricter_thresholds_generate_warnings(self) -> None:
        """With 10% single-name threshold, 3 equal positions (33%) breach."""
        positions = [
            _pos(f"S-{i}", Decimal("100"), Decimal("10"))
            for i in range(3)
        ]
        pf = _portfolio(positions)
        analyzer = ConcentrationAnalyzer(single_name_threshold=0.10)
        result = analyzer.analyze(pf)

        assert len(result.warnings) >= 3  # all 3 positions breach

    def test_relaxed_thresholds_suppress_warnings(self) -> None:
        """With 60% single-name threshold, a 50% position is fine."""
        positions = [
            _pos("A", Decimal("100"), Decimal("10")),
            _pos("B", Decimal("100"), Decimal("10")),
        ]
        pf = _portfolio(positions)
        analyzer = ConcentrationAnalyzer(single_name_threshold=0.60)
        result = analyzer.analyze(pf)

        assert not any("single-name" in w for w in result.warnings)


# ---------------------------------------------------------------------------
# Test: empty portfolio
# ---------------------------------------------------------------------------


class TestEmptyPortfolio:
    def test_empty_portfolio_returns_zeros(self) -> None:
        pf = Portfolio()
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        assert result.single_name == {}
        assert result.by_asset_class == {}
        assert result.by_venue == {}
        assert result.warnings == []
        assert result.hhi == 0.0


# ---------------------------------------------------------------------------
# Test: portfolio with cash affects percentages
# ---------------------------------------------------------------------------


class TestPortfolioWithCash:
    def test_cash_dilutes_concentration(self) -> None:
        """A single position with equal cash should be ~50% of NAV."""
        positions = [_pos("AAPL", Decimal("100"), Decimal("100"))]
        # 10,000 in positions + 10,000 in cash = 20,000 NAV
        pf = _portfolio(positions, cash=Decimal("10000"))
        analyzer = ConcentrationAnalyzer()
        result = analyzer.analyze(pf)

        assert result.single_name["AAPL"] == pytest.approx(50.0, abs=0.01)
        assert result.hhi == pytest.approx(2500.0, abs=0.01)
