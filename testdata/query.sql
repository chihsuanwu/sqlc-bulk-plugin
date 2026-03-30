-- =============================================
-- Case 1: Bulk UPDATE with $N positional params
-- =============================================
-- @bulk update
-- name: BulkUpdateProducts :exec
UPDATE products AS p SET
    name       = u.name,
    price      = u.price,
    category   = u.category,
    is_active  = u.is_active,
    updated_at = u.updated_at
FROM (
    SELECT
        UNNEST($1::int[])          AS id,
        UNNEST($2::text[])         AS name,
        UNNEST($3::int[])          AS price,
        UNNEST($4::text[])         AS category,
        UNNEST($5::boolean[])      AS is_active,
        UNNEST($6::timestamptz[])  AS updated_at
) AS u
WHERE p.id = u.id;

-- =============================================
-- Case 2: Bulk UPDATE with only a subset of columns (partial update)
-- =============================================
-- @bulk update
-- name: BulkUpdateProductPrices :exec
UPDATE products AS p SET
    price      = u.price,
    updated_at = u.updated_at
FROM (
    SELECT
        UNNEST($1::int[])          AS id,
        UNNEST($2::int[])          AS price,
        UNNEST($3::timestamptz[])  AS updated_at
) AS u
WHERE p.id = u.id;

-- =============================================
-- Case 3: Bulk Upsert (INSERT ON CONFLICT) with UNNEST in FROM clause
-- =============================================
-- @bulk upsert
-- name: UpsertProducts :exec
INSERT INTO products (name, price, category, is_active, updated_at)
SELECT name, price, category, is_active, updated_at
FROM (
    SELECT
        UNNEST($1::text[])         AS name,
        UNNEST($2::int[])          AS price,
        UNNEST($3::text[])         AS category,
        UNNEST($4::boolean[])      AS is_active,
        UNNEST($5::timestamptz[])  AS updated_at
) AS u
ON CONFLICT (name) DO UPDATE SET
    price      = EXCLUDED.price,
    category   = EXCLUDED.category,
    is_active  = EXCLUDED.is_active,
    updated_at = EXCLUDED.updated_at;

-- =============================================
-- Case 4: Bulk Upsert using UNNEST directly in SELECT (alternative syntax)
--         *** sqlc REJECTS this: "function unnest(unknown, ...) does not exist" ***
--         Multi-arg UNNEST without individual casts is not supported by sqlc's parser.
--         This syntax is NOT supported by the plugin.
-- =============================================

-- =============================================
-- Case 5: Normal query (no @bulk annotation) — should be ignored by plugin
-- =============================================
-- name: GetProductByID :one
SELECT id, name, price, category, is_active, updated_at
FROM products
WHERE id = $1;

-- =============================================
-- Case 6: Bulk UPDATE using @param_name syntax (for comparison)
-- =============================================
-- @bulk update
-- name: BulkUpdateProductNames :exec
UPDATE products AS p SET
    name = u.name
FROM (
    SELECT
        UNNEST(@ids::int[])    AS id,
        UNNEST(@names::text[]) AS name
) AS u
WHERE p.id = u.id;
