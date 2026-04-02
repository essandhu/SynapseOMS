"""Kafka consumer that feeds market data to the anomaly detector and publishes alerts."""

from __future__ import annotations

import json
import threading
from collections.abc import Callable
from datetime import datetime, timezone

import structlog
from confluent_kafka import Consumer, Producer, KafkaError

from risk_engine.anomaly.detector import StreamingAnomalyDetector, AnomalyAlert

logger = structlog.get_logger()


class AnomalyAlertPipeline:
    """Consumes market-data snapshots, runs anomaly detection, and publishes alerts."""

    def __init__(
        self,
        detector: StreamingAnomalyDetector,
        kafka_brokers: str,
        on_alert: Callable[[AnomalyAlert], None] | None = None,
    ) -> None:
        self._detector = detector
        self._kafka_brokers = kafka_brokers
        self._on_alert = on_alert
        self._running = False
        self._thread: threading.Thread | None = None
        self._alerts: list[AnomalyAlert] = []
        self._alerts_lock = threading.Lock()
        self._producer: Producer | None = None

    # ------------------------------------------------------------------
    # Lifecycle
    # ------------------------------------------------------------------

    def start(self) -> None:
        """Start consuming in a background thread."""
        self._running = True
        self._producer = Producer({"bootstrap.servers": self._kafka_brokers})
        self._thread = threading.Thread(
            target=self._consume_loop,
            daemon=True,
            name="anomaly-consumer",
        )
        self._thread.start()
        logger.info("anomaly_pipeline_started")

    def stop(self) -> None:
        """Signal consumer to stop and wait for thread to finish."""
        self._running = False
        if self._thread:
            self._thread.join(timeout=10)
        if self._producer:
            self._producer.flush(5000)
        logger.info("anomaly_pipeline_stopped")

    # ------------------------------------------------------------------
    # Alert queries
    # ------------------------------------------------------------------

    def get_recent(self, limit: int = 50) -> list[AnomalyAlert]:
        """Return the most recent alerts sorted by timestamp descending."""
        with self._alerts_lock:
            return sorted(self._alerts, key=lambda a: a.timestamp, reverse=True)[:limit]

    def acknowledge(self, alert_id: str) -> bool:
        """Mark an alert as acknowledged. Returns False if not found."""
        with self._alerts_lock:
            for alert in self._alerts:
                if alert.id == alert_id:
                    alert.acknowledged = True
                    return True
        return False

    # ------------------------------------------------------------------
    # Consume loop
    # ------------------------------------------------------------------

    def _consume_loop(self) -> None:
        """Main consume loop running in background thread."""
        consumer = Consumer({
            "bootstrap.servers": self._kafka_brokers,
            "group.id": "risk-engine-anomaly-detector",
            "auto.offset.reset": "latest",
            "enable.auto.commit": True,
        })
        consumer.subscribe(["market-data"])
        try:
            while self._running:
                msg = consumer.poll(timeout=1.0)
                if msg is None:
                    continue
                if msg.error():
                    if msg.error().code() == KafkaError._PARTITION_EOF:
                        continue
                    logger.error("anomaly_kafka_error", error=str(msg.error()))
                    continue
                self._process_message(msg)
        finally:
            consumer.close()

    # ------------------------------------------------------------------
    # Message processing
    # ------------------------------------------------------------------

    def _process_message(self, msg) -> None:  # noqa: ANN001
        """Process a single Kafka message through the anomaly detector."""
        try:
            snapshot = json.loads(msg.value())
        except (json.JSONDecodeError, TypeError):
            return

        alert = self._detector.ingest(snapshot)
        if alert is None:
            return

        # Store
        with self._alerts_lock:
            self._alerts.append(alert)

        # Publish to Kafka anomaly-alerts topic
        self._publish_alert(alert)

        # Callback (for WebSocket relay)
        if self._on_alert:
            try:
                self._on_alert(alert)
            except Exception as exc:  # noqa: BLE001
                logger.error("on_alert_callback_failed", error=str(exc))

    def _publish_alert(self, alert: AnomalyAlert) -> None:
        """Serialize and produce an alert to the anomaly-alerts topic."""
        if not self._producer:
            return
        payload = json.dumps({
            "id": alert.id,
            "instrument_id": alert.instrument_id,
            "venue_id": alert.venue_id,
            "anomaly_score": alert.anomaly_score,
            "severity": alert.severity,
            "features": alert.features,
            "description": alert.description,
            "timestamp": alert.timestamp.isoformat(),
            "acknowledged": alert.acknowledged,
        })
        self._producer.produce(
            "anomaly-alerts",
            key=alert.instrument_id.encode(),
            value=payload.encode(),
        )
        self._producer.poll(0)  # trigger delivery callbacks
