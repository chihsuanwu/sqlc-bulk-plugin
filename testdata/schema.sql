CREATE TABLE products (
    id         SERIAL PRIMARY KEY,
    name       TEXT        NOT NULL,
    price      INT         NOT NULL,
    category   TEXT,
    is_active  BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
