# Connect Your First Exchange

SynapseOMS supports multiple venues out of the box. This guide walks you through connecting Alpaca (US equities, paper trading) and Binance Testnet (crypto).

## Alpaca — Paper Trading (Equities)

Alpaca offers free paper trading accounts with real-time market data.

### 1. Create an Account

1. Go to [alpaca.markets](https://alpaca.markets/) and sign up
2. Switch to **Paper Trading** mode (toggle in the dashboard header)

### 2. Get Your API Credentials

1. Navigate to **Paper Trading** > **API Keys**
2. Click **Generate New Key**
3. Copy both the **API Key ID** and **Secret Key** — the secret is shown only once

### 3. Connect in SynapseOMS

1. Open the SynapseOMS dashboard at `http://localhost:3000`
2. If you're in the onboarding flow, select **Alpaca** as your venue
3. Enter your API Key ID and Secret Key in the credential form
4. Click **Connect** — the system will verify your credentials with a test API call
5. On success, you'll see the Alpaca venue card turn green in the Liquidity Network view

### What You Can Trade

- US equities (AAPL, MSFT, GOOG, etc.)
- Settlement: T+2 (standard equities settlement)
- Market hours: 9:30 AM - 4:00 PM ET (pre-market 4:00 AM, after-hours until 8:00 PM)

---

## Binance Testnet — Paper Trading (Crypto)

Binance Testnet provides a risk-free environment with testnet tokens.

### 1. Create a Testnet Account

1. Go to [testnet.binance.vision](https://testnet.binance.vision/)
2. Log in with a GitHub account

### 2. Get Your API Credentials

1. After logging in, click **Generate HMAC_SHA256 Key**
2. Give it a label (e.g., "SynapseOMS")
3. Copy both the **API Key** and **Secret Key**

### 3. Connect in SynapseOMS

1. Open the SynapseOMS dashboard at `http://localhost:3000`
2. Navigate to **Liquidity Network** > **Connect New Venue**
3. Select **Binance Testnet**
4. Enter your API Key and Secret Key
5. Click **Connect** — the system will ping the Binance testnet API to verify

### What You Can Trade

- Crypto pairs: BTC-USD, ETH-USD, SOL-USD
- Settlement: T+0 (instant)
- Trading hours: 24/7

---

## How Your Credentials Are Protected

SynapseOMS takes credential security seriously. Your API keys never leave your machine.

### Encryption at Rest

- **Key derivation**: Your master passphrase is processed through **Argon2id** (time=1, memory=64 MB, threads=4) to produce a 256-bit encryption key
- **Encryption**: Each credential field (API key, secret, passphrase) is encrypted with **AES-256-GCM** using a unique random nonce
- **Storage**: Encrypted credentials are stored in your local PostgreSQL database — never transmitted to any external service

### What This Means

- Even if someone gains access to your database, they cannot read your API keys without your master passphrase
- Each credential uses a unique salt, so identical keys produce different ciphertexts
- AES-256-GCM provides both confidentiality and integrity — any tampering is detected

### Key Rotation

If you need to change your master passphrase, SynapseOMS supports key rotation: all credentials are decrypted with the old passphrase and re-encrypted with the new one, atomically.

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "Connection failed" after entering credentials | Verify your API key and secret are correct. For Alpaca, ensure you're using Paper Trading keys (not live). For Binance, ensure you're using Testnet keys. |
| Venue shows "Degraded" status | The exchange API may be experiencing issues. Check [status.alpaca.markets](https://status.alpaca.markets/) or Binance system status. |
| "Invalid credentials" error | API keys may have expired. Generate new keys and reconnect. |
| Credentials not persisting after restart | Ensure PostgreSQL is running (`docker compose ps`) and the database is healthy. |
