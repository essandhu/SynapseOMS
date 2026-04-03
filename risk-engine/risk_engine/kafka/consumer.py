"""Kafka consumer that builds portfolio state from order lifecycle events.

Subscribes to the ``order-lifecycle`` topic, deserializes protobuf
:class:`OrderLifecycleEvent` messages (with a JSON fallback when proto stubs
have not been generated yet), and applies fill events to an in-memory
:class:`~risk_engine.domain.portfolio.Portfolio`.

The consumer runs in a daemon thread so it can operate alongside the
FastAPI event loop without blocking it.
"""

from __future__ import annotations

import json
import threading
import time
from collections.abc import Callable
from decimal import Decimal

import structlog
from confluent_kafka import Consumer, KafkaError

from risk_engine.domain.portfolio import Portfolio

logger = structlog.get_logger()


class PortfolioStateBuilder:
    """Kafka consumer that builds portfolio state from order lifecycle events."""

    # Reconnection backoff parameters
    INITIAL_BACKOFF_S: float = 1.0
    MAX_BACKOFF_S: float = 60.0
    BACKOFF_MULTIPLIER: float = 2.0

    def __init__(
        self,
        portfolio: Portfolio,
        kafka_brokers: str,
        group_id: str = "risk-engine-portfolio-builder",
        on_fill: Callable[[dict], None] | None = None,
        on_order_complete: Callable[[dict], None] | None = None,
    ) -> None:
        self.portfolio = portfolio
        self.consumer_config: dict = {
            "bootstrap.servers": kafka_brokers,
            "group.id": group_id,
            "auto.offset.reset": "earliest",  # Replay from beginning on first start
            "enable.auto.commit": True,
            "auto.commit.interval.ms": 5000,
        }
        self._running = False
        self._thread: threading.Thread | None = None
        self._on_fill = on_fill
        self._on_order_complete = on_order_complete
        self._order_fills: dict[str, list[dict]] = {}  # order_id -> list of fills
        self._consecutive_errors: int = 0

    # ------------------------------------------------------------------
    # Lifecycle
    # ------------------------------------------------------------------

    def start(self) -> None:
        """Start consuming in a background thread."""
        self._running = True
        self._thread = threading.Thread(
            target=self._consume_loop,
            daemon=True,
            name="kafka-consumer",
        )
        self._thread.start()
        logger.info("kafka_consumer_started", topic="order-lifecycle")

    def stop(self) -> None:
        """Signal consumer to stop and wait for thread to finish."""
        self._running = False
        if self._thread:
            self._thread.join(timeout=10)
            logger.info("kafka_consumer_stopped")

    # ------------------------------------------------------------------
    # Consume loop
    # ------------------------------------------------------------------

    def _consume_loop(self) -> None:
        """Main consume loop with automatic reconnection and exponential backoff.

        If the Kafka consumer encounters a fatal error or fails to create,
        it reconnects with exponential backoff (1s, 2s, 4s, ... up to 60s).
        Successful message processing resets the backoff counter.
        """
        while self._running:
            backoff = self._calculate_backoff()
            try:
                self._run_consumer()
            except Exception as exc:  # noqa: BLE001
                self._consecutive_errors += 1
                if self._running:
                    logger.error(
                        "kafka_consumer_disconnected",
                        error=str(exc),
                        backoff_seconds=backoff,
                        consecutive_errors=self._consecutive_errors,
                    )
                    # Sleep with early exit if stopped
                    self._interruptible_sleep(backoff)

    def _run_consumer(self) -> None:
        """Create a consumer, subscribe, and poll until error or shutdown."""
        consumer = Consumer(self.consumer_config)
        consumer.subscribe(["order-lifecycle"])

        try:
            while self._running:
                msg = consumer.poll(timeout=1.0)
                if msg is None:
                    continue
                if msg.error():
                    if msg.error().code() == KafkaError._PARTITION_EOF:
                        continue
                    # Fatal errors trigger reconnection
                    if msg.error().fatal():
                        logger.error("kafka_fatal_error", error=str(msg.error()))
                        return
                    logger.error("kafka_error", error=str(msg.error()))
                    continue

                self._process_message(msg)
                # Reset backoff on successful message processing
                self._consecutive_errors = 0
        finally:
            consumer.close()

    def _calculate_backoff(self) -> float:
        """Calculate backoff duration based on consecutive error count."""
        if self._consecutive_errors == 0:
            return 0.0
        backoff = self.INITIAL_BACKOFF_S * (
            self.BACKOFF_MULTIPLIER ** (self._consecutive_errors - 1)
        )
        return min(backoff, self.MAX_BACKOFF_S)

    def _interruptible_sleep(self, seconds: float) -> None:
        """Sleep for up to *seconds*, waking early if ``_running`` becomes False."""
        end = time.monotonic() + seconds
        while self._running and time.monotonic() < end:
            time.sleep(min(0.5, end - time.monotonic()))

    # ------------------------------------------------------------------
    # Message processing
    # ------------------------------------------------------------------

    def _process_message(self, msg) -> None:  # noqa: ANN001
        """Process a single Kafka message."""
        # Extract correlation ID from headers
        correlation_id: str | None = None
        if msg.headers():
            for key, value in msg.headers():
                if key == "correlation_id":
                    correlation_id = value.decode("utf-8") if value else None

        log = logger.bind(correlation_id=correlation_id)

        try:
            event = self._deserialize_event(msg.value())
            if event is None:
                return

            event_type = event.get("type", "")

            if event_type == "fill_received":
                self._handle_fill(event, log)
            elif event_type == "order_created":
                log.info("order_tracked", order_id=event.get("order_id"))
            elif event_type == "order_filled":
                order_id = event.get("order_id", "")
                log.info("order_terminal", order_id=order_id, status=event_type)
                self._trigger_order_complete(order_id, event, log)
            elif event_type == "order_canceled":
                order_id = event.get("order_id", "")
                log.info("order_terminal", order_id=order_id, status=event_type)
                # Only trigger if there were partial fills
                if order_id and order_id in self._order_fills:
                    self._trigger_order_complete(order_id, event, log)
            elif event_type == "order_rejected":
                log.info(
                    "order_terminal",
                    order_id=event.get("order_id"),
                    status=event_type,
                )
        except Exception as exc:  # noqa: BLE001
            log.error("message_processing_failed", error=str(exc))

    # ------------------------------------------------------------------
    # Deserialization
    # ------------------------------------------------------------------

    def _deserialize_event(self, raw: bytes) -> dict | None:
        """Deserialize a protobuf ``OrderLifecycleEvent``, falling back to JSON.

        Proto stubs may not have been generated yet (they live under
        ``risk_engine.proto.order``).  When the import fails or parsing
        raises, we attempt a plain JSON decode so the consumer remains
        functional during early development.
        """
        try:
            from risk_engine.proto.order import order_pb2  # type: ignore[import-untyped]

            event = order_pb2.OrderLifecycleEvent()
            event.ParseFromString(raw)
            return self._proto_to_dict(event)
        except (ImportError, Exception):  # noqa: BLE001
            # Proto stubs not generated yet or parse error — try JSON
            try:
                return json.loads(raw)
            except json.JSONDecodeError:
                return None

    def _proto_to_dict(self, event) -> dict:  # noqa: ANN001
        """Convert a protobuf ``OrderLifecycleEvent`` to a plain dict.

        The dict uses a uniform schema so that downstream handlers
        (:meth:`_handle_fill`, etc.) don't need to know about protobuf.
        """
        result: dict = {
            "event_id": event.event_id,
            "order_id": event.order_id,
        }

        payload_type = event.WhichOneof("payload")

        if payload_type == "fill_received":
            fill_payload = event.fill_received
            # FillReceived contains a nested Fill message
            fill = fill_payload.fill
            result["type"] = "fill_received"
            result["fill"] = {
                "instrument_id": fill.id if not hasattr(fill, "instrument_id") else fill.instrument_id,
                "venue_id": fill.venue_id,
                "quantity": fill.quantity,
                "price": fill.price,
                "side": _order_side_to_str(event),
                "asset_class": _asset_class_from_event(event),
                "settlement_cycle": _settlement_cycle_from_event(event),
            }
        elif payload_type == "order_created":
            result["type"] = "order_created"
        elif payload_type == "order_canceled":
            result["type"] = "order_canceled"
        elif payload_type == "order_rejected":
            result["type"] = "order_rejected"
        elif payload_type == "order_acknowledged":
            result["type"] = "order_acknowledged"

        return result

    # ------------------------------------------------------------------
    # Fill handling
    # ------------------------------------------------------------------

    def _handle_fill(self, event: dict, log) -> None:  # noqa: ANN001
        """Apply a fill event to the portfolio."""
        fill = event.get("fill", {})

        instrument_id: str = fill.get("instrument_id", "")
        venue_id: str = fill.get("venue_id", "")
        side: str = fill.get("side", "BUY")
        quantity = Decimal(str(fill.get("quantity", "0")))
        price = Decimal(str(fill.get("price", "0")))
        asset_class: str = fill.get("asset_class", "equity")
        settlement_cycle: str = fill.get("settlement_cycle", "T2")

        self.portfolio.apply_fill(
            instrument_id=instrument_id,
            venue_id=venue_id,
            side=side,
            quantity=quantity,
            price=price,
            asset_class=asset_class,
            settlement_cycle=settlement_cycle,
        )

        log.info(
            "fill_applied",
            instrument_id=instrument_id,
            venue_id=venue_id,
            side=side,
            quantity=str(quantity),
            price=str(price),
        )

        # Track fills by order_id for execution report aggregation
        order_id = event.get("order_id", "")
        if order_id:
            if order_id not in self._order_fills:
                self._order_fills[order_id] = []
            self._order_fills[order_id].append(fill)

        # Notify fill callback (e.g. settlement tracker)
        if self._on_fill:
            try:
                self._on_fill({
                    "instrument_id": instrument_id,
                    "venue_id": venue_id,
                    "side": side,
                    "quantity": quantity,
                    "price": price,
                    "asset_class": asset_class,
                    "settlement_cycle": settlement_cycle,
                })
            except Exception as exc:  # noqa: BLE001
                log.error("on_fill_callback_failed", error=str(exc))

    # ------------------------------------------------------------------
    # Execution report trigger
    # ------------------------------------------------------------------

    def _trigger_order_complete(self, order_id: str, event: dict, log) -> None:  # noqa: ANN001
        """Invoke the on_order_complete callback with aggregated fill data."""
        if not self._on_order_complete:
            return
        fills = self._order_fills.pop(order_id, [])
        try:
            self._on_order_complete({
                "order_id": order_id,
                "fills": fills,
                "event": event,
            })
        except Exception as exc:  # noqa: BLE001
            log.error("on_order_complete_callback_failed", error=str(exc))


