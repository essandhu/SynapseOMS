/**
 * capture.ts — regenerate the README screenshots and demo GIF.
 *
 * Prerequisites:
 *   1. Stack running:   docker compose -f deploy/docker-compose.yml up -d
 *   2. Playwright:      cd e2e && npm ci && npx playwright install chromium
 *
 * Run from the repo root (Node >= 23, uses built-in TS type stripping):
 *   node docs/media/capture.ts
 *
 * Environment:
 *   BASE_URL            dashboard URL (default http://localhost:3000)
 *   DEMO_PASSPHRASE     onboarding passphrase (default DemoPassphrase123!)
 *
 * Output (this directory):
 *   blotter-dark.png, blotter-light.png, portfolio.png, risk.png,
 *   venues.png, insights.png, order-flow.gif
 */
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

// Playwright is installed in e2e/, not at the repo root — resolve from there.
const requireFromE2E = createRequire(
  new URL("../../e2e/package.json", import.meta.url),
);
const { chromium } = requireFromE2E("playwright");

const MEDIA_DIR = path.dirname(fileURLToPath(import.meta.url));
const BASE_URL = process.env.BASE_URL || "http://localhost:3000";
const PASSPHRASE = process.env.DEMO_PASSPHRASE || "DemoPassphrase123!";

// Buys that seed a realistic cross-asset book (simulated venue fills
// instantly). Sizes stay under the risk engine's 25%-of-NAV concentration
// limit so every order passes the pre-trade check.
const SEED_ORDERS: Array<[symbol: string, qty: string]> = [
  ["AAPL", "25"],
  ["MSFT", "15"],
  ["GOOG", "10"],
  ["BTC-USD", "0.15"],
  ["ETH-USD", "2"],
];

async function waitForReady(url: string, timeoutMs = 180_000) {
  const deadline = Date.now() + timeoutMs;
  let lastError = "";
  while (Date.now() < deadline) {
    try {
      const res = await fetch(url);
      if (res.ok) return;
      lastError = `HTTP ${res.status}`;
    } catch (err) {
      lastError = String(err);
    }
    await sleep(2000);
  }
  throw new Error(`Timed out waiting for ${url} (${lastError})`);
}

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// Mirrors e2e/helpers/onboarding.ts — first run walks the wizard with the
// built-in simulated exchange; subsequent runs land on the blotter directly.
async function completeOnboarding(page: any) {
  await page.goto(BASE_URL + "/");
  const getStarted = page.getByText("Get Started");
  const submitOrder = page.getByText("Submit Order");
  await getStarted.or(submitOrder).waitFor({ timeout: 30_000 });
  if (await submitOrder.isVisible()) return;

  await getStarted.click();
  const passwordInputs = page.locator('input[type="password"]');
  await passwordInputs.nth(0).fill(PASSPHRASE);
  await passwordInputs.nth(1).fill(PASSPHRASE);
  await page.getByText("Set Passphrase").click();
  await page.getByText("Start with Simulator").click();
  await page.getByText("Skip to Finish").click();
  await page.getByText("Open Trading Terminal").click();
  await submitOrder.waitFor({ timeout: 15_000 });
}

async function newestOrder(symbol: string): Promise<any | undefined> {
  const res = await fetch(`${BASE_URL}/api/v1/orders?limit=5`);
  const orders = await res.json();
  return orders.find((o: any) => o.instrument_id === symbol);
}

async function marketBuy(page: any, symbol: string, qty: string) {
  for (let attempt = 1; attempt <= 3; attempt++) {
    await page.locator("#instrument-select").selectOption(symbol);
    // Pin the venue: Smart Route may pick a disconnected venue on a fresh
    // install, and a rejected order would poison the screenshots.
    const venue = page.locator("#venue-select");
    await venue.selectOption("sim-exchange");
    // Venue-status stream updates can re-render the ticket and reset the
    // select back to Smart Route — verify it stuck before submitting.
    await sleep(600);
    if ((await venue.inputValue()) !== "sim-exchange") {
      await venue.selectOption("sim-exchange");
      await sleep(400);
    }
    await page.getByRole("button", { name: "Buy" }).click();
    const qtyInput = page.locator('input[placeholder="0"]').first();
    await qtyInput.clear();
    await qtyInput.fill(qty);
    const before = await newestOrder(symbol).catch(() => undefined);
    await page.getByText("Submit Order").click();
    try {
      await waitForNewestFill(symbol, 20_000, before?.id);
      return;
    } catch (err) {
      if (attempt === 3) throw err;
      console.warn(`  ${symbol} attempt ${attempt} failed (${err}) — retrying`);
      await sleep(1500);
    }
  }
}

