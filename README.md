# sqlc-bulk-plugin

> **⚠️ Early Release** — Core features work but the project is still under active development. Please review generated code before using in production.

A [sqlc](https://sqlc.dev/) process plugin that generates adapter functions for bulk operations using PostgreSQL's `UNNEST` pattern.

## Problem

When using `UNNEST` for bulk UPDATE, sqlc generates column-oriented params:

```go
// sqlc generates this — one slice per column
type BulkUpdateProductsParams struct {
    Column1 []int32
    Column2 []string
    Column3 []int32
}
```

You end up writing boilerplate to convert from `[]Product` to this struct for every bulk query.

## Solution

Annotate your query with `-- @bulk update` and this plugin generates the adapter for you:

```go
// Plugin generates this (default style: function)
func BulkUpdateProductsBatch(ctx context.Context, q Querier, items []Product) error {
    params := BulkUpdateProductsParams{
        Column1: make([]int32, len(items)),
        Column2: make([]string, len(items)),
        // ...
    }
    for i, item := range items {
        params.Column1[i] = item.ID
        params.Column2[i] = item.Name
        // ...
    }
    return q.BulkUpdateProducts(ctx, params)
}
```

## Setup

### 1. Install

```bash
go install github.com/chihsuanwu/sqlc-bulk-plugin@latest
```

### 2. Configure `sqlc.yaml`

```yaml
version: "2"
plugins:
  - name: bulk
    process:
      cmd: sqlc-bulk-plugin
sql:
  - schema: schema.sql
    queries: query.sql
    engine: postgresql
    gen:
      go:
        package: db
        out: gen
        sql_package: pgx/v5
    codegen:
      - plugin: bulk
        out: gen            # Must match gen.go.out
        options:
          package: db       # Must match gen.go.package
          style: function   # "function" (default) | "method" | "interface"
```

### 3. Annotate queries

Add `-- @bulk` above your query:

```sql
-- @bulk
-- name: BulkUpdateProducts :exec
UPDATE products AS p SET
    name       = u.name,
    price      = u.price,
    category   = u.category
FROM (
    SELECT
        UNNEST($1::int[])  AS id,
        UNNEST($2::text[]) AS name,
        UNNEST($3::int[])  AS price,
        UNNEST($4::text[]) AS category
) AS u
WHERE p.id = u.id;
```

```sql
-- @bulk
-- name: UpsertProducts :exec
INSERT INTO products (id, name, price, category)
VALUES (
    UNNEST($1::int[]),
    UNNEST($2::text[]),
    UNNEST($3::int[]),
    UNNEST($4::text[])
)
ON CONFLICT (id) DO UPDATE SET
    name     = EXCLUDED.name,
    price    = EXCLUDED.price,
    category = EXCLUDED.category;
```

### 4. Generate

```bash
sqlc generate
```

This produces `bulk.go` alongside sqlc's normal output.

## Features

- Single `-- @bulk` annotation for all bulk operations (UPDATE, upsert, INSERT)
- Supports both `$N` and `@param_name` parameter syntax
- Full-column queries reuse sqlc's model struct (e.g. `[]Product`)
- Partial-column queries get a dedicated `XxxItem` struct
- Three generation styles via `style` option (see below)
- Handles nullable columns (`pgtype.*` ↔ base type conversion)

## Generation Styles

Control how the adapter is generated with the `style` option:

| `style` | Generates | Best for |
|---|---|---|
| `function` (default) | Standalone function: `BulkXxxBatch(ctx, q, items)` | Projects using `Querier` interface (`emit_interface: true`) |
| `method` | Method on `*Queries`: `q.BulkXxxBatch(ctx, items)` | Projects using `*Queries` directly |
| `interface` | Method + `BulkQuerier` interface (embeds `Querier`) | Projects willing to adopt a combined interface |

## Supported Types

| PostgreSQL | Go (not null) | Go (nullable) |
|---|---|---|
| `int4`, `serial` | `int32` | `pgtype.Int4` |
| `int2` | `int16` | `pgtype.Int2` |
| `int8`, `bigserial` | `int64` | `pgtype.Int8` |
| `text`, `varchar` | `string` | `pgtype.Text` |
| `bool` | `bool` | `pgtype.Bool` |
| `timestamptz` | `pgtype.Timestamptz` | `pgtype.Timestamptz` |
| `timestamp` | `pgtype.Timestamp` | `pgtype.Timestamp` |
| `float4` | `float32` | `pgtype.Float4` |
| `float8` | `float64` | `pgtype.Float8` |

## What about bulk INSERT?

For pure bulk INSERT (without `ON CONFLICT`), you don't need this plugin. Use sqlc's built-in [`:copyfrom`](https://docs.sqlc.dev/en/stable/howto/insert.html#using-copyfrom) command, which generates a row-oriented adapter using PostgreSQL's `COPY` protocol directly.

## Limitations

- PostgreSQL + pgx/v5 only
- Assumes default sqlc settings (`rename`, `overrides`, `emit_pointers_for_null_types` not yet supported)
- Process plugin only (no WASM)

## Disclaimer

This project was almost entirely built by [Claude Code](https://claude.ai/code) — from the initial spike and spec design through implementation and testing. If you have concerns about AI-generated code, please review the source carefully before using it in production.

## License

MIT
