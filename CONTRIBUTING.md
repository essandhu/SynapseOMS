# Contributing to SynapseOMS

Thanks for your interest in contributing. This document covers how to set up your development environment, run tests, and submit changes.

## Development Environment

### Prerequisites

- Docker and Docker Compose
- Go 1.25+
- Python 3.12+
- Node.js 20+

### Setup

```bash
# Clone the repo
git clone https://github.com/essandhu/SynapseOMS.git
cd SynapseOMS

# Start infrastructure services
cp deploy/.env.example deploy/.env
# Edit deploy/.env and set SYNAPSE_MASTER_PASSPHRASE
docker compose -f deploy/docker-compose.yml up postgres redis kafka -d

# Gateway (Go)
cd gateway
go mod download
go test ./...

# Risk Engine (Python)
cd ../risk-engine
pip install -e ".[dev]"
pytest tests/

# Dashboard (TypeScript)
cd ../dashboard
npm install
npx vitest run

# AI Modules (Python)
cd ../ai
pip install -r requirements.txt
PYTHONPATH=. pytest execution_analyst/tests/ rebalancing_assistant/tests/ smart_router_ml/tests/
```

### Dev Mode with Hot Reload

```bash
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml up
```

This mounts source directories for live reloading and exposes debug ports:
- Gateway (Delve): port 2345
- Risk Engine (debugpy): port 5678

## Code Style

| Service | Formatter | Linter |
|---------|-----------|--------|
| Gateway (Go) | `gofmt` | `go vet`, `staticcheck` |
| Risk Engine (Python) | `ruff format` | `ruff check` |
| Dashboard (TypeScript) | `prettier` | `eslint` |

### Go Conventions

- Structured JSON logging with `slog`
- Early return pattern, max 3 levels of nesting
- Typed errors, not string errors
- Functions under 50 lines, files under 200 lines

### Python Conventions

- Type annotations on all public functions
- `structlog` for JSON logging with correlation IDs
- Dataclasses for domain types

### TypeScript Conventions

- Zustand for state management
- Tailwind CSS for styling
- Vitest + React Testing Library for tests

## Running Tests

```bash
# All Gateway tests
cd gateway && go test ./...

# All Risk Engine tests (from repo root)
PYTHONPATH=. python -m pytest risk-engine/tests/ -q

# All Dashboard tests
cd dashboard && npx vitest run

# All AI tests (from repo root)
PYTHONPATH=ai python -m pytest ai/execution_analyst/tests/ ai/rebalancing_assistant/tests/ ai/smart_router_ml/tests/
```

## PR Process

1. **Fork** the repository
2. **Branch** from `main`: `git checkout -b feature/my-feature`
3. **Implement** with tests (TDD encouraged)
4. **Test** — ensure all existing tests still pass
5. **Commit** with a clear message: `feat(gateway): add Kraken adapter`
6. **Push** and open a Pull Request against `main`
7. **Review** — address feedback, keep the PR focused

### Commit Message Convention

```
type(scope): short description

type: feat, fix, refactor, test, docs, chore
scope: gateway, risk-engine, dashboard, ai, deploy, docs
```

## Primary Contribution Path: Venue Adapters

The highest-impact contribution is adding support for new exchanges. See [docs/write-adapter.md](docs/write-adapter.md) for a step-by-step guide.

Each adapter:
- Implements the `LiquidityProvider` interface (16 methods)
- Passes the shared contract test suite
- Includes venue-specific unit tests

## Issue Labels

| Label | Meaning |
|-------|---------|
| `good first issue` | Suitable for newcomers |
| `adapter` | Exchange adapter work |
| `bug` | Something broken |
| `enhancement` | Feature request |
| `risk-engine` | Risk analytics work |
| `dashboard` | Frontend work |

## Code of Conduct

Be respectful, constructive, and inclusive. We're building tools for traders — focus on shipping quality software. Harassment, discrimination, or personal attacks are not tolerated.

## Questions?

Open a GitHub Issue or Discussion. For adapter-specific questions, tag the issue with the `adapter` label.
