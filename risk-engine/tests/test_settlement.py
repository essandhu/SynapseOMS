"""Tests for Settlement Tracker — T+0 vs T+2 settlement risk."""

from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest

from risk_engine.settlement.tracker import PendingSettlement, SettlementTracker


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_tracker() -> SettlementTracker:
    return SettlementTracker()


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestT0Settlement:
    """Crypto fills settle immediately and create no pending record."""

    def test_t0_settlement_immediate(self):
        tracker = _make_tracker()
        tracker.record_fill(
            instrument_id="BTC-USD",
            asset_class="crypto",
            side="BUY",
            quantity=Decimal("1"),
            price=Decimal("65000"),
            trade_date=date(2026, 3, 30),
        )
        # No pending record should be created for T0 assets
        assert len(tracker._pending) == 0

    def test_tokenized_security_t0(self):
        tracker = _make_tracker()
        tracker.record_fill(
            instrument_id="TOKEN-A",
            asset_class="tokenized_security",
            side="BUY",
            quantity=Decimal("100"),
            price=Decimal("50"),
            trade_date=date(2026, 3, 30),
        )
        assert len(tracker._pending) == 0


class TestT2Settlement:
    """Equity fills create pending records with correct settlement dates."""

    def test_t2_settlement_creates_pending(self):
        tracker = _make_tracker()
        # Monday March 30 2026 -> T+2 = Wednesday April 1 2026
        tracker.record_fill(
            instrument_id="AAPL",
            asset_class="equity",
            side="BUY",
            quantity=Decimal("100"),
            price=Decimal("175"),
            trade_date=date(2026, 3, 30),  # Monday
        )
        assert len(tracker._pending) == 1
        pending = tracker._pending[0]
        assert pending.instrument_id == "AAPL"
        assert pending.trade_date == date(2026, 3, 30)
        assert pending.settlement_date == date(2026, 4, 1)  # Wednesday
        assert pending.amount == Decimal("-17500")  # Buy = negative cash
        assert pending.status == "pending"

    def test_weekend_skipping(self):
        """T+2 settlement on a Friday should skip the weekend and settle Tuesday."""
        tracker = _make_tracker()
        # Friday April 3 2026 -> T+2 skips Sat/Sun -> Tuesday April 7 2026
        tracker.record_fill(
            instrument_id="MSFT",
            asset_class="equity",
            side="BUY",
            quantity=Decimal("50"),
            price=Decimal("400"),
            trade_date=date(2026, 4, 3),  # Friday
        )
        assert len(tracker._pending) == 1
        pending = tracker._pending[0]
        assert pending.settlement_date == date(2026, 4, 7)  # Tuesday

    def test_thursday_t2_settlement(self):
        """T+2 on Thursday -> settlement on Monday (skips weekend)."""
        tracker = _make_tracker()
        tracker.record_fill(
            instrument_id="GOOG",
            asset_class="equity",
            side="SELL",
            quantity=Decimal("10"),
            price=Decimal("150"),
            trade_date=date(2026, 4, 2),  # Thursday
        )
        pending = tracker._pending[0]
        assert pending.settlement_date == date(2026, 4, 6)  # Monday


class TestSettleMatured:
    """settle_matured marks settlements as complete when the date arrives."""

    def test_settle_matured_marks_complete(self):
        tracker = _make_tracker()
        tracker.record_fill(
            instrument_id="AAPL",
            asset_class="equity",
            side="BUY",
            quantity=Decimal("100"),
            price=Decimal("175"),
            trade_date=date(2026, 3, 30),  # Monday -> settles Wed Apr 1
        )
        assert tracker._pending[0].status == "pending"

        # Before settlement date: still pending
        tracker.settle_matured(as_of=date(2026, 3, 31))
        assert tracker._pending[0].status == "pending"

        # On settlement date: marked settled
        tracker.settle_matured(as_of=date(2026, 4, 1))
        assert tracker._pending[0].status == "settled"

    def test_settle_matured_after_date(self):
        tracker = _make_tracker()
        tracker.record_fill(
            instrument_id="AAPL",
            asset_class="equity",
            side="SELL",
            quantity=Decimal("50"),
            price=Decimal("180"),
            trade_date=date(2026, 3, 30),
        )
        # Well past settlement date
        tracker.settle_matured(as_of=date(2026, 4, 10))
        assert tracker._pending[0].status == "settled"


