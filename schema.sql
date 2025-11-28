-- Database: assignment

CREATE TABLE IF NOT EXISTS rewards (
    id UUID PRIMARY KEY,
    user_id TEXT NOT NULL,
    symbol TEXT NOT NULL,
    quantity NUMERIC(18,6) NOT NULL,
    rewarded_at TIMESTAMPTZ NOT NULL,
    idempotency_key TEXT,
    fees_brokerage NUMERIC(18,4) DEFAULT 0,
    fees_stt NUMERIC(18,4) DEFAULT 0,
    fees_gst NUMERIC(18,4) DEFAULT 0,
    fees_other NUMERIC(18,4) DEFAULT 0,
    unit_price_inr NUMERIC(18,4) NOT NULL,
    total_inr_cost NUMERIC(18,4) NOT NULL,
    priced_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rewards_user_date ON rewards(user_id, rewarded_at);
CREATE UNIQUE INDEX IF NOT EXISTS rewards_idem ON rewards(user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS ledger_entries (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES rewards(id),
    user_id TEXT NOT NULL,
    account TEXT NOT NULL,
    symbol TEXT,
    units NUMERIC(18,6) NOT NULL DEFAULT 0,
    amount_inr NUMERIC(18,4) NOT NULL,
    entry_type TEXT NOT NULL CHECK (entry_type IN ('debit','credit')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ledger_event ON ledger_entries(event_id);
CREATE INDEX IF NOT EXISTS idx_ledger_user ON ledger_entries(user_id);