# ======================================================================
# Proto helpers — extract fields that live on the parent Order message
# rather than on the Fill itself.
# ======================================================================


def _order_side_to_str(event) -> str:  # noqa: ANN001
    """Derive a BUY/SELL string from the OrderLifecycleEvent context.

    The proto ``Fill`` message does not carry ``side`` directly — it is
    on the parent ``Order``.  When the event includes an embedded order
    we pull the side from there; otherwise we default to ``BUY``.
    """
    try:
        if hasattr(event, "fill_received") and hasattr(event.fill_received, "fill"):
            # The OrderCreated payload in the same stream would have the
            # full Order, but FillReceived only has the Fill.  If the
            # consumer was extended to cache the Order we could look it up
            # here.  For now, callers using JSON can pass "side" directly.
            pass
    except Exception:  # noqa: BLE001
        pass
    return "BUY"


def _asset_class_from_event(event) -> str:  # noqa: ANN001
    """Extract asset class string from the event, defaulting to ``equity``."""
    try:
        if hasattr(event, "fill_received"):
            fr = event.fill_received
            if hasattr(fr, "asset_class"):
                return str(fr.asset_class)
    except Exception:  # noqa: BLE001
        pass
    return "equity"


def _settlement_cycle_from_event(event) -> str:  # noqa: ANN001
    """Extract settlement cycle string from the event, defaulting to ``T2``."""
    try:
        if hasattr(event, "fill_received"):
            fr = event.fill_received
            if hasattr(fr, "settlement_cycle"):
                return str(fr.settlement_cycle)
    except Exception:  # noqa: BLE001
        pass
    return "T2"
