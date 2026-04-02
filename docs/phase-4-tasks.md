# Phase 4 Tasks — AI Features

**Goal:** User gets post-trade analysis in plain English; user describes a rebalancing goal in natural language and gets an executable trade list.

**Acceptance Test:** After a trade, user sees an AI-generated execution quality report with a letter grade, implementation shortfall, and recommendations. User types "reduce crypto to 30%, maximize Sharpe, keep turnover under $5K" → gets a proposed trade list → executes it. User sees an anomaly alert when the system detects unusual volume on a monitored instrument.

**Architecture Doc References:** Sections 5.2 (AI Execution Analyst), 5.3 (Portfolio Rebalancing Assistant), 5.4 (Market Data Anomaly Detection), 4B (StreamingAnomalyDetector, REST endpoints), 4C (InsightsPanel.tsx, stores, WebSocket streams), Appendix A.3 (AI Rebalancing sequence diagram)

**Previous Phase Review:** Phase 3 completed 28/28 tasks (all pass, 0 catch-up). Key items relevant to Phase 4:
- Optimizer (cvxpy) exists at `risk-engine/risk_engine/optimizer/mean_variance.py` with `OptimizationConstraints` in `constraints.py`. The rebalancing assistant produces constraints JSON that feeds into this optimizer.
- REST endpoint `POST /api/v1/optimizer/optimize` exists at `risk-engine/risk_engine/rest/router_optimizer.py`. Phase 4 adds a new `POST /api/v1/ai/rebalance` endpoint that wraps AI constraint extraction + optimizer.
- Kafka producer already publishes to `order-lifecycle`, `market-data`, `venue-status` topics from Gateway. Phase 4 adds `anomaly-alerts` topic (produced by Risk Engine, consumed by Gateway for WebSocket relay).
- WebSocket Hub (`gateway/internal/ws/hub.go`) uses StreamType enum + per-type broadcast. Adding anomaly alerts requires a new `StreamAnomalies` type and `/ws/anomalies` endpoint.
- Risk Engine main.py wires all modules at startup with dependency injection pattern. New anomaly detector and AI routes follow the same pattern.
- Dashboard uses Zustand stores + `ky`-based REST client + `reconnecting-websocket` for real-time streams. All established patterns in Phase 2/3.
- AI service (`ai/`) currently only contains `smart_router_ml/` (Phase 3). Phase 4 adds `execution_analyst/` and `rebalancing_assistant/` packages.
- No architecture divergences from Phase 3 affect Phase 4 design.

---

## Tasks

### P4-01: AI Execution Analyst — Prompt Template + Domain Types

**Service:** AI
**Files:**
- `ai/execution_analyst/__init__.py` (create)
- `ai/execution_analyst/prompt_templates.py` (create)
- `ai/execution_analyst/types.py` (create)
**Dependencies:** None
**Acceptance Criteria:**
- `EXECUTION_ANALYSIS_PROMPT` string template with all placeholders from architecture Section 5.2
- `TradeContext` dataclass with fields: side, quantity, instrument_id, asset_class, order_type, limit_price, submitted_at, completed_at, venues, fill_count, fill_table, arrival_price, spread_bps, vwap_5min, adv_30d, size_pct_adv, venue_comparison_table
- `TradeContext.to_prompt_vars()` method returns dict suitable for `EXECUTION_ANALYSIS_PROMPT.format(**vars)`
- `ExecutionReport` dataclass: overall_grade (str, A-F), implementation_shortfall_bps (float), summary (str), venue_analysis (list of dicts with venue/grade/comment), recommendations (list of str), market_impact_estimate_bps (float)
- Unit tests: TradeContext.to_prompt_vars() returns all expected keys; ExecutionReport round-trips from JSON

**Architecture Context:**
From Section 5.2, the prompt template:
```python
EXECUTION_ANALYSIS_PROMPT = """You are an institutional-grade trade execution analyst.
Analyze the following completed trade and provide a concise execution quality report.

## Trade Summary
- Order: {side} {quantity} {instrument_id} ({asset_class})
- Order type: {order_type}, limit price: {limit_price}
- Submitted: {submitted_at}, completed: {completed_at}
- Venue(s) used: {venues}
- Total fills: {fill_count}

## Fill Details
{fill_table}

## Market Context at Submission
- Arrival price (mid at submission): {arrival_price}
- Spread at submission: {spread_bps} bps
- 5-minute VWAP around execution: {vwap_5min}
- 30-day average daily volume: {adv_30d}
- Order size as % of ADV: {size_pct_adv}%

## Venue Comparison
{venue_comparison_table}

## Instructions
Provide your analysis in the following JSON structure:
{{
  "overall_grade": "A/B/C/D/F",
  "implementation_shortfall_bps": <number>,
  "summary": "<2-3 sentence executive summary>",
  "venue_analysis": [
    {{"venue": "<name>", "grade": "<A-F>", "comment": "<1 sentence>"}}
  ],
  "recommendations": ["<actionable suggestion 1>", "..."],
  "market_impact_estimate_bps": <number>
}}
"""
```

---

### P4-02: AI Execution Analyst — Anthropic API Integration

**Service:** AI
**Files:**
- `ai/execution_analyst/analyst.py` (create)
- `ai/execution_analyst/tests/__init__.py` (create)
- `ai/execution_analyst/tests/test_analyst.py` (create)
**Dependencies:** P4-01 (types and prompt template)
**Acceptance Criteria:**
- `ExecutionAnalyst` class with `__init__(self)` that creates `anthropic.Anthropic()` client using `ANTHROPIC_API_KEY` env var
- `model` field set to `"claude-sonnet-4-6"` (fast + capable for structured analysis)
- `async analyze_execution(self, trade_context: TradeContext) -> ExecutionReport` method:
  - Builds prompt using `EXECUTION_ANALYSIS_PROMPT.format(**trade_context.to_prompt_vars())`
  - Calls `self.client.messages.create(model=self.model, max_tokens=1024, messages=[...])`
  - Parses JSON from `response.content[0].text` into `ExecutionReport`
  - Handles JSON parse errors gracefully (returns a fallback report with grade "N/A" and error in summary)
- Rate limiting: class-level counter, max 10 analyses per hour (raises `RateLimitExceeded` if exceeded)
- Unit tests: mock the Anthropic client, verify prompt is formatted correctly, verify response parsing, verify rate limiting, verify JSON parse error fallback

**Architecture Context:**
From Section 5.2:
```python
class ExecutionAnalyst:
    def __init__(self):
        self.client = anthropic.Anthropic()  # Uses ANTHROPIC_API_KEY env var
        self.model = "claude-sonnet-4-6"

    async def analyze_execution(self, trade_context: TradeContext) -> ExecutionReport:
        prompt = EXECUTION_ANALYSIS_PROMPT.format(**trade_context.to_prompt_vars())
        response = self.client.messages.create(
            model=self.model, max_tokens=1024,
            messages=[{"role": "user", "content": prompt}],
        )
        report_json = json.loads(response.content[0].text)
        return ExecutionReport(**report_json)
```

Trigger: automatically runs when an order reaches terminal state (Filled, Canceled with partial fills). Rate-limited to max 10 analyses per hour.

