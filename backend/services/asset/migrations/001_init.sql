-- Tokenised items (NFTs on OMM Chain). The item itself stays in item-service;
-- this table owns *who* holds it and what state the token is in.
CREATE TABLE tokens (
    id              BIGSERIAL PRIMARY KEY,
    item_id         UUID NOT NULL UNIQUE,
    item            JSONB NOT NULL DEFAULT '{}',
    item_hash       TEXT NOT NULL DEFAULT '',
    rarity          TEXT NOT NULL DEFAULT '',
    slot            TEXT NOT NULL DEFAULT '',
    owner_account   UUID NOT NULL,
    state           TEXT NOT NULL,          -- pending | vault | claiming | depositing | bound | burned
    bound_character UUID,
    listing_id      BIGINT,
    op_id           TEXT,                   -- in-flight saga op (idempotent retries)
    op_character    UUID,
    op_prev_state   TEXT,
    minted_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX tokens_owner ON tokens (owner_account);
CREATE INDEX tokens_state ON tokens (state, updated_at);
CREATE UNIQUE INDEX tokens_op ON tokens (op_id) WHERE op_id IS NOT NULL;

CREATE TABLE listings (
    id              BIGSERIAL PRIMARY KEY,
    token_id        BIGINT NOT NULL REFERENCES tokens(id),
    seller_account  UUID NOT NULL,
    seller_character UUID,                  -- gold payouts go to this character
    currency        TEXT NOT NULL CHECK (currency IN ('TON','GOLD')),
    price           BIGINT NOT NULL CHECK (price > 0),
    fee             BIGINT NOT NULL DEFAULT 0,
    status          TEXT NOT NULL,          -- active | settling | sold | cancelled
    buyer_account   UUID,
    buyer_character UUID,
    op_id           TEXT,
    request_id      TEXT UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at       TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX listings_active ON listings (status, currency, created_at DESC);
CREATE UNIQUE INDEX listings_one_active_per_token ON listings (token_id) WHERE status IN ('active','settling');

-- TON balances (nanoTON), custodial. `locked` holds funds of pending withdrawals.
CREATE TABLE ton_balances (
    account_id UUID PRIMARY KEY,
    balance    BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    locked     BIGINT NOT NULL DEFAULT 0 CHECK (locked >= 0)
);
CREATE TABLE ton_ledger (
    id            BIGSERIAL PRIMARY KEY,
    account_id    UUID NOT NULL,
    delta         BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    reason        TEXT NOT NULL,
    ref           TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ton_ledger_account ON ton_ledger (account_id, id DESC);
CREATE TABLE deposit_memos (
    account_id UUID PRIMARY KEY,
    memo       TEXT NOT NULL UNIQUE
);
CREATE TABLE ton_deposits (
    tx_hash    TEXT PRIMARY KEY,
    lt         NUMERIC(20,0) NOT NULL,
    account_id UUID,
    amount     BIGINT NOT NULL,
    memo       TEXT NOT NULL,
    sender     TEXT NOT NULL,
    status     TEXT NOT NULL,               -- credited | unmatched
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE ton_withdrawals (
    id         BIGSERIAL PRIMARY KEY,
    account_id UUID NOT NULL,
    to_address TEXT NOT NULL,
    amount     BIGINT NOT NULL CHECK (amount > 0),
    fee        BIGINT NOT NULL,
    status     TEXT NOT NULL,               -- pending | approved | sending | sent | failed | rejected
    tx_hash    TEXT,
    reviewer   TEXT,
    note       TEXT,
    request_id TEXT UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ton_withdrawals_status ON ton_withdrawals (status, id);

-- OMM Chain: an append-only, hash-linked, signed log of every asset and TON
-- movement. Transactions are sealed into blocks with a Merkle root.
CREATE TABLE chain_txs (
    id           BIGSERIAL PRIMARY KEY,
    hash         TEXT NOT NULL UNIQUE,
    kind         TEXT NOT NULL,
    token_id     BIGINT,
    from_party   TEXT,
    to_party     TEXT,
    amount       BIGINT,
    currency     TEXT,
    payload      JSONB,
    block_height BIGINT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chain_txs_pending ON chain_txs (id) WHERE block_height IS NULL;
CREATE INDEX chain_txs_token ON chain_txs (token_id, id);
CREATE TABLE chain_blocks (
    height      BIGINT PRIMARY KEY,
    prev_hash   TEXT NOT NULL,
    merkle_root TEXT NOT NULL,
    hash        TEXT NOT NULL UNIQUE,
    tx_count    INT NOT NULL,
    signature   TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL
);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value JSONB NOT NULL
);
