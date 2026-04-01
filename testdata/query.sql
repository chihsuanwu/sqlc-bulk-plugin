-- 1. Bulk UPDATE, full columns, $N syntax → model struct reuse
-- Tests: nullable text conversion, custom enum, timestamptz
-- @bulk
-- name: BulkUpdateOrders :exec
UPDATE orders AS o SET
    customer   = u.customer,
    amount     = u.amount,
    note       = u.note,
    is_paid    = u.is_paid,
    status     = u.status,
    ordered_at = u.ordered_at
FROM (
    SELECT
        UNNEST($1::int[])          AS id,
        UNNEST($2::text[])         AS customer,
        UNNEST($3::int[])          AS amount,
        UNNEST($4::text[])         AS note,
        UNNEST($5::boolean[])      AS is_paid,
        UNNEST($6::order_status[]) AS status,
        UNNEST($7::timestamptz[])  AS ordered_at
) AS u
WHERE o.id = u.id;

-- 2. Bulk UPDATE, partial columns, @param syntax → Item struct + named params
-- @bulk
-- name: BulkUpdateItemPrices :exec
UPDATE items AS t SET
    price = u.price
FROM (
    SELECT
        UNNEST(@ids::int[])    AS id,
        UNNEST(@prices::int[]) AS price
) AS u
WHERE t.id = u.id;

-- 3. Upsert, VALUES format
-- @bulk
-- name: UpsertItems :exec
INSERT INTO items (name, price)
VALUES (
    UNNEST($1::text[]),
    UNNEST($2::int[])
)
ON CONFLICT (name) DO UPDATE SET
    price = EXCLUDED.price;

-- 4. Upsert, SELECT format
-- @bulk
-- name: UpsertOrders :exec
INSERT INTO orders (customer, amount, note, is_paid, status, ordered_at)
SELECT
    UNNEST($1::text[]),
    UNNEST($2::int[]),
    UNNEST($3::text[]),
    UNNEST($4::boolean[]),
    UNNEST($5::order_status[]),
    UNNEST($6::timestamptz[])
ON CONFLICT (id) DO UPDATE SET
    amount = EXCLUDED.amount;

-- 5. INSERT with RETURNING (single column, :many)
-- @bulk
-- name: BulkInsertItems :many
INSERT INTO items (name, price)
VALUES (
    UNNEST($1::text[]),
    UNNEST($2::int[])
)
RETURNING id;

-- 6. Bulk UPDATE with uuid, jsonb, bytea, numeric types
-- @bulk
-- name: BulkUpdateEvents :exec
UPDATE events AS e SET
    payload = u.payload,
    image   = u.image,
    score   = u.score
FROM (
    SELECT
        UNNEST($1::uuid[])    AS id,
        UNNEST($2::jsonb[])   AS payload,
        UNNEST($3::bytea[])   AS image,
        UNNEST($4::numeric[]) AS score
) AS u
WHERE e.id = u.id;

-- 7. Upsert with NULLIF wrapping UNNEST
-- @bulk
-- name: UpsertItemsNullif :exec
INSERT INTO items (name, price)
VALUES (
    NULLIF(UNNEST($1::text[]), ''),
    UNNEST($2::int[])
)
ON CONFLICT (name) DO UPDATE SET
    price = EXCLUDED.price;

-- 8. Non-bulk query — should be ignored by plugin
-- name: GetOrderByID :one
SELECT * FROM orders WHERE id = $1;