---

### P4-03: AI Rebalancing Assistant — Prompt Template + Types

**Service:** AI
**Files:**
- `ai/rebalancing_assistant/__init__.py` (create)
- `ai/rebalancing_assistant/prompt_templates.py` (create)
- `ai/rebalancing_assistant/types.py` (create)
**Dependencies:** None
**Acceptance Criteria:**
- `CONSTRAINT_EXTRACTION_PROMPT` string template with all placeholders from architecture Section 5.3
- `RebalanceRequest` dataclass: user_input (str), portfolio_summary (str), available_instruments (str)
- `RebalanceRequest.to_prompt_vars()` returns dict for template formatting
- `ExtractedConstraints` dataclass matching the JSON schema: objective, target_return, risk_aversion, long_only, max_single_weight, asset_class_bounds, sector_limits, target_volatility, max_turnover_usd, instruments_to_include, instruments_to_exclude, reasoning
- `ExtractedConstraints.to_optimization_constraints()` method that converts to the existing `OptimizationConstraints` dataclass (from `risk_engine.optimizer.constraints`)
- Unit tests: prompt formatting, ExtractedConstraints from JSON, conversion to OptimizationConstraints

**Architecture Context:**
From Section 5.3, the constraint extraction prompt:
```python
CONSTRAINT_EXTRACTION_PROMPT = """You are a portfolio construction assistant.
The user will describe a rebalancing goal in natural language. Extract structured
optimization constraints from their description.

Current portfolio:
{portfolio_summary}

Available instruments:
{available_instruments}

User request: "{user_input}"

Extract constraints as JSON:
{{
  "objective": "maximize_sharpe" | "minimize_variance" | "target_return",
  "target_return": <float or null>,
  "risk_aversion": <float, default 1.0>,
  "long_only": <bool>,
  "max_single_weight": <float or null>,
  "asset_class_bounds": {{"equity": [<min>, <max>], "crypto": [<min>, <max>]}} or null,
  "sector_limits": {{}} or null,
  "target_volatility": <float or null>,
  "max_turnover_usd": <float or null>,
  "instruments_to_include": [<list>] or null,
  "instruments_to_exclude": [<list>] or null,
  "reasoning": "<1-2 sentences explaining interpretation>"
}}

If the user's request is ambiguous, set conservative defaults and explain in reasoning.
"""
```

The existing `OptimizationConstraints` (in `risk-engine/risk_engine/optimizer/constraints.py`) has: risk_aversion, long_only, max_single_weight, sector_limits, target_volatility, max_turnover, asset_class_bounds. The `to_optimization_constraints()` method maps extracted fields to this structure, converting `max_turnover_usd` to weight-based `max_turnover` by dividing by portfolio NAV.

---

### P4-04: AI Rebalancing Assistant — Anthropic API Integration

**Service:** AI
**Files:**
- `ai/rebalancing_assistant/assistant.py` (create)
- `ai/rebalancing_assistant/tests/__init__.py` (create)
- `ai/rebalancing_assistant/tests/test_assistant.py` (create)
**Dependencies:** P4-03 (types and prompt template)
**Acceptance Criteria:**
- `RebalancingAssistant` class with `anthropic.Anthropic()` client, model `"claude-sonnet-4-6"`
- `async extract_constraints(self, request: RebalanceRequest) -> ExtractedConstraints` method:
  - Builds prompt using `CONSTRAINT_EXTRACTION_PROMPT.format(**request.to_prompt_vars())`
  - Calls Anthropic API with `max_tokens=1024`
  - Parses JSON response into `ExtractedConstraints`
  - Validates extracted constraints (e.g., bounds are feasible, instruments exist in available list)
- Returns validation errors as part of `ExtractedConstraints.reasoning` if ambiguous
- Unit tests: mock Anthropic client, verify prompt includes portfolio context, verify JSON parsing, verify validation catches infeasible bounds

**Architecture Context:**
From Section 5.3 and Appendix A.3, the flow is:
1. User NL input arrives at `POST /api/v1/ai/rebalance`
2. Risk Engine calls this assistant with current portfolio context + user prompt
3. LLM returns structured constraints JSON
4. Constraints validated (bounds feasible, instruments exist)
5. Constraints converted to `OptimizationConstraints` and fed to existing optimizer
6. Optimizer returns target weights + trade list
7. Response includes proposed trades with before/after comparison

---

### P4-05: AI Dependencies Update

**Service:** AI
**Files:**
- `ai/requirements.txt` (modify)
**Dependencies:** None
**Acceptance Criteria:**
- Add `anthropic>=0.40,<1` to requirements.txt (for Anthropic API client)
- Add `scikit-learn>=1.4,<2` to requirements.txt (for Isolation Forest in anomaly detection, shared dependency)
- Existing dependencies (fastapi, uvicorn, xgboost, numpy, pandas, pytest, httpx) remain unchanged
- Verify no version conflicts between new and existing dependencies

**Architecture Context:**
The `ai/` service currently serves the ML scorer sidecar (FastAPI on port 8090). The execution analyst and rebalancing assistant are library modules imported by the Risk Engine, not standalone services. However, they share the `ai/` package and its dependencies. The `anthropic` package is needed for both AI modules. `scikit-learn` provides `IsolationForest` used by the anomaly detector (implemented in `risk-engine/` but the dependency pattern follows the project convention).

---

### P4-06: Anomaly Detection — Streaming Isolation Forest Detector

**Service:** Risk Engine
**Files:**
- `risk-engine/risk_engine/anomaly/__init__.py` (create)
- `risk-engine/risk_engine/anomaly/detector.py` (create)
- `risk-engine/tests/test_anomaly_detector.py` (create)
**Dependencies:** None
**Acceptance Criteria:**
- `AnomalyAlert` dataclass: id (str, UUID), instrument_id, venue_id, anomaly_score (float), severity (str: "info"/"warning"/"critical"), features (dict[str, float]), description (str), timestamp (datetime), acknowledged (bool, default False)
- Severity thresholds: info: score < -0.3, warning: score < -0.5, critical: score < -0.7
- `StreamingAnomalyDetector` class:
  - `__init__(retrain_interval_minutes=60, contamination=0.01)` — creates `IsolationForest(contamination=contamination, random_state=42)`
  - `feature_window: deque[np.ndarray]` with maxlen=10_000
  - `ingest(snapshot: MarketDataSnapshot) -> AnomalyAlert | None` — extracts features, appends to window, retrains if due, scores, returns alert if anomalous
  - `_extract_features(snapshot)` — produces feature vector with: volume z-score (vs 30-day rolling mean), price return z-score, bid-ask spread z-score, volume/price correlation shift, cross-venue price divergence
  - `_should_retrain()` — True if retrain_interval has elapsed since last retrain
  - `_describe_features(features)` — returns dict mapping feature names to values for alert context
- Human-readable description in alert: e.g., "ETH-USD volume on Binance 4.2x above 30-day mean"
- Unit tests: feature extraction produces correct shape (5 features); normal data returns None; extreme outlier triggers alert; severity thresholds correct; retrain interval respected

