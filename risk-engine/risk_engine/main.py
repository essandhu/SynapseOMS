"""FastAPI application for the Risk Engine service.

Wires three concurrent subsystems into a single process:
1. **FastAPI** (REST, port 8081) — risk metrics, portfolio state, health checks
2. **gRPC** (port 50051) — pre-trade risk checks for the Gateway
3. **Kafka consumer** (background thread) — order-lifecycle events → portfolio state

All three share the same in-memory Portfolio object (thread-safe via threading.Lock).
"""

from __future__ import annotations

import os
import signal
import uuid
from contextlib import asynccontextmanager
from decimal import Decimal
from typing import AsyncIterator

import numpy as np
import structlog
from fastapi import FastAPI, Request, Response
from fastapi.middleware.cors import CORSMiddleware

from risk_engine.concentration.analyzer import ConcentrationAnalyzer
from risk_engine.domain.portfolio import Portfolio
from risk_engine.greeks.calculator import GreeksCalculator
from risk_engine.grpc_server.server import serve as grpc_serve
from risk_engine.kafka.consumer import PortfolioStateBuilder
from risk_engine.optimizer.mean_variance import PortfolioOptimizer
from risk_engine.rest.router_optimizer import (
    OptimizerDependencies,
    configure_dependencies as configure_optimizer_dependencies,
    router as optimizer_router,
)
from risk_engine.rest.router_risk import (
    RiskDependencies,
    configure_dependencies,
    router as risk_router,
)
from risk_engine.settlement.tracker import SettlementTracker
from risk_engine.var.historical import HistoricalVaR
from risk_engine.var.monte_carlo import MonteCarloVaR
from risk_engine.var.parametric import ParametricVaR

# ---------------------------------------------------------------------------
# Structlog configuration — JSON output with correlation-ID support
# ---------------------------------------------------------------------------

structlog.configure(
    processors=[
        structlog.contextvars.merge_contextvars,
        structlog.processors.add_log_level,
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.StackInfoRenderer(),
        structlog.processors.format_exc_info,
        structlog.processors.JSONRenderer(),
    ],
    wrapper_class=structlog.make_filtering_bound_logger(0),
    context_class=dict,
    logger_factory=structlog.PrintLoggerFactory(),
    cache_logger_on_first_use=True,
)

logger: structlog.stdlib.BoundLogger = structlog.get_logger()

# ---------------------------------------------------------------------------
# Environment configuration
# ---------------------------------------------------------------------------

KAFKA_BROKERS = os.getenv("KAFKA_BROKERS", "localhost:9092")
GRPC_PORT = int(os.getenv("GRPC_PORT", "50051"))
POSTGRES_URL = os.getenv("POSTGRES_URL", "")
REDIS_URL = os.getenv("REDIS_URL", "")

# ---------------------------------------------------------------------------
# Shared state — single instances used by all three subsystems
# ---------------------------------------------------------------------------

portfolio = Portfolio()
settlement_tracker = SettlementTracker()
historical_var = HistoricalVaR()
parametric_var = ParametricVaR()
monte_carlo_var = MonteCarloVaR(num_simulations=10_000, horizon_days=1, confidence=0.99)
portfolio_optimizer = PortfolioOptimizer()
concentration_analyzer = ConcentrationAnalyzer(
    single_name_threshold=0.25,
    asset_class_threshold=0.50,
)
greeks_calculator = GreeksCalculator()

# ---------------------------------------------------------------------------
# Subsystem references (set during startup, used for health checks & shutdown)
# ---------------------------------------------------------------------------

kafka_consumer: PortfolioStateBuilder | None = None
grpc_server = None


# ---------------------------------------------------------------------------
# Settlement integration callback
# ---------------------------------------------------------------------------


def _on_fill_callback(fill: dict) -> None:
    """Called by the Kafka consumer after each fill is applied to the portfolio.

    Forwards fill details to the SettlementTracker so that pending
    settlement records are created for T+2 instruments.
    """
    settlement_tracker.record_fill(
        instrument_id=fill["instrument_id"],
        asset_class=fill["asset_class"],
        side=fill["side"],
        quantity=Decimal(str(fill["quantity"])),
        price=Decimal(str(fill["price"])),
    )


# ---------------------------------------------------------------------------
# Lifespan — startup / shutdown hooks
# ---------------------------------------------------------------------------