/** Poll the orders API until the newest order for `symbol` reports filled. */
async function waitForNewestFill(
  symbol: string,
  timeoutMs = 20_000,
  ignoreId?: string,
) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const newest = await newestOrder(symbol);
      if (newest && newest.id !== ignoreId) {
        if (newest.status === "rejected") {
          throw new Error(`${symbol} order was rejected by ${newest.venue_id}`);
        }
        if (newest.status === "filled") return;
      }
    } catch (err) {
      if (String(err).includes("rejected")) throw err;
    }
    await sleep(500);
  }
  throw new Error(`Timed out waiting for ${symbol} fill`);
}

async function shoot(page: any, name: string, settleMs = 2500) {
  await sleep(settleMs); // let charts/grids finish animating
  const file = path.join(MEDIA_DIR, name);
  await page.screenshot({ path: file });
  console.log(`  captured ${name}`);
}

function supportsGif(exe: string): boolean {
  const probe = spawnSync(exe, ["-hide_banner", "-muxers"], {
    encoding: "utf8",
  });
  return probe.status === 0 && /\bgif\b/.test(probe.stdout || "");
}

function findFfmpeg(): string | null {
  // Prefer a full ffmpeg from PATH — Playwright's bundled build lacks the
  // GIF muxer, so it is only a fallback and must be probed first.
  if (supportsGif("ffmpeg")) return "ffmpeg";
  try {
    const { registry } = requireFromE2E(
      "playwright-core/lib/server/registry/index",
    );
    const exe = registry.findExecutable("ffmpeg").executablePath("javascript");
    if (exe && fs.existsSync(exe) && supportsGif(exe)) return exe;
  } catch {
    /* no bundled ffmpeg either */
  }
  return null;
}

function wantView(name: string): boolean {
  const idx = process.argv.indexOf("--views");
  if (idx === -1) return true; // no filter → capture everything
  return (process.argv[idx + 1] || "").split(",").includes(name);
}

async function captureScreenshots(browser: any) {
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
  });
  const page = await context.newPage();

  console.log("Onboarding with the simulated exchange...");
  await completeOnboarding(page);

  // Market orders fill instantly, so the default Active-only filter would
  // hide them — switch to All before seeding so fills are observable.
  await page.getByText("All", { exact: true }).click();

  if (!process.argv.includes("--no-seed")) {
    console.log("Seeding a cross-asset portfolio...");
    for (const [symbol, qty] of SEED_ORDERS) {
      await marketBuy(page, symbol, qty);
      console.log(`  filled ${symbol} x ${qty}`);
    }
  }

  console.log("Capturing views...");
  if (wantView("blotter")) {
    // The chart panel stays hidden: candles build from the live tick stream
    // only while the page is open, so a fresh session has too few bars to
    // showcase. The Filled tab is the cleanest hero: real orders, no noise.
    await page.getByText("Filled", { exact: true }).first().click();
    await page.locator(".ag-center-cols-container .ag-row").first().waitFor({
      timeout: 15_000,
    });
    // Dark is the product default — capture the hero in both themes.
    await shoot(page, "blotter-dark.png", 4000);

    await page.getByRole("button", { name: "Switch to light mode" }).click();
    await shoot(page, "blotter-light.png", 1500);
    // Remaining views are captured in light mode.
  }

  if (wantView("portfolio")) {
    await page.getByRole("link", { name: "Portfolio" }).click();
    await page.getByText("Positions").first().waitFor({ timeout: 15_000 });
    await shoot(page, "portfolio.png");
  }

  if (wantView("risk")) {
    await page.getByRole("link", { name: "Risk" }).click();
    await page.getByText(/Value at Risk|VaR/i).first().waitFor({ timeout: 20_000 });
    await shoot(page, "risk.png", 4000);
    // The analytics computed from live positions sit below the fold.
    await page.getByText(/drawdown/i).first().scrollIntoViewIfNeeded();
    await page.mouse.wheel(0, 400);
    await shoot(page, "risk-analytics.png", 2000);
  }

  if (wantView("venues")) {
    await page.getByRole("link", { name: "Venues" }).click();
    await shoot(page, "venues.png");
  }

  if (wantView("insights")) {
    await page.getByRole("link", { name: "Insights" }).click();
    await shoot(page, "insights.png");
  }

  await context.close();
}