class TestComputeSettlementRisk:
    """Verify total_unsettled, by_date breakdown, and cash_committed."""

    def test_compute_settlement_risk(self):
        tracker = _make_tracker()
        # Buy 100 AAPL @ 175 on Monday Mar 30 -> settles Wed Apr 1
        tracker.record_fill(
            instrument_id="AAPL",
            asset_class="equity",
            side="BUY",
            quantity=Decimal("100"),
            price=Decimal("175"),
            trade_date=date(2026, 3, 30),
        )
        # Sell 50 MSFT @ 400 on Monday Mar 30 -> settles Wed Apr 1
        tracker.record_fill(
            instrument_id="MSFT",
            asset_class="equity",
            side="SELL",
            quantity=Decimal("50"),
            price=Decimal("400"),
            trade_date=date(2026, 3, 30),
        )

        risk = tracker.compute_settlement_risk()

        # total_unsettled = abs(-17500) + abs(20000) = 37500
        assert risk["total_unsettled"] == Decimal("37500")

        # by_date: both settle Apr 1: -17500 + 20000 = 2500
        assert risk["by_date"] == {"2026-04-01": "2500"}

        # cash_committed = abs(-17500) = 17500 (only buy side)
        assert risk["cash_committed"] == Decimal("17500")

        assert risk["pending_count"] == 2

    def test_settled_items_excluded_from_risk(self):
        tracker = _make_tracker()
        tracker.record_fill(
            instrument_id="AAPL",
            asset_class="equity",
            side="BUY",
            quantity=Decimal("100"),
            price=Decimal("175"),
            trade_date=date(2026, 3, 30),
        )
        # Settle it
        tracker.settle_matured(as_of=date(2026, 4, 1))

        risk = tracker.compute_settlement_risk()
        assert risk["total_unsettled"] == Decimal("0")
        assert risk["cash_committed"] == Decimal("0")
        assert risk["pending_count"] == 0


class TestCashImpact:
    """Buy orders commit cash (negative), sell orders free cash (positive)."""

    def test_buy_commits_cash(self):
        tracker = _make_tracker()
        tracker.record_fill(
            instrument_id="AAPL",
            asset_class="equity",
            side="BUY",
            quantity=Decimal("100"),
            price=Decimal("175"),
            trade_date=date(2026, 3, 30),
        )
        pending = tracker._pending[0]
        assert pending.amount == Decimal("-17500")  # Negative = cash outgoing

        risk = tracker.compute_settlement_risk()
        assert risk["cash_committed"] == Decimal("17500")

    def test_sell_frees_cash(self):
        tracker = _make_tracker()
        tracker.record_fill(
            instrument_id="AAPL",
            asset_class="equity",
            side="SELL",
            quantity=Decimal("50"),
            price=Decimal("180"),
            trade_date=date(2026, 3, 30),
        )
        pending = tracker._pending[0]
        assert pending.amount == Decimal("9000")  # Positive = cash incoming

        risk = tracker.compute_settlement_risk()
        # No buy-side commitment
        assert risk["cash_committed"] == Decimal("0")


class TestMultipleSettlements:
    """Multiple fills are tracked independently."""

    def test_multiple_settlements(self):
        tracker = _make_tracker()

        # Fill 1: Buy AAPL on Monday Mar 30
        tracker.record_fill(
            instrument_id="AAPL",
            asset_class="equity",
            side="BUY",
            quantity=Decimal("100"),
            price=Decimal("175"),
            trade_date=date(2026, 3, 30),  # Monday -> Wed Apr 1
        )
        # Fill 2: Buy MSFT on Tuesday Mar 31
        tracker.record_fill(
            instrument_id="MSFT",
            asset_class="equity",
            side="BUY",
            quantity=Decimal("50"),
            price=Decimal("400"),
            trade_date=date(2026, 3, 31),  # Tuesday -> Thu Apr 2
        )
        # Fill 3: Crypto — no pending
        tracker.record_fill(
            instrument_id="BTC-USD",
            asset_class="crypto",
            side="BUY",
            quantity=Decimal("0.5"),
            price=Decimal("65000"),
            trade_date=date(2026, 3, 30),
        )

        assert len(tracker._pending) == 2  # Crypto excluded

        # Each has its own settlement date
        dates = {s.instrument_id: s.settlement_date for s in tracker._pending}
        assert dates["AAPL"] == date(2026, 4, 1)   # Wed
        assert dates["MSFT"] == date(2026, 4, 2)    # Thu

        risk = tracker.compute_settlement_risk()
        # 17500 + 20000 = 37500
        assert risk["total_unsettled"] == Decimal("37500")
        assert risk["pending_count"] == 2
        # Both are buys so cash_committed = 37500
        assert risk["cash_committed"] == Decimal("37500")

        # Settle only the first one (as of Apr 1)
        tracker.settle_matured(as_of=date(2026, 4, 1))

        risk2 = tracker.compute_settlement_risk()
        assert risk2["total_unsettled"] == Decimal("20000")
        assert risk2["pending_count"] == 1
        assert risk2["cash_committed"] == Decimal("20000")


class TestAddBusinessDays:
    """Unit tests for the business day helper."""

    def test_zero_days(self):
        result = SettlementTracker._add_business_days(date(2026, 3, 30), 0)
        assert result == date(2026, 3, 30)  # Monday, 0 days = same day

    def test_one_business_day(self):
        result = SettlementTracker._add_business_days(date(2026, 3, 30), 1)
        assert result == date(2026, 3, 31)  # Monday + 1 = Tuesday

    def test_across_weekend(self):
        result = SettlementTracker._add_business_days(date(2026, 4, 3), 1)
        assert result == date(2026, 4, 6)  # Friday + 1 = Monday