**Architecture Context:**
From Section 4B and Section 5.4:
```python
class StreamingAnomalyDetector:
    """
    Isolation Forest on streaming market data features.
    Maintains a sliding window of feature vectors and retrains periodically.

    Features per instrument per window:
    - Volume z-score (vs 30-day rolling mean)
    - Price return z-score
    - Bid-ask spread z-score
    - Volume/price correlation shift
    - Cross-venue price divergence (if multi-venue)
    """
    def __init__(self, retrain_interval_minutes=60, contamination=0.01):
        self.model = IsolationForest(contamination=contamination, random_state=42)
        self.feature_window: deque[np.ndarray] = deque(maxlen=10_000)

    def ingest(self, snapshot: MarketDataSnapshot) -> AnomalyAlert | None:
        features = self._extract_features(snapshot)
        self.feature_window.append(features)
        if self._should_retrain():
            self.model.fit(np.array(self.feature_window))
        score = self.model.decision_function(features.reshape(1, -1))[0]
        if score < self.anomaly_threshold:
            return AnomalyAlert(...)
        return None
```

The detector needs enough data in the window (at least `contamination * maxlen` samples) before meaningful scoring. Before the model is first trained, `ingest()` should accumulate features silently and return None.

---

### P4-07: Anomaly Detection — Kafka Consumer + Alert Pipeline

**Service:** Risk Engine
**Files:**
- `risk-engine/risk_engine/anomaly/consumer.py` (create)
- `risk-engine/tests/test_anomaly_consumer.py` (create)
**Dependencies:** P4-06 (StreamingAnomalyDetector)
**Acceptance Criteria:**
- `AnomalyAlertPipeline` class:
  - Consumes from Kafka topic `market-data` (same topic the existing PortfolioStateBuilder reads from — separate consumer group)
  - Consumer group: `"risk-engine-anomaly-detector"`
  - Deserializes market data snapshots (JSON, same format as existing Kafka messages)
  - Feeds each snapshot to `StreamingAnomalyDetector.ingest()`
  - When an alert is returned:
    1. Publishes to Kafka topic `anomaly-alerts` (requires a Kafka producer in Risk Engine)
    2. Stores alert in an in-memory list (for REST API retrieval)
    3. Calls an optional `on_alert` callback (for WebSocket relay)
  - Runs in a daemon thread (same pattern as `PortfolioStateBuilder`)
- `start()` and `stop()` lifecycle methods
- In-memory alert storage with `get_recent(limit=50) -> list[AnomalyAlert]` for REST
- Alert acknowledging: `acknowledge(alert_id: str) -> bool`
- Unit tests: mock Kafka consumer, verify detector is called, verify alert is stored and published, verify acknowledge flips flag

**Architecture Context:**
The existing Kafka consumer pattern is in `risk-engine/risk_engine/kafka/consumer.py` (`PortfolioStateBuilder`). It runs in a daemon thread with `start()`/`stop()` and uses `confluent_kafka.Consumer`. Follow the same pattern.

The `anomaly-alerts` Kafka topic is new. The Risk Engine needs a Kafka producer to publish alerts. Use `confluent_kafka.Producer` with the same broker config (`KAFKA_BROKERS` env var).

Alert structure for Kafka message:
```json
{
  "id": "uuid",
  "instrument_id": "ETH-USD",
  "venue_id": "binance",
  "anomaly_score": -0.65,
  "severity": "warning",
  "features": {"volume_zscore": 4.2, "spread_zscore": 1.1},
  "description": "ETH-USD volume on Binance 4.2x above 30-day mean",
  "timestamp": "2026-04-15T14:30:00Z",
  "acknowledged": false
}
```

---

### P4-08: Risk Engine — Anomaly REST Endpoints

**Service:** Risk Engine
**Files:**
- `risk-engine/risk_engine/rest/router_anomaly.py` (create)
- `risk-engine/tests/test_rest_anomaly.py` (create)
**Dependencies:** P4-07 (AnomalyAlertPipeline with in-memory storage)
**Acceptance Criteria:**
- `GET /api/v1/anomalies` — returns recent anomaly alerts (default limit 50), sorted by timestamp descending
  - Query params: `limit` (int, default 50), `severity` (optional filter: "info"/"warning"/"critical"), `instrument_id` (optional filter)
  - Response: `{ "alerts": [...], "total": N }`
- `POST /api/v1/anomalies/{alert_id}/acknowledge` — marks an alert as acknowledged
  - Returns 200 with updated alert or 404 if not found
- Dependency injection pattern matching existing routers: `AnomalyDependencies` class, `configure_dependencies()` function
- Unit tests: GET returns alerts sorted by time, filter by severity works, filter by instrument works, acknowledge flips flag, 404 for unknown ID

**Architecture Context:**
Follow the same pattern as `router_risk.py` and `router_optimizer.py`:
```python
router = APIRouter(prefix="/api/v1", tags=["anomaly"])

class AnomalyDependencies:
    def __init__(self, alert_pipeline: AnomalyAlertPipeline | None = None):
        self.alert_pipeline = alert_pipeline

_deps: AnomalyDependencies | None = None

def configure_dependencies(deps: AnomalyDependencies) -> None:
    global _deps
    _deps = deps
```

The existing architecture doc lists `GET /api/v1/anomalies` as a Risk Engine REST endpoint (Section 4B REST table).

---

### P4-09: Risk Engine — AI Rebalancing Endpoint

**Service:** Risk Engine
**Files:**
- `risk-engine/risk_engine/rest/router_ai.py` (create)
- `risk-engine/tests/test_rest_ai.py` (create)
**Dependencies:** P4-04 (RebalancingAssistant), P4-03 (ExtractedConstraints.to_optimization_constraints)
**Acceptance Criteria:**
- `POST /api/v1/ai/rebalance` endpoint:
  - Request body: `{ "prompt": "reduce crypto to 30%, maximize Sharpe, keep turnover under $5K" }`
  - Builds `RebalanceRequest` with current portfolio summary and available instruments from portfolio state
  - Calls `RebalancingAssistant.extract_constraints()` with the request
  - Converts `ExtractedConstraints` to `OptimizationConstraints` via `.to_optimization_constraints()`
  - Runs existing `PortfolioOptimizer.optimize()` with the constraints
  - Returns: `{ "constraints": {...extracted}, "optimization": {...result}, "reasoning": "..." }`
- Error handling: returns 422 if constraints are infeasible, 503 if Anthropic API is unavailable
- `POST /api/v1/ai/execution-report` endpoint:
  - Request body: `TradeContext` JSON (order ID or full trade context)
  - Calls `ExecutionAnalyst.analyze_execution()`
  - Returns `ExecutionReport` JSON
- `GET /api/v1/ai/execution-reports` endpoint:
  - Returns stored execution reports, sorted by recency
  - Query param: `limit` (default 20)
- Dependency injection: `AIDependencies` class with `execution_analyst`, `rebalancing_assistant`, `optimizer`, `portfolio`
- Unit tests: mock AI modules, verify rebalance flow end-to-end, verify execution report storage and retrieval

