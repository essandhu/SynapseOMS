# Phase 8 Tasks — Final Audit Remediation

**Goal:** Address all remaining Priority 2 and Priority 3 items from the 2026-04-03 final audit: fix the `test_rest_ai.py` import error, add missing `orderStore` and `positionStore` unit tests, and clean up the stale duplicate `train_test.py` file.

**Acceptance Test:** All risk-engine Python tests pass including `test_rest_ai.py` (no import errors). Dashboard Vitest suite passes with new `orderStore.test.ts` and `positionStore.test.ts` files covering core store behavior. The stale `ai/smart_router_ml/train_test.py` duplicate is removed.

**Architecture Doc References:** Section 4.3 (test coverage), Appendix A (deliverables checklist)

**Previous Phase Review:** Phase 7 passed all 10 tasks. Final audit identified 4 remaining items: (1) `test_rest_ai.py` import error (P2), (2) missing orderStore + positionStore tests (P2), (3) venue-status topic consumer (P3 — acknowledged as fire-and-forget, no action needed), (4) stale duplicate test file (P3).

---

## Tasks

### P8-01: ✅ COMPLETE — Fix `test_rest_ai.py` Import Error

**Service:** Risk Engine
**Files:**
- `risk-engine/tests/conftest.py` (modify — add `ai/` parent directory to sys.path)
**Dependencies:** None
**Acceptance Criteria:**
- `from ai.execution_analyst.types import ExecutionReport, TradeContext` resolves correctly when running pytest from `risk-engine/`
- All 5 tests in `test_rest_ai.py` pass
- No other risk-engine tests are broken by the change
- Fix uses `sys.path.insert` in `conftest.py` to add the project root (parent of both `ai/` and `risk-engine/`)

**Architecture Context:**
The `test_rest_ai.py` file imports from `ai.execution_analyst.types` and `ai.rebalancing_assistant.types`, but `ai/` is a sibling directory to `risk-engine/`, not a sub-package. When pytest runs from `risk-engine/`, the `ai/` package is not on `PYTHONPATH`. Fix: add `sys.path.insert(0, str(Path(__file__).resolve().parent.parent.parent))` at the top of `conftest.py` to include the project root directory.

---

### P8-02: ✅ COMPLETE — Add `orderStore.test.ts` Unit Tests

**Service:** Dashboard
**Files:**
- `dashboard/src/stores/orderStore.test.ts` (create)
**Dependencies:** None
**Acceptance Criteria:**
- Tests cover: initial empty state, `loadOrders` success and error paths, `submitOrder` success and error paths, `cancelOrder` success and error paths, `applyUpdate` applying a WebSocket order update, `activeOrders` filtering, and `subscribe` (WebSocket connection + cleanup)
- Follow the same pattern as `venueStore.test.ts` (mock `../api/rest` and `../api/ws`, use `vi.mock`, reset state in `beforeEach`)
- All tests pass via `npx vitest run`

**Architecture Context:**
`orderStore.ts` is a Zustand store with: `orders` Map, `loading`, `error`, `activeOrders()` getter, `submitOrder()`, `cancelOrder()`, `applyUpdate()`, `loadOrders()`, `subscribe()`. REST functions to mock: `submitOrder` (aliased as `apiSubmitOrder`), `cancelOrder` (aliased as `apiCancelOrder`), `fetchOrders`. WS function to mock: `createOrderStream`. Types: `Order`, `OrderUpdate`, `SubmitOrderRequest` from `../api/types`.

---

### P8-03: ✅ COMPLETE — Add `positionStore.test.ts` Unit Tests

**Service:** Dashboard
**Files:**
- `dashboard/src/stores/positionStore.test.ts` (create)
**Dependencies:** None
**Acceptance Criteria:**
- Tests cover: initial empty state, `loadPositions` success and error paths, `applyUpdate` applying a WebSocket position update with correct key (`instrumentId:venueId`), and `subscribe` (WebSocket connection + cleanup)
- Follow the same pattern as `venueStore.test.ts`
- All tests pass via `npx vitest run`

**Architecture Context:**
`positionStore.ts` is a Zustand store with: `positions` Map (keyed by `instrumentId:venueId`), `loading`, `error`, `applyUpdate()`, `loadPositions()`, `subscribe()`. REST function to mock: `fetchPositions`. WS function to mock: `createPositionStream`. The `positionKey` helper is a module-private function — test its behavior indirectly through `applyUpdate` and `loadPositions`.

---

### P8-04: ✅ COMPLETE — Remove Stale Duplicate `ai/smart_router_ml/train_test.py`

**Service:** AI
**Files:**
- `ai/smart_router_ml/train_test.py` (delete)
**Dependencies:** None
**Acceptance Criteria:**
- The file `ai/smart_router_ml/train_test.py` is deleted
- The canonical test file `ai/smart_router_ml/tests/test_train.py` still exists and passes
- No other test files reference or import from the deleted file

---

## Phase 8 Deviations

### Deviation 1: P8-04 moved test file instead of just deleting
**Architecture Doc Says:** Delete `ai/smart_router_ml/train_test.py` (stale duplicate)
**Actual Implementation:** Moved the tracked `train_test.py` to `tests/test_train.py` (the canonical location), because the "canonical" `tests/test_train.py` was never committed to git — it only existed as an untracked file in the main working directory. The `tests/test_train.py` version is a newer, more complete rewrite with fixtures, additional test cases (`parse_args`, `rmse_history`, `creates_parent_directories`), and updated imports.
**Reason:** Simply deleting `train_test.py` would have left the module with no committed training tests. Moving it ensures the tests are properly tracked in git.
**Impact:** None — the canonical test location `tests/test_train.py` now has a committed file, consistent with the convention used by `tests/test_features.py` and `tests/test_model.py`.
