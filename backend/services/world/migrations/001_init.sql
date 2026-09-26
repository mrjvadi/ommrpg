-- Only the seed and static structure of a world are persisted; all terrain,
-- species, monsters and dungeon entrances are recomputed from the seed.
CREATE TABLE worlds (
    id          SERIAL PRIMARY KEY,
    seed        NUMERIC(20,0) NOT NULL,
    size_chunks INT NOT NULL CHECK (size_chunks BETWEEN 8 AND 512),
    name        TEXT NOT NULL,
    is_default  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX worlds_one_default ON worlds (is_default) WHERE is_default;
