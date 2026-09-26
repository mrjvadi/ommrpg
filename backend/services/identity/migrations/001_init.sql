CREATE TABLE accounts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id   BIGINT UNIQUE,
    dev_username  TEXT UNIQUE,
    username      TEXT,
    display_name  TEXT NOT NULL,
    language      TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (telegram_id IS NOT NULL OR dev_username IS NOT NULL)
);

-- Per-account preferences such as the client HUD layout.
CREATE TABLE account_settings (
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    key        TEXT NOT NULL,
    value      JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, key)
);
