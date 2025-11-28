# Stock Reward Service

Small Go service for granting stock rewards to users, valuing positions with a deterministic mock pricing feed, and exposing portfolio/ledger-style views over HTTP. Runs against PostgreSQL or a lightweight in-memory store for local use.

## Quick start
- Requirements: Go 1.23+, PostgreSQL (optional if you want persistence).
- Copy env: `cp .env.example bin/.env` and adjust values. The loader looks in `bin/.env` (next to the built binary) and falls back to `.env`.
- (Optional) Create tables: `psql "$DATABASE_URL" -f schema.sql`.
- Run the server: `go run ./cmd/server` (defaults to `:8080`).

## Configuration
Environment variables (load order: `bin/.env`, `.env`):
- `PORT` (default `8080`)
- `ENVIRONMENT` (`local` | `dev` | `prod`, default `local`)
- `DATABASE_URL` (PostgreSQL connection string; if empty the app uses the in-memory repository)
- `PRICE_TTL_MINUTES` (cache TTL for mock quotes, default `60`)

## Postman collection
- Import `postman_collection.json` and set the `baseUrl` and `userId` variables as needed.

## API
Base URL: `http://localhost:PORT`

- `POST /reward` — create a reward event (idempotent via `eventId`).
  ```bash
  curl -X POST http://localhost:8080/reward \
    -H "Content-Type: application/json" \
    -d '{
      "userId": "u1",
      "symbol": "AAPL",
      "quantity": "5.5",
      "rewardedAt": "2024-12-25T10:00:00Z",
      "eventId": "evt-123",          // optional idempotency key
      "adjustment": false,
      "fees": { "brokerage": "5.25", "stt": "1.1", "gst": "0.9", "other": "0" }
    }'
  ```
  Response: `201` with `rewardId`, `totalInrCost`, etc. Returns `409` on duplicate `eventId`.

- `GET /today-stocks/:userId` — rewards for the user created today (UTC).
- `GET /historical-inr/:userId` — daily INR totals across history (uses historical mock prices).
- `GET /stats/:userId` — total shares granted today per symbol + latest portfolio value.
- `GET /portfolio/:userId` — current positions with latest prices and INR values.

## Data model
- `schema.sql` defines `rewards` and `ledger_entries` tables (unique idempotency index on `user_id + idempotency_key`).
- The pricing service is deterministic pseudo-random; values change with time but are stable within the cache TTL.

## Edge cases and behavior
- Duplicate rewards: prevented with the idempotency key (`eventId`), enforced in DB and service.
- Adjustments/refunds: allowed via `adjustment: true` with negative quantities.
- Pricing outages/staleness: on-demand mock pricing; if calls fail, endpoints log and skip affected symbols (values may be partial).
- Corporate actions (splits/mergers/delistings): not implemented; would require symbol mapping and position rewrites.
- Rounding: uses `shopspring/decimal` with NUMERIC columns to avoid float drift.

## Scaling notes
- Read/write separation via repository interface; swap in other stores as needed.
- Pricing cache TTL reduces repeated lookups; could be extended with a background refresher or real provider.
- Idempotent writes and DB indexes keep ingestion safe under retries.

## Development notes
- Logging via logrus with request middleware in `internal/http`.
- In-memory repository is thread-safe but non-persistent; PostgreSQL implementation lives in `internal/repository/postgres`.
- Build to `bin/` if you want to colocate the binary and `.env`.
