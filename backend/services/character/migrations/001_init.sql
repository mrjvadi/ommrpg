CREATE TABLE characters (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id  UUID NOT NULL,
    name        TEXT NOT NULL,
    world_id    INT NOT NULL,
    level       INT NOT NULL DEFAULT 1,
    xp          BIGINT NOT NULL DEFAULT 0,
    attr_str    INT NOT NULL DEFAULT 5,
    attr_agi    INT NOT NULL DEFAULT 5,
    attr_int    INT NOT NULL DEFAULT 5,
    attr_vit    INT NOT NULL DEFAULT 5,
    free_points INT NOT NULL DEFAULT 0 CHECK (free_points >= 0),
    class       TEXT NOT NULL DEFAULT '',
    hidden      JSONB NOT NULL,           -- Root / Talent / Potential: never sent to clients
    behaviour   JSONB NOT NULL DEFAULT '{}',
    appearance  JSONB NOT NULL,
    zone        TEXT NOT NULL,
    pos_x       DOUBLE PRECISION NOT NULL,
    pos_y       DOUBLE PRECISION NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX characters_name_unique ON characters (lower(name));
CREATE INDEX characters_account ON characters (account_id);

-- Consumed event ids, so at-least-once delivery never double-applies XP.
CREATE TABLE processed_events (
    event_id     TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
