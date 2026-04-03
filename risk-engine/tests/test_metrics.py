"""Tests for Prometheus metrics definitions and behavior."""

from __future__ import annotations

import pytest
from prometheus_client import CollectorRegistry, Counter, Gauge, Histogram


class TestMetricsImport:
    """Verify all 5 metrics are importable and correctly typed."""

    def test_var_computation_seconds_is_histogram(self) -> None:
        from risk_engine.metrics import var_computation_seconds

        assert isinstance(var_computation_seconds, Histogram)

    def test_pretrade_check_seconds_is_histogram(self) -> None:
        from risk_engine.metrics import pretrade_check_seconds

        assert isinstance(pretrade_check_seconds, Histogram)

    def test_pretrade_check_rejected_total_is_counter(self) -> None:
        from risk_engine.metrics import pretrade_check_rejected_total

        assert isinstance(pretrade_check_rejected_total, Counter)

    def test_portfolio_var_ratio_is_gauge(self) -> None:
        from risk_engine.metrics import portfolio_var_ratio

        assert isinstance(portfolio_var_ratio, Gauge)

    def test_anomalies_detected_total_is_counter(self) -> None:
        from risk_engine.metrics import anomalies_detected_total

        assert isinstance(anomalies_detected_total, Counter)


class TestMetricsBehavior:
    """Verify metric operations work correctly."""

    def test_var_computation_observe(self) -> None:
        from risk_engine.metrics import var_computation_seconds

        # Should not raise
        var_computation_seconds.labels(method="historical").observe(0.123)

    def test_pretrade_check_observe(self) -> None:
        from risk_engine.metrics import pretrade_check_seconds

        pretrade_check_seconds.observe(0.005)

    def test_pretrade_rejected_increment(self) -> None:
        from risk_engine.metrics import pretrade_check_rejected_total

        before = pretrade_check_rejected_total._value.get()
        pretrade_check_rejected_total.inc()
        after = pretrade_check_rejected_total._value.get()
        assert after == before + 1

    def test_portfolio_var_ratio_set(self) -> None:
        from risk_engine.metrics import portfolio_var_ratio

        portfolio_var_ratio.set(0.05)
        assert portfolio_var_ratio._value.get() == 0.05

    def test_anomalies_detected_increment(self) -> None:
        from risk_engine.metrics import anomalies_detected_total

        before = anomalies_detected_total._value.get()
        anomalies_detected_total.inc()
        after = anomalies_detected_total._value.get()
        assert after == before + 1
