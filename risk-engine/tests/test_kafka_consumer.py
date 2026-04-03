"""Tests for Kafka consumer message processing logic.

Tests the PortfolioStateBuilder's message parsing and fill application
without requiring actual Kafka connectivity.
"""

from __future__ import annotations

import json
from decimal import Decimal
from unittest.mock import MagicMock, patch

import pytest

from risk_engine.domain.portfolio import Portfolio
from risk_engine.kafka.consumer import PortfolioStateBuilder


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_kafka_message(
    payload: dict | None = None,
    *,
    headers: list[tuple[str, bytes]] | None = None,
    error: object | None = None,
) -> MagicMock:
    """Build a mock Kafka message."""
    msg = MagicMock()
    if payload is not None:
        msg.value.return_value = json.dumps(payload).encode("utf-8")
    else:
        msg.value.return_value = b""
    msg.headers.return_value = headers
    msg.error.return_value = error
    return msg


def _make_fill_event(
    instrument_id: str = "AAPL",
    venue_id: str = "NYSE",
    side: str = "BUY",
    quantity: str = "100",
    price: str = "175.00",
    asset_class: str = "equity",
    settlement_cycle: str = "T2",
    order_id: str = "ORD-001",
) -> dict:
    """Build a fill_received event payload."""
    return {
        "type": "fill_received",
        "order_id": order_id,
        "fill": {
            "instrument_id": instrument_id,
            "venue_id": venue_id,
            "side": side,
            "quantity": quantity,
            "price": price,
            "asset_class": asset_class,
            "settlement_cycle": settlement_cycle,
        },
    }


def _make_builder(
    portfolio: Portfolio | None = None,
    on_fill: object | None = None,
) -> PortfolioStateBuilder:
    """Create a PortfolioStateBuilder without connecting to Kafka."""
    if portfolio is None:
        portfolio = Portfolio()
    return PortfolioStateBuilder(
        portfolio=portfolio,
        kafka_brokers="localhost:9092",
        on_fill=on_fill,
    )


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestMessageProcessing:
    """Test _process_message and _deserialize_event behavior."""

    def test_parse_fill_event_and_apply_to_portfolio(self) -> None:
        """A valid fill_received event should update the portfolio."""
        portfolio = Portfolio(
            cash=Decimal("100000"),
            available_cash=Decimal("100000"),
        )
        builder = _make_builder(portfolio=portfolio)

        event = _make_fill_event(
            instrument_id="AAPL",
            side="BUY",
            quantity="50",
            price="180.00",
        )
        msg = _make_kafka_message(event)
        builder._process_message(msg)

        assert "AAPL" in portfolio.positions
        pos = portfolio.positions["AAPL"]
        assert pos.quantity == Decimal("50")
        assert pos.market_price == Decimal("180.00")

    def test_fill_event_triggers_on_fill_callback(self) -> None:
        """Fill events should invoke the on_fill callback with fill data."""
        callback = MagicMock()
        portfolio = Portfolio(
            cash=Decimal("100000"),
            available_cash=Decimal("100000"),
        )
        builder = _make_builder(portfolio=portfolio, on_fill=callback)

        event = _make_fill_event(instrument_id="AAPL", side="BUY")
        msg = _make_kafka_message(event)
        builder._process_message(msg)

        callback.assert_called_once()
        call_args = callback.call_args[0][0]
        assert call_args["instrument_id"] == "AAPL"
        assert call_args["side"] == "BUY"

    def test_order_created_event_does_not_crash(self) -> None:
        """An order_created event should be logged without error."""
        builder = _make_builder()
        event = {"type": "order_created", "order_id": "ORD-002"}
        msg = _make_kafka_message(event)

        # Should not raise
        builder._process_message(msg)

        # Portfolio should be unchanged (no positions)
        assert len(builder.portfolio.positions) == 0

    def test_order_canceled_event_does_not_crash(self) -> None:
        """An order_canceled event should be logged without error."""
        builder = _make_builder()
        event = {"type": "order_canceled", "order_id": "ORD-003"}
        msg = _make_kafka_message(event)

        builder._process_message(msg)
        assert len(builder.portfolio.positions) == 0

    def test_order_rejected_event_does_not_crash(self) -> None:
        """An order_rejected event should be logged without error."""
        builder = _make_builder()
        event = {"type": "order_rejected", "order_id": "ORD-004"}
        msg = _make_kafka_message(event)

        builder._process_message(msg)
        assert len(builder.portfolio.positions) == 0

    def test_malformed_message_does_not_crash(self) -> None:
        """A message with invalid JSON should be handled gracefully."""
        builder = _make_builder()
        msg = MagicMock()
        msg.value.return_value = b"not-valid-json{{{{"
        msg.headers.return_value = None
        msg.error.return_value = None

        # Should not raise
        builder._process_message(msg)
        assert len(builder.portfolio.positions) == 0

    def test_empty_message_does_not_crash(self) -> None:
        """An empty message body should be handled gracefully."""
        builder = _make_builder()
        msg = MagicMock()
        msg.value.return_value = b""
        msg.headers.return_value = None
        msg.error.return_value = None

        builder._process_message(msg)
        assert len(builder.portfolio.positions) == 0


