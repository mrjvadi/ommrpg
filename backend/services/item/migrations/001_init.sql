CREATE TABLE items (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id         UUID NOT NULL,             -- character id
    seed             NUMERIC(20,0) NOT NULL,
    item             JSONB NOT NULL,            -- immutable generated snapshot (items.Item)
    rarity           TEXT NOT NULL,
    slot             TEXT NOT NULL,
    enhance          INT NOT NULL DEFAULT 0,
    level            INT NOT NULL DEFAULT 1,
    xp               BIGINT NOT NULL DEFAULT 0,
    enhance_attempts BIGINT NOT NULL DEFAULT 0,
    equipped_slot    TEXT,
    source           TEXT NOT NULL,
    source_ref       TEXT UNIQUE,              -- prevents duplicate grants of the same drop
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX items_owner ON items (owner_id);
CREATE UNIQUE INDEX items_one_per_slot ON items (owner_id, equipped_slot) WHERE equipped_slot IS NOT NULL;

CREATE TABLE wallets (
    owner_id UUID PRIMARY KEY,
    gold     BIGINT NOT NULL DEFAULT 0 CHECK (gold >= 0),
    essence  BIGINT NOT NULL DEFAULT 0 CHECK (essence >= 0)
);

-- Append-only economic ledger (docs/ECONOMY.md).
CREATE TABLE ledger (
    id            BIGSERIAL PRIMARY KEY,
    owner_id      UUID NOT NULL,
    currency      TEXT NOT NULL,
    delta         BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    reason        TEXT NOT NULL,
    ref           TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ledger_owner ON ledger (owner_id, id);

-- Enhancement audit: every roll is reproducible from (secret, item, attempt).
CREATE TABLE enhance_log (
    id         BIGSERIAL PRIMARY KEY,
    item_id    UUID NOT NULL,
    owner_id   UUID NOT NULL,
    attempt    BIGINT NOT NULL,
    from_level INT NOT NULL,
    to_level   INT NOT NULL,
    chance     DOUBLE PRECISION NOT NULL,
    roll       DOUBLE PRECISION NOT NULL,
    gold       BIGINT NOT NULL,
    essence    BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE idempotency (
    request_id TEXT PRIMARY KEY,
    owner_id   UUID NOT NULL,
    response   JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE processed_events (
    event_id     TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
