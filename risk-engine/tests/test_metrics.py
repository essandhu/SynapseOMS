"""Tests for Prometheus metrics definitions and behaviour."""

from __future__ import annotations

import prometheus_client
import pytest


@pytest.fixture(autouse=True)
def _reset_registry():
    """Unregister custom collectors between tests so re-imports don't collide."""
    yield
    # Nothing to clean — module-level singletons are fine across a single run.


# ------------------------------------------------------------------
# Import tests — all 5 metrics exist and have the correct types
# ------------------------------------------------------------------


def test_var_computation_seconds_is_histogram():
    from risk_engine.metrics import var_computation_seconds

    assert isinstance(var_computation_seconds, prometheus_client.Histogram)


def test_pretrade_check_seconds_is_histogram():
    from risk_engine.metrics import pretrade_check_seconds

    assert isinstance(pretrade_check_seconds, prometheus_client.Histogram)


def test_pretrade_check_rejected_total_is_counter():
    from risk_engine.metrics import pretrade_check_rejected_total

    assert isinstance(pretrade_check_rejected_total, prometheus_client.Counter)


def test_portfolio_var_ratio_is_gauge():
    from risk_engine.metrics import portfolio_var_ratio

    assert isinstance(portfolio_var_ratio, prometheus_client.Gauge)


def test_anomalies_detected_total_is_counter():
    from risk_engine.metrics import anomalies_detected_total

    assert isinstance(anomalies_detected_total, prometheus_client.Counter)


# ------------------------------------------------------------------
# Behavioral tests
# ------------------------------------------------------------------


def test_histogram_observe():
    from risk_engine.metrics import var_computation_seconds

    # Should not raise — labels(method=...) returns a child that can observe
    var_computation_seconds.labels(method="historical").observe(0.042)


def test_histogram_time_context_manager():
    from risk_engine.metrics import pretrade_check_seconds

    with pretrade_check_seconds.time():
        pass  # just verify the context-manager protocol works


def test_counter_increment():
    from risk_engine.metrics import pretrade_check_rejected_total

    before = pretrade_check_rejected_total._value.get()
    pretrade_check_rejected_total.inc()
    after = pretrade_check_rejected_total._value.get()
    assert after == before + 1


def test_gauge_set():
    from risk_engine.metrics import portfolio_var_ratio

    portfolio_var_ratio.set(0.05)
    assert portfolio_var_ratio._value.get() == 0.05


def test_anomalies_counter_increment():
    from risk_engine.metrics import anomalies_detected_total

    before = anomalies_detected_total._value.get()
    anomalies_detected_total.inc()
    after = anomalies_detected_total._value.get()
    assert after == before + 1