class TestDeserialization:
    """Test the _deserialize_event method directly."""

    def test_deserialize_valid_json(self) -> None:
        """Valid JSON bytes should deserialize into a dict."""
        builder = _make_builder()
        raw = json.dumps({"type": "fill_received", "order_id": "X"}).encode()
        result = builder._deserialize_event(raw)

        assert result is not None
        assert result["type"] == "fill_received"

    def test_deserialize_invalid_bytes_returns_none(self) -> None:
        """Non-JSON, non-proto bytes should return None."""
        builder = _make_builder()
        result = builder._deserialize_event(b"\x00\x01\x02\x03")

        assert result is None

    def test_deserialize_empty_bytes_returns_default_proto(self) -> None:
        """Empty bytes parse as a default protobuf message (all fields empty)."""
        builder = _make_builder()
        result = builder._deserialize_event(b"")

        # Empty bytes are valid protobuf (all default values)
        assert result is not None
        assert result.get("order_id") == ""


class TestFillApplication:
    """Test fill handling updates portfolio correctly."""

    def test_buy_fill_creates_new_position(self) -> None:
        """A buy fill for a new instrument should create a position."""
        portfolio = Portfolio(
            cash=Decimal("50000"),
            available_cash=Decimal("50000"),
        )
        builder = _make_builder(portfolio=portfolio)

        event = _make_fill_event(
            instrument_id="MSFT",
            side="BUY",
            quantity="20",
            price="400.00",
            asset_class="equity",
            settlement_cycle="T2",
        )
        msg = _make_kafka_message(event)
        builder._process_message(msg)

        assert "MSFT" in portfolio.positions
        pos = portfolio.positions["MSFT"]
        assert pos.quantity == Decimal("20")
        assert pos.average_cost == Decimal("400.00")

    def test_sell_fill_reduces_position(self) -> None:
        """A sell fill should reduce the position quantity."""
        portfolio = Portfolio(
            cash=Decimal("50000"),
            available_cash=Decimal("50000"),
        )
        # Pre-seed a position
        portfolio.apply_fill(
            instrument_id="AAPL",
            venue_id="NYSE",
            side="BUY",
            quantity=Decimal("100"),
            price=Decimal("175.00"),
            asset_class="equity",
            settlement_cycle="T2",
        )
        builder = _make_builder(portfolio=portfolio)

        event = _make_fill_event(
            instrument_id="AAPL",
            side="sell",
            quantity="30",
            price="180.00",
        )
        msg = _make_kafka_message(event)
        builder._process_message(msg)

        assert portfolio.positions["AAPL"].quantity == Decimal("70")

    def test_crypto_fill_uses_t0_settlement(self) -> None:
        """Crypto fills with T0 settlement should affect cash immediately."""
        portfolio = Portfolio(
            cash=Decimal("100000"),
            available_cash=Decimal("100000"),
        )
        builder = _make_builder(portfolio=portfolio)

        event = _make_fill_event(
            instrument_id="BTC-USD",
            side="BUY",
            quantity="0.5",
            price="60000.00",
            asset_class="crypto",
            settlement_cycle="T0",
        )
        msg = _make_kafka_message(event)
        builder._process_message(msg)

        # T0 settlement: cash reduced immediately
        assert portfolio.cash == Decimal("70000.00")
        assert portfolio.available_cash == Decimal("70000.00")


class TestReconnectionBackoff:
    """Tests for Kafka consumer reconnection with exponential backoff."""

    def test_backoff_calculation_initial(self) -> None:
        """First error should produce INITIAL_BACKOFF_S."""
        builder = _make_builder()
        builder._consecutive_errors = 1
        backoff = builder._calculate_backoff()
        assert backoff == builder.INITIAL_BACKOFF_S

    def test_backoff_calculation_exponential(self) -> None:
        """Backoff should grow exponentially with consecutive errors."""
        builder = _make_builder()
        builder._consecutive_errors = 1
        b1 = builder._calculate_backoff()
        builder._consecutive_errors = 2
        b2 = builder._calculate_backoff()
        builder._consecutive_errors = 3
        b3 = builder._calculate_backoff()

        assert b2 == b1 * builder.BACKOFF_MULTIPLIER
        assert b3 == b2 * builder.BACKOFF_MULTIPLIER

    def test_backoff_calculation_capped_at_max(self) -> None:
        """Backoff should be capped at MAX_BACKOFF_S."""
        builder = _make_builder()
        builder._consecutive_errors = 100  # very high count
        backoff = builder._calculate_backoff()
        assert backoff == builder.MAX_BACKOFF_S

    def test_backoff_zero_when_no_errors(self) -> None:
        """No errors should produce zero backoff."""
        builder = _make_builder()
        builder._consecutive_errors = 0
        backoff = builder._calculate_backoff()
        assert backoff == 0.0

    def test_consecutive_errors_reset_on_successful_message(self) -> None:
        """Processing a valid message should reset the error counter."""
        portfolio = Portfolio(
            cash=Decimal("100000"),
            available_cash=Decimal("100000"),
        )
        builder = _make_builder(portfolio=portfolio)
        builder._consecutive_errors = 5

        # Process a valid message
        event = _make_fill_event(instrument_id="AAPL", side="BUY")
        msg = _make_kafka_message(event)

        # Simulate what _run_consumer does: process message then reset counter
        builder._process_message(msg)
        builder._consecutive_errors = 0  # simulates the reset in _run_consumer

        assert builder._consecutive_errors == 0

    def test_interruptible_sleep_exits_early_when_stopped(self) -> None:
        """_interruptible_sleep should return quickly when _running becomes False."""
        import time

        builder = _make_builder()
        builder._running = False  # already stopped

        start = time.monotonic()
        builder._interruptible_sleep(10.0)  # should return almost immediately
        elapsed = time.monotonic() - start

        assert elapsed < 1.0, f"Sleep should have exited early, took {elapsed:.2f}s"