**Architecture Context:**
From Appendix A.3 sequence diagram:
```
User types NL → Dashboard sends POST /api/v1/ai/rebalance → Risk Engine calls Anthropic API
→ gets constraints JSON → validates → runs optimizer → returns proposed trades → Dashboard shows trade list
→ User clicks "Execute All" → Dashboard submits each trade via Gateway REST
```

The endpoint lives in Risk Engine because it needs access to portfolio state and the optimizer. The AI modules (`ai/execution_analyst/`, `ai/rebalancing_assistant/`) are imported as library dependencies — they are NOT separate services. The Risk Engine's `pyproject.toml` or `requirements.txt` will need `anthropic` added.

---

### P4-10: Risk Engine — Execution Report Trigger on Fill Events

**Service:** Risk Engine
**Files:**
- `risk-engine/risk_engine/kafka/consumer.py` (modify — add execution analysis trigger)
**Dependencies:** P4-02 (ExecutionAnalyst), P4-09 (report storage)
**Acceptance Criteria:**
- When the Kafka consumer detects a terminal order state (Filled or Canceled-with-partial-fills), it triggers `ExecutionAnalyst.analyze_execution()` asynchronously
- The trigger builds `TradeContext` from the fill events and portfolio state
- Analysis result is stored in the in-memory report store (from P4-09's `AIDependencies`)
- If `ANTHROPIC_API_KEY` is not set, the trigger is silently skipped (graceful degradation)
- Rate limiting is enforced by the ExecutionAnalyst (max 10/hour)
- Unit tests: mock consumer event triggers analysis; missing API key skips; rate limit respected

**Architecture Context:**
From Section 5.2: "Trigger: Automatically runs when an order reaches terminal state (Filled, Canceled with partial fills). Rate-limited to avoid excessive API calls (max 10 analyses per hour)."

The existing `_on_fill_callback` in `main.py` forwards fills to the settlement tracker. Extend this callback chain to also trigger execution analysis when an order completes. The Kafka consumer already tracks fills by order — detect when all fills for an order are received (order status is terminal).

---

### P4-11: Risk Engine — main.py Wiring for Phase 4 Modules

**Service:** Risk Engine
**Files:**
- `risk-engine/risk_engine/main.py` (modify)
**Dependencies:** P4-06, P4-07, P4-08, P4-09, P4-10
**Acceptance Criteria:**
- Instantiate `StreamingAnomalyDetector` at module level (same pattern as other singletons)
- Instantiate `AnomalyAlertPipeline` in lifespan startup, passing detector + Kafka config
- Start anomaly pipeline in lifespan (after Kafka consumer)
- Stop anomaly pipeline in lifespan shutdown
- Instantiate `ExecutionAnalyst` and `RebalancingAssistant` (if `ANTHROPIC_API_KEY` is set)
- Configure `AnomalyDependencies` and include `anomaly_router` in app
- Configure `AIDependencies` and include `ai_router` in app
- Update health endpoint to report anomaly detector and AI module status
- Add `ANTHROPIC_API_KEY` to environment configuration (optional — AI features degrade gracefully without it)

**Architecture Context:**
Follow the existing wiring pattern in `main.py`:
```python
# Module-level singletons
anomaly_detector = StreamingAnomalyDetector()

# In lifespan:
# Configure anomaly dependencies
anomaly_pipeline = AnomalyAlertPipeline(detector=anomaly_detector, kafka_brokers=KAFKA_BROKERS)
anomaly_deps = AnomalyDependencies(alert_pipeline=anomaly_pipeline)
configure_anomaly_dependencies(anomaly_deps)
anomaly_pipeline.start()

# Configure AI dependencies (optional)
if os.getenv("ANTHROPIC_API_KEY"):
    execution_analyst = ExecutionAnalyst()
    rebalancing_assistant = RebalancingAssistant()
    ai_deps = AIDependencies(execution_analyst=execution_analyst, ...)
    configure_ai_dependencies(ai_deps)

# Include routers
app.include_router(anomaly_router)
app.include_router(ai_router)
```

---

### P4-12: Risk Engine — pyproject.toml Dependencies Update

**Service:** Risk Engine
**Files:**
- `risk-engine/pyproject.toml` (modify)
**Dependencies:** None
**Acceptance Criteria:**
- Add `anthropic>=0.40,<1` (for AI execution analyst and rebalancing assistant)
- Add `scikit-learn>=1.4,<2` (for IsolationForest in anomaly detection)
- Existing dependencies unchanged: fastapi, grpcio, confluent-kafka, numpy, scipy, cvxpy, pandas, structlog, etc.
- Verify no version conflicts

**Architecture Context:**
The Risk Engine imports the AI modules as libraries (not via network calls). Both `anthropic` and `scikit-learn` are runtime dependencies of the Risk Engine process.

---

### P4-13: Kafka Topic — anomaly-alerts

**Service:** Gateway (Kafka producer topic constant)
**Files:**
- `gateway/internal/kafka/producer.go` (modify — add topic constant)
**Dependencies:** None
**Acceptance Criteria:**
- Add `TopicAnomalyAlerts = "anomaly-alerts"` constant alongside existing topic constants
- No new publish method needed in Gateway producer (Risk Engine produces to this topic directly)
- The constant is defined here for consistency and for future Gateway consumption
- Update topic constant tests if they enumerate all topics

**Architecture Context:**
Existing topics: `order-lifecycle`, `market-data`, `venue-status`. The new `anomaly-alerts` topic is produced by the Risk Engine's `AnomalyAlertPipeline` and consumed by the Gateway for WebSocket relay to the dashboard. Defining the constant in the Gateway keeps topic names centralized, but the actual producer is in Risk Engine.

---

### P4-14: Gateway — Anomaly Alert Kafka Consumer + WebSocket Relay

**Service:** Gateway
**Files:**
- `gateway/internal/kafka/anomaly_consumer.go` (create)
- `gateway/internal/kafka/anomaly_consumer_test.go` (create)
**Dependencies:** P4-13 (topic constant), P4-15 (WebSocket anomaly stream)
**Acceptance Criteria:**
- `AnomalyConsumer` struct that subscribes to Kafka topic `anomaly-alerts`
- Consumer group: `"gateway-anomaly-relay"`
- Deserializes JSON alert messages into a Go struct matching the `AnomalyAlert` schema
- On each alert, calls a callback function (to push to WebSocket hub)
- `Start(ctx context.Context)` runs in a goroutine, `Stop()` for graceful shutdown
- Unit tests: mock Kafka consumer, verify deserialization, verify callback invocation

**Architecture Context:**
The Gateway already has a Kafka producer (`gateway/internal/kafka/producer.go`) but no consumer (the Risk Engine consumes order-lifecycle events). This is the first Kafka consumer in the Gateway. Use `confluent_kafka` (Go: `github.com/confluentinc/confluent-kafka-go/v2/kafka`). Follow the same CGO-required pattern as the producer.

The consumer reads alerts published by the Risk Engine and forwards them to the WebSocket hub for real-time dashboard delivery.

---

### P4-15: Gateway — WebSocket Anomaly Alert Stream

**Service:** Gateway
**Files:**
- `gateway/internal/ws/hub.go` (modify — add StreamAnomalies type and broadcast method)
- `gateway/internal/ws/server.go` (modify — add HandleAnomalies endpoint)
- `gateway/internal/ws/server_test.go` (modify — add anomaly stream test)
**Dependencies:** None
**Acceptance Criteria:**
- Add `StreamAnomalies StreamType = "anomalies"` constant
- Add `anomalyData` struct: id, instrument_id, venue_id, anomaly_score, severity, features (map), description, timestamp, acknowledged
- Add `NotifyAnomalyAlert(alert AnomalyAlertEvent)` method to Hub that marshals and broadcasts to `StreamAnomalies`
- Add `AnomalyAlertEvent` struct (input type for the notification)
- Add `HandleAnomalies(w, r)` on Server that upgrades to `/ws/anomalies`
- Unit tests: connect to anomaly stream, receive broadcast alert, verify JSON structure

**Architecture Context:**
Follow the existing pattern in `hub.go`:
```go
const StreamAnomalies StreamType = "anomalies"

type AnomalyAlertEvent struct {
    ID             string
    InstrumentID   string
    VenueID        string
    AnomalyScore   float64
    Severity       string
    Features       map[string]float64
    Description    string
    Timestamp      time.Time
    Acknowledged   bool
}

type anomalyData struct {
    ID           string             `json:"id"`
    InstrumentID string             `json:"instrument_id"`
    VenueID      string             `json:"venue_id"`
    AnomalyScore float64            `json:"anomaly_score"`
    Severity     string             `json:"severity"`
    Features     map[string]float64 `json:"features"`
    Description  string             `json:"description"`
    Timestamp    time.Time          `json:"timestamp"`
    Acknowledged bool               `json:"acknowledged"`
}
```

The WebSocket endpoint is `/ws/anomalies` per architecture Section 5.4.

---

### P4-16: Gateway — main.go Wiring for Anomaly Consumer + WebSocket

**Service:** Gateway
**Files:**
- `gateway/cmd/gateway/main.go` (modify)
**Dependencies:** P4-14 (AnomalyConsumer), P4-15 (WebSocket anomaly stream)
**Acceptance Criteria:**
- Register `/ws/anomalies` route on the HTTP mux: `mux.HandleFunc("/ws/anomalies", wsSrv.HandleAnomalies)`
- Create `AnomalyConsumer` with Kafka broker config and a callback that calls `hub.NotifyAnomalyAlert()`
- Start anomaly consumer in a goroutine during startup
- Stop anomaly consumer during graceful shutdown
- Add `KAFKA_BROKERS` to the Gateway's environment configuration (already exists in docker-compose)

**Architecture Context:**
The existing main.go registers WebSocket routes at lines 327-329:
```go
mux.HandleFunc("/ws/orders", wsSrv.HandleOrders)
mux.HandleFunc("/ws/positions", wsSrv.HandlePositions)
mux.HandleFunc("/ws/venues", wsSrv.HandleVenues)
```
Add the anomaly route in the same block. The consumer is started after Kafka is available (same timing as other Kafka-dependent components).

---

### P4-17: Dashboard — Zustand Insight Store

**Service:** Dashboard
**Files:**
- `dashboard/src/stores/insightStore.ts` (create)
- `dashboard/src/stores/insightStore.test.ts` (create)
**Dependencies:** None
**Acceptance Criteria:**
- `useInsightStore` Zustand store with state:
  - `executionReports: ExecutionReport[]` — sorted by recency
  - `anomalyAlerts: AnomalyAlert[]` — sorted by recency
  - `rebalanceState: { loading: boolean; result: RebalanceResult | null; error: string | null }`
- Actions:
  - `fetchExecutionReports()` — calls `GET /api/v1/ai/execution-reports`
  - `submitRebalancePrompt(prompt: string)` — calls `POST /api/v1/ai/rebalance`, sets loading state
  - `clearRebalanceResult()` — resets rebalance state
  - `applyAnomalyAlert(alert: AnomalyAlert)` — called by WebSocket handler, prepends to list
  - `acknowledgeAlert(alertId: string)` — calls `POST /api/v1/anomalies/{id}/acknowledge`
  - `fetchAnomalyAlerts()` — calls `GET /api/v1/anomalies` for initial load
  - `unacknowledgedCount()` — derived: count of alerts where acknowledged === false
- Unit tests: fetchExecutionReports populates state; submitRebalancePrompt sets loading→result; applyAnomalyAlert prepends; acknowledgeAlert updates flag; unacknowledgedCount computes correctly

**Architecture Context:**
This is the checklist item `dashboard/src/stores/insightStore.ts`. Follow the existing Zustand pattern from `orderStore.ts` and `optimizerStore.ts`.

---

### P4-18: Dashboard — TypeScript Types for AI Features

**Service:** Dashboard
**Files:**
- `dashboard/src/api/types.ts` (modify)
**Dependencies:** None
**Acceptance Criteria:**
- Add `ExecutionReport` interface: overallGrade (string), implementationShortfallBps (number), summary (string), venueAnalysis ({ venue: string, grade: string, comment: string }[]), recommendations (string[]), marketImpactEstimateBps (number), orderId (string), analyzedAt (string)
- Add `AnomalyAlert` interface: id (string), instrumentId (string), venueId (string), anomalyScore (number), severity ("info" | "warning" | "critical"), features (Record<string, number>), description (string), timestamp (string), acknowledged (boolean)
- Add `RebalanceResult` interface: constraints (ExtractedConstraints), optimization (OptimizationResult), reasoning (string)
- Add `ExtractedConstraints` interface: objective (string), targetReturn (number | null), riskAversion (number), longOnly (boolean), maxSingleWeight (number | null), assetClassBounds (Record<string, [number, number]> | null), sectorLimits (Record<string, number> | null), targetVolatility (number | null), maxTurnoverUsd (number | null), instrumentsToInclude (string[] | null), instrumentsToExclude (string[] | null), reasoning (string)
- Add `AnomalyAlertUpdate` WebSocket update type: `{ type: "anomaly_alert"; alert: AnomalyAlert }`

**Architecture Context:**
These types map directly to the JSON responses from the Risk Engine REST endpoints defined in P4-08 and P4-09, and the WebSocket messages from P4-15.

---

### P4-19: Dashboard — REST Client Extensions for AI Endpoints

**Service:** Dashboard
**Files:**
- `dashboard/src/api/rest.ts` (modify)
**Dependencies:** P4-18 (types)
**Acceptance Criteria:**
- Add `fetchExecutionReports(limit?: number): Promise<ExecutionReport[]>` — calls `GET /api/v1/ai/execution-reports?limit=N` on riskApi
- Add `submitRebalancePrompt(prompt: string): Promise<RebalanceResult>` — calls `POST /api/v1/ai/rebalance` on riskApi with `{ prompt }`
- Add `fetchAnomalyAlerts(params?: { limit?: number; severity?: string; instrumentId?: string }): Promise<{ alerts: AnomalyAlert[]; total: number }>` — calls `GET /api/v1/anomalies` on riskApi
- Add `acknowledgeAnomalyAlert(alertId: string): Promise<AnomalyAlert>` — calls `POST /api/v1/anomalies/{alertId}/acknowledge` on riskApi

**Architecture Context:**
All AI and anomaly endpoints are on the Risk Engine (port 8081), so use the existing `riskApi` ky instance. Follow the same pattern as `fetchVaR()`, `optimizePortfolio()`, etc.

---

### P4-20: Dashboard — WebSocket Anomaly Stream Integration

**Service:** Dashboard
**Files:**
- `dashboard/src/api/ws.ts` (modify)
**Dependencies:** P4-18 (AnomalyAlertUpdate type)
**Acceptance Criteria:**
- Add `createAnomalyStream(onAlert: (alert: AnomalyAlert) => void): ReconnectingWebSocket` — connects to `${BASE_WS}/ws/anomalies`
- Update `initializeStreams()` to accept an `onAnomalyAlert` handler and include the anomaly stream in the returned cleanup function
- Follow the same pattern as existing streams (ReconnectingWebSocket, JSON parse, error handling)

**Architecture Context:**
The anomaly WebSocket is on the Gateway (port 8080), same as orders/positions/venues, so use `BASE_WS`. The stream delivers `AnomalyAlertUpdate` messages with `type: "anomaly_alert"`.

---

### P4-21: Dashboard — AI Insights Panel View

**Service:** Dashboard
**Files:**
- `dashboard/src/views/InsightsPanel.tsx` (create)
- `dashboard/src/views/InsightsPanel.test.tsx` (create)
**Dependencies:** P4-17 (insightStore), P4-18 (types), P4-19 (REST), P4-22, P4-23, P4-24
**Acceptance Criteria:**
- Tabbed view with three tabs: "Execution Analysis" | "Rebalancing" | "Anomaly Alerts"
- Purple accent color for AI features (per terminal theme: `accent.purple: "#a855f7"`)
- Tab indicator shows unacknowledged alert count badge on "Anomaly Alerts" tab
- Each tab renders its corresponding sub-component (P4-22, P4-23, P4-24)
- Fetches execution reports and anomaly alerts on mount
- Initializes anomaly WebSocket stream on mount, cleans up on unmount
- Unit tests: renders three tabs; clicking tabs switches content; badge shows unacknowledged count

**Architecture Context:**
From Section 4C:
```
AI Insights Panel (InsightsPanel.tsx):
- Tabbed view: Execution Analysis | Rebalancing | Anomaly Alerts
- Execution Analysis tab: rendered markdown reports from the AI execution analyst, sorted by recency
- Rebalancing tab: natural language input box → loading state → proposed trade list table with "Execute All" button
- Anomaly Alerts tab: timeline of anomaly detections with instrument, venue, score, and feature breakdown
```

Use Radix UI `Tabs` primitive for the tab component. Use Tailwind for styling. Follow the terminal dark theme (`bg-secondary` for card backgrounds, `text-primary` for content).

---

### P4-22: Dashboard — Execution Analysis Tab Component

**Service:** Dashboard
**Files:**
- `dashboard/src/components/ExecutionAnalysisTab.tsx` (create)
- `dashboard/src/components/ExecutionAnalysisTab.test.tsx` (create)
**Dependencies:** P4-17 (insightStore), P4-18 (ExecutionReport type)
**Acceptance Criteria:**
- Renders a list of execution reports sorted by most recent first
- Each report card shows:
  - Letter grade as a large, color-coded badge (A=green, B=blue, C=yellow, D=orange, F=red)
  - Order summary (instrument, side, quantity, venue)
  - Implementation shortfall in bps
  - 2-3 sentence summary
  - Expandable section with venue-by-venue analysis and recommendations
- Empty state: "No execution reports yet. Reports are generated automatically after trades complete."
- Loading state while fetching
- Unit tests: renders reports list; empty state shown when no reports; grade color coding correct

**Architecture Context:**
The execution report JSON structure:
```json
{
  "overall_grade": "B",
  "implementation_shortfall_bps": 3.2,
  "summary": "Solid execution with minor slippage on the secondary venue...",
  "venue_analysis": [
    {"venue": "binance", "grade": "A", "comment": "Best fills at tightest spreads"},
    {"venue": "simulated", "grade": "C", "comment": "Higher slippage due to wider spreads"}
  ],
  "recommendations": ["Consider increasing allocation to Binance for ETH orders", "..."],
  "market_impact_estimate_bps": 1.5
}
```

---

### P4-23: Dashboard — Rebalancing Chat Tab Component

**Service:** Dashboard
**Files:**
- `dashboard/src/components/RebalancingTab.tsx` (create)
- `dashboard/src/components/RebalancingTab.test.tsx` (create)
**Dependencies:** P4-17 (insightStore), P4-18 (RebalanceResult type)
**Acceptance Criteria:**
- Natural language input box at the top with placeholder: "Describe your rebalancing goal..."
- Submit button (or Enter key) triggers `insightStore.submitRebalancePrompt()`
- Loading state: animated skeleton / spinner with "Analyzing your request..."
- Result display:
  - "AI Interpretation" section showing the extracted constraints and reasoning
  - Before/after allocation comparison (current weights vs target weights)
  - Trade list table: instrument, side, quantity, estimated cost (same format as OptimizerView)
  - "Execute All" button that submits each trade via `orderStore.submitOrder()` (same as OptimizerView)
- Error state: shows error message with "Try Again" button
- Clear/reset button to start a new request
- Unit tests: input submits prompt; loading state shown; result renders trade list; Execute All triggers order submissions; error state renders

**Architecture Context:**
From Section 5.3 and Appendix A.3:
1. User types: "Reduce crypto to 30% of portfolio, maximize Sharpe, keep turnover under $5K"
2. Frontend sends `POST /api/v1/ai/rebalance` with `{ "prompt": "..." }`
3. Response includes `{ "constraints": {...}, "optimization": {...trades, weights...}, "reasoning": "..." }`
4. User reviews → clicks "Execute All" → orders submitted via Gateway REST API

The "Execute All" flow reuses the same pattern from `OptimizerView.tsx` / `optimizerStore.ts`: iterate trades, call `submitOrder()` for each.

---

### P4-24: Dashboard — Anomaly Alerts Tab Component

**Service:** Dashboard
**Files:**
- `dashboard/src/components/AnomalyAlertsTab.tsx` (create)
- `dashboard/src/components/AnomalyAlertsTab.test.tsx` (create)
**Dependencies:** P4-17 (insightStore), P4-18 (AnomalyAlert type)
**Acceptance Criteria:**
- Timeline layout: alerts listed vertically with most recent at top
- Each alert card shows:
  - Severity badge: color-coded (info=blue, warning=yellow, critical=red)
  - Instrument and venue
  - Human-readable description (e.g., "ETH-USD volume on Binance 4.2x above 30-day mean")
  - Anomaly score
  - Feature breakdown: small bar chart or key-value list showing which features triggered
  - Timestamp (relative: "2 minutes ago")
  - Acknowledge button (dims the alert when clicked)
- Unacknowledged alerts have a left border accent (severity color)
- Acknowledged alerts are visually muted
- Critical alerts for instruments with open positions show a "Position at Risk" indicator
- Empty state: "No anomalies detected. The system monitors market data 24/7."
- Real-time updates via WebSocket (new alerts appear at top with a brief highlight animation)
- Unit tests: renders alert list; severity colors correct; acknowledge button works; new alert appears via store update

**Architecture Context:**
From Section 5.4:
```python
@dataclass
class AnomalyAlert:
    id: str
    instrument_id: str
    venue_id: str
    anomaly_score: float
    severity: str                 # "info", "warning", "critical"
    features: dict[str, float]    # Which features triggered
    description: str              # Human-readable
    timestamp: datetime
    acknowledged: bool = False
```

Severity thresholds:
- info: anomaly score < -0.3
- warning: anomaly score < -0.5
- critical: anomaly score < -0.7 AND user has open position in affected instrument

---

### P4-25: Dashboard — NL Rebalancing Input → Execute Flow Integration

**Service:** Dashboard
**Files:**
- `dashboard/src/views/InsightsPanel.tsx` (modify — wire up Execute All to order submission)
- `dashboard/src/App.tsx` (modify — add /insights route)
- `dashboard/src/components/TerminalLayout.tsx` (modify — add Insights nav item)
**Dependencies:** P4-21, P4-22, P4-23, P4-24
**Acceptance Criteria:**
- App.tsx: add route `<Route path="insights" element={<InsightsPanel />} />`
- TerminalLayout: add "Insights" nav item with AI icon (Lucide `Brain` or `Sparkles`), positioned after Optimizer in nav order
- Navigation order: Blotter (index), /portfolio, /risk, /venues, /optimizer, /insights
- WebSocket anomaly stream initialized in App.tsx via `initializeStreams()` update (connect on mount, clean up on unmount)
- The anomaly stream feeds into `insightStore.applyAnomalyAlert()`
- Anomaly alert badge visible on the Insights nav item when there are unacknowledged alerts
- Unit tests: /insights route renders InsightsPanel; nav item present; badge shows count

**Architecture Context:**
The existing App.tsx routes (from Phase 3):
```tsx
<Route element={<TerminalLayout />}>
  <Route index element={<BlotterView />} />
  <Route path="portfolio" element={<PortfolioView />} />
  <Route path="risk" element={<RiskDashboard />} />
  <Route path="venues" element={<LiquidityNetwork />} />
  <Route path="optimizer" element={<OptimizerView />} />
</Route>
```

The `initializeStreams()` function in `ws.ts` currently handles orders, positions, risk, and venues. It needs to be extended to also handle anomaly alerts.

---

### P4-26: Dashboard — Anomaly Alert Notification Badges

**Service:** Dashboard
**Files:**
- `dashboard/src/components/TerminalLayout.tsx` (modify — add badge to nav)
- `dashboard/src/components/AlertBadge.tsx` (create)
- `dashboard/src/components/AlertBadge.test.tsx` (create)
**Dependencies:** P4-17 (insightStore.unacknowledgedCount), P4-25 (nav integration)
**Acceptance Criteria:**
- `AlertBadge` component: renders a small circular badge with a number (unacknowledged alert count)
  - Hidden when count is 0
  - Red background for critical alerts, yellow for warnings, blue for info
  - Highest severity among unacknowledged alerts determines badge color
- Badge displayed on the "Insights" nav item in TerminalLayout
- Optional: toast notification when a critical alert arrives (using a simple toast component or browser Notification API)
- Unit tests: badge hidden when count=0; shows correct count; color matches highest severity

**Architecture Context:**
The terminal theme accent colors: red (#ef4444) for critical, yellow (#eab308) for warning, blue (#3b82f6) for info. Purple (#a855f7) is reserved for AI features broadly. The badge should be positioned as a superscript dot/count on the nav icon.

---

### P4-27: Docker Compose — Phase 4 Updates

**Service:** Infrastructure
**Files:**
- `deploy/docker-compose.yml` (modify)
**Dependencies:** P4-11 (Risk Engine wiring), P4-16 (Gateway wiring)
**Acceptance Criteria:**
- Add `ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}` to risk-engine environment (optional — AI features degrade gracefully)
- Add `KAFKA_BROKERS=kafka:9092` to gateway environment if not already present (it is — verify)
- Verify the anomaly-alerts Kafka topic will auto-create (Kafka default `auto.create.topics.enable=true`) or add a topic creation init step
- No new services needed — all Phase 4 additions are within existing services (Risk Engine, Gateway, Dashboard)
- Update risk-engine healthcheck to optionally report AI and anomaly module status

**Architecture Context:**
Current docker-compose services: gateway, dashboard, postgres, redis, kafka, ml-scorer, risk-engine. Phase 4 adds no new containers — the execution analyst, rebalancing assistant, and anomaly detector all run within the Risk Engine process.

---

## Dependency Graph

```
Independent (can start immediately):
  P4-01, P4-03, P4-05, P4-06, P4-13, P4-15, P4-18

Wave 2 (after Wave 1):
  P4-02 (← P4-01)
  P4-04 (← P4-03)
  P4-07 (← P4-06)
  P4-12

Wave 3 (after Wave 2):
  P4-08 (← P4-07)
  P4-09 (← P4-04, P4-03)
  P4-14 (← P4-13, P4-15)
  P4-17, P4-19 (← P4-18)
  P4-20 (← P4-18)

Wave 4 (after Wave 3):
  P4-10 (← P4-02, P4-09)
  P4-11 (← P4-06, P4-07, P4-08, P4-09, P4-10)
  P4-16 (← P4-14, P4-15)
  P4-22, P4-23, P4-24 (← P4-17, P4-18)

Wave 5 (after Wave 4):
  P4-21 (← P4-17, P4-22, P4-23, P4-24)
  P4-25 (← P4-21)
  P4-26 (← P4-17, P4-25)
  P4-27 (← P4-11, P4-16)
```

## Checklist Cross-Reference

All 9 roadmap deliverables from architecture doc Section 8 Phase 4 are covered:

| Deliverable | Task(s) |
|------------|---------|
| AI: Execution analyst (Anthropic API integration, structured prompt, response parsing) | P4-01, P4-02 |
| AI: Rebalancing assistant (NL → optimizer constraints via Anthropic API) | P4-03, P4-04 |
| AI: Anomaly detection (Isolation Forest on streaming market data features) | P4-06 |
| Risk Engine: Anomaly detection integration (Kafka consumer, alert pipeline) | P4-07, P4-08 |
| Dashboard: AI Insights panel (execution reports, rebalancing chat, anomaly timeline) | P4-21, P4-22, P4-23, P4-24 |
| Dashboard: NL rebalancing input → loading → trade list → execute flow | P4-23, P4-25 |
| Dashboard: Anomaly alert badges and notification system | P4-26 |
| Gateway: Anomaly alerts via WebSocket | P4-14, P4-15, P4-16 |
| Kafka topic: anomaly-alerts | P4-13 |

Unchecked deliverables-checklist items addressed by Phase 4:

| Checklist Item | Task |
|---------------|------|
| `risk_engine/anomaly/detector.py` | P4-06 |
| `dashboard/src/stores/insightStore.ts` | P4-17 |
| `dashboard/src/views/InsightsPanel.tsx` | P4-21 |
| `ai/execution_analyst/analyst.py` | P4-02 |
| `ai/execution_analyst/prompt_templates.py` | P4-01 |
| `ai/rebalancing_assistant/assistant.py` | P4-04 |
| `ai/rebalancing_assistant/prompt_templates.py` | P4-03 |
| `risk_engine/anomaly/detector_test.py` | P4-06 |
| `risk_engine/tests/test_anomaly.py` | P4-07 |
| `ai/execution_analyst/analyst_test.py` | P4-02 |
| `ai/rebalancing_assistant/assistant_test.py` | P4-04 |

Unchecked items NOT in Phase 4 scope (deferred to Phase 5):
- `gateway/internal/orderbook/book.go` — Phase 4/5 per checklist note
- `gateway/internal/adapter/tokenized/adapter.go` — Phase 5 deliverable
- `risk_engine/timeseries/regime.py` — Not in any phase roadmap; future enhancement
- `risk_engine/rest/router_scenario.py` — Not in Phase 4 roadmap; future enhancement
- `risk_engine/requirements.lock` — Phase 5 hardening
- `dashboard/src/components/CandlestickChart.tsx` — Not in Phase 4 roadmap
- All `deploy/k8s/`, `deploy/grafana/`, `deploy/prometheus.yml` — Phase 5
- All `loadtest/` — Phase 5
- All documentation (README, quickstart, guides, LICENSE, CONTRIBUTING) — Phase 5
- All CI/CD workflows — Phase 5
- All E2E Playwright tests — Phase 5
- `deploy/docker-compose.dev.yml` — Phase 5
- `scripts/health-check.sh` — Phase 5
- `gateway/internal/pipeline/pipeline_bench_test.go` — Phase 5

---

## Task Completion Status

| Task | Status | Notes |
|------|--------|-------|
| P4-01 | ✅ COMPLETE | TradeContext, ExecutionReport types, EXECUTION_ANALYSIS_PROMPT — 6 tests |
| P4-02 | ✅ COMPLETE | ExecutionAnalyst with Anthropic API, rate limiting, fallback — 5 tests |
| P4-03 | ✅ COMPLETE | RebalanceRequest, ExtractedConstraints, CONSTRAINT_EXTRACTION_PROMPT — 6 tests |
| P4-04 | ✅ COMPLETE | RebalancingAssistant with Anthropic API, validation — 4 tests |
| P4-05 | ✅ COMPLETE | Added anthropic>=0.40,<1 and scikit-learn>=1.4,<2 to ai/requirements.txt |
| P4-06 | ✅ COMPLETE | StreamingAnomalyDetector with IsolationForest, AnomalyAlert — 11 tests |
| P4-07 | ✅ COMPLETE | AnomalyAlertPipeline with Kafka consumer/producer — 7 tests |
| P4-08 | ✅ COMPLETE | GET /api/v1/anomalies, POST acknowledge — 7 tests |
| P4-09 | ✅ COMPLETE | POST /ai/rebalance, POST /ai/execution-report, GET /ai/execution-reports — 5 tests |
| P4-10 | ✅ COMPLETE | on_order_complete callback on terminal fill events — 4 tests |
| P4-11 | ✅ COMPLETE | main.py wires anomaly detector, pipeline, AI modules, routers, health |
| P4-12 | ✅ COMPLETE | Added anthropic>=0.40,<1 to risk-engine/pyproject.toml |
| P4-13 | ✅ COMPLETE | TopicAnomalyAlerts = "anomaly-alerts" constant |
| P4-14 | ✅ COMPLETE | AnomalyConsumer Go struct for Kafka→WebSocket relay |
| P4-15 | ✅ COMPLETE | StreamAnomalies type, NotifyAnomalyAlert, HandleAnomalies — 5 tests |
| P4-16 | ✅ COMPLETE | main.go: /ws/anomalies route, anomaly consumer start/stop |
| P4-17 | ✅ COMPLETE | useInsightStore Zustand store — 5 tests |
| P4-18 | ✅ COMPLETE | ExecutionReport, AnomalyAlert, ExtractedConstraints, RebalanceResult TS types |
| P4-19 | ✅ COMPLETE | fetchExecutionReports, submitRebalancePrompt, fetchAnomalyAlerts, acknowledgeAnomalyAlert |
| P4-20 | ✅ COMPLETE | createAnomalyStream, initializeStreams updated with onAnomalyAlert |
| P4-21 | ✅ COMPLETE | InsightsPanel tabbed view with 3 tabs — 3 tests |
| P4-22 | ✅ COMPLETE | ExecutionAnalysisTab with grade badges, expandable detail — 4 tests |
| P4-23 | ✅ COMPLETE | RebalancingTab with NL input, trade list, Execute All — 5 tests |
| P4-24 | ✅ COMPLETE | AnomalyAlertsTab with severity colors, acknowledge — 5 tests |
| P4-25 | ✅ COMPLETE | /insights route in App.tsx, Insights nav in TerminalLayout, anomaly WS init |
| P4-26 | ✅ COMPLETE | AlertBadge component, nav badge on Insights tab — 3 tests |
| P4-27 | ✅ COMPLETE | ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY} in docker-compose risk-engine env |

**Summary:** 27 of 27 tasks complete, 0 modified, 0 deferred.

## Phase 4 Deviations

### Deviation 1: Anomaly Consumer Placement in Gateway main.go
**Architecture Doc Says:** Initialize anomaly consumer after Kafka producer (section 7).
**Actual Implementation:** Initialized after WebSocket hub creation (section 10b) because the callback closure references `hub` which must be in scope.
**Reason:** Forward reference compile error — `hub` is declared after section 7.
**Impact:** None — functionally identical, consumer still starts before HTTP server.

### Deviation 2: RebalancingAssistant.extract_constraints Is Synchronous
**Architecture Doc Says:** `async def extract_constraints(...)` with `await` in the router.
**Actual Implementation:** Synchronous method (Anthropic Python SDK's `messages.create` is synchronous). Router calls it without `await`.
**Reason:** The `anthropic` SDK provides synchronous and async clients; the implementation uses the synchronous client matching the ExecutionAnalyst pattern.
**Impact:** Low — blocking call runs in FastAPI's thread pool via default async-to-sync bridge. Could be upgraded to `AsyncAnthropic` if latency becomes an issue.

### Deviation 3: ai/__init__.py Created
**Architecture Doc Says:** No `ai/__init__.py` in directory listing.
**Actual Implementation:** Created `ai/__init__.py` to make `ai/` a proper Python package for test imports.
**Reason:** Without it, `from ai.execution_analyst.types import ...` fails with ImportError.
**Impact:** None — empty file, required for standard Python packaging.

### Deviation 4: Execution Analyst Tests Alongside Package
**Architecture Doc Says:** `ai/execution_analyst/analyst_test.py` (flat alongside module).
**Actual Implementation:** `ai/execution_analyst/tests/test_analyst.py` (tests subdirectory with pytest convention).
**Reason:** Consistent with Phase 3's ml_scorer test layout and pytest discovery conventions.
**Impact:** None — tests are discoverable and pass.
