CREATE TABLE admin_users (
    username      TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('owner','admin','moderator','viewer')),
    disabled      BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ
);

-- Every state-changing admin action is recorded here.
CREATE TABLE audit_log (
    id         BIGSERIAL PRIMARY KEY,
    admin      TEXT NOT NULL,
    action     TEXT NOT NULL,
    target     TEXT,
    details    JSONB,
    ok         BOOLEAN NOT NULL,
    error      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_time ON audit_log (id DESC);
