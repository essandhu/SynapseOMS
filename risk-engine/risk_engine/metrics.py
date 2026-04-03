"""Prometheus metrics for the Risk Engine service."""

from prometheus_client import Counter, Gauge, Histogram

var_computation_seconds = Histogram(
    "risk_var_computation_seconds",
    "VaR computation time by method",
    labelnames=["method"],
    buckets=[0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.0, 5.0],
)

pretrade_check_seconds = Histogram(
    "risk_pretrade_check_seconds",
    "gRPC pre-trade check latency",
    buckets=[0.001, 0.005, 0.01, 0.025, 0.05, 0.1],
)

pretrade_check_rejected_total = Counter(
    "risk_pretrade_check_rejected_total",
    "Pre-trade rejections",
)

portfolio_var_ratio = Gauge(
    "risk_portfolio_var_ratio",
    "Current VaR as percentage of NAV",
)

anomalies_detected_total = Counter(
    "risk_anomalies_detected_total",
    "Anomaly alerts fired",
)
