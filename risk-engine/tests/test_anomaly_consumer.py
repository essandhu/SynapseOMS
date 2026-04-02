"""Tests for the AnomalyAlertPipeline Kafka consumer."""

from __future__ import annotations

import json
from datetime import datetime, timezone
from unittest.mock import MagicMock, patch

import pytest

from risk_engine.anomaly.detector import AnomalyAlert, StreamingAnomalyDetector


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_alert(**overrides) -> AnomalyAlert:
    defaults = {
        "id": "alert-001",
        "instrument_id": "BTC-USD",
        "venue_id": "binance",
        "anomaly_score": -0.65,
        "severity": "warning",
        "features": {"volume_zscore": 3.2},
        "description": "BTC-USD on binance volume 3.2x above rolling mean",
        "timestamp": datetime(2026, 4, 1, 12, 0, 0, tzinfo=timezone.utc),
        "acknowledged": False,
    }
    defaults.update(overrides)
    return AnomalyAlert(**defaults)


def _mock_kafka_message(payload: dict) -> MagicMock:
    """Create a mock Kafka message whose .value() returns JSON bytes."""
    msg = MagicMock()
    msg.value.return_value = json.dumps(payload).encode()
    msg.error.return_value = None
    return msg


SAMPLE_SNAPSHOT = {
    "instrument_id": "BTC-USD",
    "venue_id": "binance",
    "volume": 100.0,
    "price": 50000.0,
    "bid": 49990.0,
    "ask": 50010.0,
}


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

@patch("risk_engine.anomaly.consumer.Producer")
@patch("risk_engine.anomaly.consumer.Consumer")
class TestAnomalyAlertPipeline:
    """Unit tests for AnomalyAlertPipeline._process_message path."""

    def _make_pipeline(self, MockConsumer, MockProducer, on_alert=None):
        from risk_engine.anomaly.consumer import AnomalyAlertPipeline

        detector = MagicMock(spec=StreamingAnomalyDetector)
        detector.ingest.return_value = None  # default: no anomaly

        pipeline = AnomalyAlertPipeline(
            detector=detector,
            kafka_brokers="localhost:9092",
            on_alert=on_alert,
        )
        # Simulate start (create producer) without actually starting thread
        pipeline._producer = MockProducer({"bootstrap.servers": "localhost:9092"})
        return pipeline, detector

    # 1. detector.ingest() is called with parsed snapshot
    def test_detector_is_called_on_message(self, MockConsumer, MockProducer):
        pipeline, detector = self._make_pipeline(MockConsumer, MockProducer)
        msg = _mock_kafka_message(SAMPLE_SNAPSHOT)

        pipeline._process_message(msg)

        detector.ingest.assert_called_once_with(SAMPLE_SNAPSHOT)

    # 2. Alert is stored and retrievable via get_recent()
    def test_alert_is_stored(self, MockConsumer, MockProducer):
        pipeline, detector = self._make_pipeline(MockConsumer, MockProducer)
        alert = _make_alert()
        detector.ingest.return_value = alert
        msg = _mock_kafka_message(SAMPLE_SNAPSHOT)

        pipeline._process_message(msg)

        recent = pipeline.get_recent()
        assert len(recent) == 1
        assert recent[0].id == "alert-001"

    # 3. Alert is published to "anomaly-alerts" Kafka topic
    def test_alert_is_published_to_kafka(self, MockConsumer, MockProducer):
        pipeline, detector = self._make_pipeline(MockConsumer, MockProducer)
        alert = _make_alert()
        detector.ingest.return_value = alert
        msg = _mock_kafka_message(SAMPLE_SNAPSHOT)

        pipeline._process_message(msg)

        pipeline._producer.produce.assert_called_once()
        call_kwargs = pipeline._producer.produce.call_args
        assert call_kwargs[1]["topic"] if "topic" in (call_kwargs[1] or {}) else call_kwargs[0][0] == "anomaly-alerts"

    # 4. on_alert callback is invoked when alert detected
    def test_callback_invoked_on_alert(self, MockConsumer, MockProducer):
        callback = MagicMock()
        pipeline, detector = self._make_pipeline(MockConsumer, MockProducer, on_alert=callback)
        alert = _make_alert()
        detector.ingest.return_value = alert
        msg = _mock_kafka_message(SAMPLE_SNAPSHOT)

        pipeline._process_message(msg)

        callback.assert_called_once_with(alert)

    # 5. acknowledge() sets acknowledged=True
    def test_acknowledge_flips_flag(self, MockConsumer, MockProducer):
        pipeline, detector = self._make_pipeline(MockConsumer, MockProducer)
        alert = _make_alert(id="ack-test-001")
        detector.ingest.return_value = alert
        msg = _mock_kafka_message(SAMPLE_SNAPSHOT)
        pipeline._process_message(msg)

        result = pipeline.acknowledge("ack-test-001")

        assert result is True
        assert pipeline.get_recent()[0].acknowledged is True

    # 6. acknowledge() returns False for unknown ID
    def test_acknowledge_unknown_returns_false(self, MockConsumer, MockProducer):
        pipeline, detector = self._make_pipeline(MockConsumer, MockProducer)

        result = pipeline.acknowledge("nonexistent-id")

        assert result is False

    # 7. get_recent() returns alerts sorted descending by timestamp
    def test_get_recent_sorted_by_time(self, MockConsumer, MockProducer):
        pipeline, detector = self._make_pipeline(MockConsumer, MockProducer)

        older = _make_alert(id="old", timestamp=datetime(2026, 4, 1, 10, 0, 0, tzinfo=timezone.utc))
        newer = _make_alert(id="new", timestamp=datetime(2026, 4, 1, 14, 0, 0, tzinfo=timezone.utc))
        middle = _make_alert(id="mid", timestamp=datetime(2026, 4, 1, 12, 0, 0, tzinfo=timezone.utc))

        # Ingest in non-sorted order
        for alert in [older, newer, middle]:
            detector.ingest.return_value = alert
            pipeline._process_message(_mock_kafka_message(SAMPLE_SNAPSHOT))

        recent = pipeline.get_recent()
        assert [a.id for a in recent] == ["new", "mid", "old"]
