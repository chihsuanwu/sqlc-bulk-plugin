-- Custom enum type (tests custom type detection)
CREATE TYPE order_status AS ENUM ('pending', 'confirmed', 'cancelled');

-- Table 1: all common PG types + nullable columns + custom enum
CREATE TABLE orders (
    id          SERIAL PRIMARY KEY,
    customer    TEXT         NOT NULL,
    amount      INT          NOT NULL,
    note        TEXT,                     -- nullable text
    is_paid     BOOLEAN      NOT NULL DEFAULT FALSE,
    status      order_status NOT NULL,    -- custom enum (NOT NULL)
    ordered_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Table 2: simple table for partial-column / upsert tests
CREATE TABLE items (
    id    SERIAL PRIMARY KEY,
    name  TEXT NOT NULL UNIQUE,
    price INT  NOT NULL
);