async function captureOrderFlowGif(browser: any) {
  const ffmpeg = findFfmpeg();
  if (!ffmpeg) {
    console.warn(
      "! ffmpeg not found — skipping order-flow.gif. Install ffmpeg and re-run.",
    );
    return;
  }

  console.log("Recording order flow video...");
  const videoDir = fs.mkdtempSync(path.join(MEDIA_DIR, ".video-"));
  // Fresh context → dark theme default, matching the product's first-run look.
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    recordVideo: { dir: videoDir, size: { width: 1280, height: 800 } },
  });
  const page = await context.newPage();

  await page.goto(BASE_URL + "/");
  await page.getByText("Submit Order").waitFor({ timeout: 30_000 });
  // Show the Filled tab so the viewer sees the new order land as a fill.
  await page.getByText("Filled", { exact: true }).first().click();
  await sleep(2500);

  await page.locator("#instrument-select").selectOption("SOL-USD");
  await sleep(1000);
  await page.locator("#venue-select").selectOption("sim-exchange");
  await sleep(800);
  if ((await page.locator("#venue-select").inputValue()) !== "sim-exchange") {
    await page.locator("#venue-select").selectOption("sim-exchange");
    await sleep(400);
  }
  await page.getByRole("button", { name: "Buy" }).click();
  const qtyInput = page.locator('input[placeholder="0"]').first();
  await qtyInput.clear();
  await qtyInput.pressSequentially("20", { delay: 150 });
  await sleep(800);
  const before = await newestOrder("SOL-USD").catch(() => undefined);
  await page.getByText("Submit Order").click();
  await waitForNewestFill("SOL-USD", 30_000, before?.id);
  await sleep(3500); // show the fill + position update streaming in

  await context.close(); // flushes the video file

  const webm = fs
    .readdirSync(videoDir)
    .map((f) => path.join(videoDir, f))
    .find((f) => f.endsWith(".webm"));
  if (!webm) throw new Error("Playwright produced no video file");

  console.log("Converting to GIF...");
  const gif = path.join(MEDIA_DIR, "order-flow.gif");
  const result = spawnSync(ffmpeg, [
    "-y",
    "-i", webm,
    "-vf",
    "fps=10,scale=960:-1:flags=lanczos,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse",
    "-loop", "0",
    gif,
  ]);
  fs.rmSync(videoDir, { recursive: true, force: true });
  if (result.status !== 0) {
    throw new Error(`ffmpeg failed: ${result.stderr?.toString().slice(-500)}`);
  }
  const mb = fs.statSync(gif).size / (1024 * 1024);
  console.log(`  captured order-flow.gif (${mb.toFixed(1)} MB)`);
}

console.log(`Waiting for SynapseOMS at ${BASE_URL} ...`);
await waitForReady(`${BASE_URL}/api/v1/health`);
await waitForReady(BASE_URL);

const gifOnly = process.argv.includes("--gif-only");
const browser = await chromium.launch();
try {
  if (!gifOnly) await captureScreenshots(browser);
  await captureOrderFlowGif(browser);
} finally {
  await browser.close();
}
console.log(`Done. Media written to ${MEDIA_DIR}`);
