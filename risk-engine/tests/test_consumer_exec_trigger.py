"""Tests for execution report trigger on terminal order states.

Verifies that PortfolioStateBuilder fires the ``on_order_complete``
callback when orders reach terminal states (filled, canceled with
partial fills) and correctly aggregates fill data.
"""

from __future__ import annotations

import json
from decimal import Decimal
from unittest.mock import MagicMock

from risk_engine.domain.portfolio import Portfolio
from risk_engine.kafka.consumer import PortfolioStateBuilder


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_kafka_message(
    payload: dict,
    *,
    headers: list[tuple[str, bytes]] | None = None,
) -> MagicMock:
    """Build a mock Kafka message from a dict payload."""
    msg = MagicMock()
    msg.value.return_value = json.dumps(payload).encode("utf-8")
    msg.headers.return_value = headers
    msg.error.return_value = None
    return msg


def _make_builder(
    portfolio: Portfolio | None = None,
    on_fill: object | None = None,
    on_order_complete: object | None = None,
) -> PortfolioStateBuilder:
    """Create a PortfolioStateBuilder without connecting to Kafka."""
    if portfolio is None:
        portfolio = Portfolio(
            cash=Decimal("100000"),
            available_cash=Decimal("100000"),
        )
    return PortfolioStateBuilder(
        portfolio=portfolio,
        kafka_brokers="localhost:9092",
        on_fill=on_fill,
        on_order_complete=on_order_complete,
    )


def _fill_event(order_id: str = "ORD-100") -> dict:
    """Build a fill_received event payload."""
    return {
        "type": "fill_received",
        "order_id": order_id,
        "fill": {
            "instrument_id": "AAPL",
            "venue_id": "NYSE",
            "side": "BUY",
            "quantity": "50",
            "price": "175.00",
            "asset_class": "equity",
            "settlement_cycle": "T2",
        },
    }


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


class TestOrderFilledTriggersCallback:
    """order_filled events should invoke on_order_complete."""

    def test_order_filled_triggers_callback(self) -> None:
        callback = MagicMock()
        builder = _make_builder(on_order_complete=callback)

        # Send a fill first so aggregated fills are available
        fill_msg = _make_kafka_message(_fill_event("ORD-100"))
        builder._process_message(fill_msg)

        # Now send the terminal order_filled event
        filled_event = {
            "type": "order_filled",
            "order_id": "ORD-100",
        }
        filled_msg = _make_kafka_message(filled_event)
        builder._process_message(filled_msg)

        callback.assert_called_once()
        call_data = callback.call_args[0][0]
        assert call_data["order_id"] == "ORD-100"
        assert len(call_data["fills"]) == 1
        assert call_data["fills"][0]["instrument_id"] == "AAPL"
        assert call_data["event"]["type"] == "order_filled"


class TestOrderCanceledWithFills:
    """order_canceled with prior fills should trigger callback."""

    def test_order_canceled_with_fills_triggers_callback(self) -> None:
        callback = MagicMock()
        builder = _make_builder(on_order_complete=callback)

        # Partial fill
        fill_msg = _make_kafka_message(_fill_event("ORD-200"))
        builder._process_message(fill_msg)

        # Cancel
        cancel_event = {
            "type": "order_canceled",
            "order_id": "ORD-200",
        }
        cancel_msg = _make_kafka_message(cancel_event)
        builder._process_message(cancel_msg)

        callback.assert_called_once()
        call_data = callback.call_args[0][0]
        assert call_data["order_id"] == "ORD-200"
        assert len(call_data["fills"]) == 1
        assert call_data["event"]["type"] == "order_canceled"


class TestOrderCanceledWithoutFills:
    """order_canceled without prior fills should NOT trigger callback."""

    def test_order_canceled_without_fills_skips_callback(self) -> None:
        callback = MagicMock()
        builder = _make_builder(on_order_complete=callback)

        cancel_event = {
            "type": "order_canceled",
            "order_id": "ORD-300",
        }
        cancel_msg = _make_kafka_message(cancel_event)
        builder._process_message(cancel_msg)

        callback.assert_not_called()


class TestMissingCallback:
    """on_order_complete=None should not cause errors."""

    def test_missing_callback_graceful(self) -> None:
        builder = _make_builder(on_order_complete=None)

        # Fill + terminal event with no callback set
        fill_msg = _make_kafka_message(_fill_event("ORD-400"))
        builder._process_message(fill_msg)

        filled_event = {
            "type": "order_filled",
            "order_id": "ORD-400",
        }
        filled_msg = _make_kafka_message(filled_event)
        # Should not raise
        builder._process_message(filled_msg)
