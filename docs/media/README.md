# README media

All screenshots and the demo GIF are captured from a real running stack (built-in
simulated exchange, no external API keys) by [`capture.ts`](capture.ts).

## Regenerate

```bash
# 1. Start the stack
docker compose -f deploy/docker-compose.yml up -d

# 2. Install Playwright (once)
cd e2e && npm ci && npx playwright install chromium && cd ..

# 3. Capture (Node >= 23)
node docs/media/capture.ts
```

The script onboards with the simulated exchange, submits a handful of equity and
crypto orders to build a realistic cross-asset portfolio (verifying each fill via
the orders API), then captures each dashboard view at 1280×800 (2x scale). The
order-flow GIF is recorded as video and converted with `ffmpeg` from PATH
(Playwright's bundled ffmpeg lacks the GIF muxer, so it is only probed as a
fallback). Useful flags: `--gif-only`, `--no-seed`, `--views blotter,portfolio`.

| File | Content |
|------|---------|
| `blotter-dark.png` / `blotter-light.png` | Order blotter hero (both themes) |
| `portfolio.png` | Portfolio view — positions, NAV, exposure |
| `risk-analytics.png` | Greeks heatmap + concentration treemap |
| `venues.png` | Liquidity network — venue status |
| `order-flow.gif` | Submitting a market order through to fill |

The script also writes `risk.png` (VaR gauges) and `insights.png` (AI panel);
these are not committed because a fresh install has no VaR return history and
the AI panel needs `ANTHROPIC_API_KEY` — both render as empty states.