@asynccontextmanager
async def lifespan(_app: FastAPI) -> AsyncIterator[None]:
    """Application lifespan handler: start and stop all subsystems."""
    global kafka_consumer, grpc_server  # noqa: PLW0603

    logger.info("starting_risk_engine", version="0.1.0")

    # 1. Configure REST dependencies ----------------------------------------
    deps = RiskDependencies(
        portfolio=portfolio,
        historical_var=historical_var,
        parametric_var=parametric_var,
        settlement_tracker=settlement_tracker,
        monte_carlo_var=monte_carlo_var,
        greeks_calculator=greeks_calculator,
        concentration_analyzer=concentration_analyzer,
    )
    configure_dependencies(deps)

    # 1b. Configure optimizer dependencies ----------------------------------
    optimizer_deps = OptimizerDependencies(
        portfolio=portfolio,
        optimizer=portfolio_optimizer,
        expected_returns=np.array([]),
        covariance_matrix=np.array([]).reshape(0, 0),
    )
    configure_optimizer_dependencies(optimizer_deps)

    # 2. Start Kafka consumer (background thread) ---------------------------
    kafka_consumer = PortfolioStateBuilder(
        portfolio=portfolio,
        kafka_brokers=KAFKA_BROKERS,
        on_fill=_on_fill_callback,
    )
    kafka_consumer.start()

    # 3. Start gRPC server (background thread pool) -------------------------
    grpc_server = grpc_serve(
        portfolio=portfolio,
        var_engine=parametric_var,
        port=GRPC_PORT,
    )

    logger.info(
        "risk_engine_started",
        kafka=KAFKA_BROKERS,
        grpc_port=GRPC_PORT,
        postgres=POSTGRES_URL or "(not configured)",
        redis=REDIS_URL or "(not configured)",
    )

    yield

    # Graceful shutdown -----------------------------------------------------
    logger.info("shutting_down_risk_engine")

    if kafka_consumer:
        kafka_consumer.stop()

    if grpc_server:
        grpc_server.stop(grace=5)

    logger.info("risk_engine_stopped")


# ---------------------------------------------------------------------------
# FastAPI application
# ---------------------------------------------------------------------------

app = FastAPI(
    title="SynapseOMS Risk Engine",
    version="0.1.0",
    lifespan=lifespan,
)


# ---------------------------------------------------------------------------
# Middleware — inject correlation ID
# ---------------------------------------------------------------------------


@app.middleware("http")
async def correlation_id_middleware(request: Request, call_next) -> Response:  # noqa: ANN001
    """Attach a correlation ID to every request for distributed tracing."""
    correlation_id = request.headers.get("X-Correlation-ID", str(uuid.uuid4()))
    structlog.contextvars.clear_contextvars()
    structlog.contextvars.bind_contextvars(correlation_id=correlation_id)

    response: Response = await call_next(request)
    response.headers["X-Correlation-ID"] = correlation_id
    return response


# ---------------------------------------------------------------------------
# CORS
# ---------------------------------------------------------------------------

app.add_middleware(
    CORSMiddleware,
    allow_origins=[
        "http://localhost:3000",
        "http://localhost:5173",
        "http://127.0.0.1:3000",
        "http://127.0.0.1:5173",
    ],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# ---------------------------------------------------------------------------
# Routers
# ---------------------------------------------------------------------------

app.include_router(risk_router)
app.include_router(optimizer_router)


# ---------------------------------------------------------------------------
# Health endpoint — reports status of all three subsystems
# ---------------------------------------------------------------------------


@app.get("/api/v1/health")
async def health() -> dict:
    """Liveness / readiness probe reporting each subsystem's status."""
    return {
        "status": "ok",
        "fastapi": "ok",
        "grpc": "ok" if grpc_server else "not_started",
        "kafka": (
            "ok"
            if kafka_consumer and kafka_consumer._running
            else "not_started"
        ),
        "monte_carlo_var": "ok" if monte_carlo_var else "not_configured",
        "portfolio_optimizer": "ok" if portfolio_optimizer else "not_configured",
        "concentration_analyzer": "ok" if concentration_analyzer else "not_configured",
        "greeks_calculator": "ok" if greeks_calculator else "not_configured",
    }


# ---------------------------------------------------------------------------
# Graceful shutdown on SIGTERM (e.g. container orchestration)
# ---------------------------------------------------------------------------


def _handle_sigterm(signum: int, frame) -> None:  # noqa: ANN001
    """Raise SystemExit so that FastAPI's lifespan shutdown runs cleanly."""
    logger.info("sigterm_received")
    raise SystemExit(0)


signal.signal(signal.SIGTERM, _handle_sigterm)
