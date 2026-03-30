# sqlc-bulk-plugin

## Project Overview

A sqlc process plugin that generates adapter functions for bulk operations (UPDATE, Upsert, INSERT) using UNNEST. Converts row-oriented `[]Struct` into the column-oriented `XxxParams` that sqlc generates.

## Architecture

All source files are in `package main` (the plugin binary). Generated output goes into the user's sqlc package (e.g. `gen/`).

| File | Purpose |
|---|---|
| `main.go` | Entry point — `codegen.Run(generate)` |
| `generate.go` | Core orchestration + `bulkQuery`/`bulkField` types |
| `parse.go` | Regex-based SQL parsing (UNNEST aliases, INSERT columns, UPDATE table) |
| `catalog.go` | Catalog lookup (table columns, nullable, full-column match) |
| `typemap.go` | PG type → Go type mapping + custom enum detection + conversion expressions |
| `naming.go` | PascalCase, singularize, params field naming |
| `template.go` | text/template rendering + go/format |

## Development

```bash
# Build
go build -o sqlc-bulk-plugin .

# Run end-to-end (requires sqlc installed)
sqlc generate

# Unit tests
go test -v .

# E2E test (requires sqlc installed — builds plugin, runs sqlc generate, verifies compilation)
go test -tags e2e -v -run TestE2E .

# Update golden files after intentional output changes
go test -update .
go test -tags e2e -update -run TestE2E .
```

## Key Design Decisions

- Single `@bulk` annotation — plugin auto-detects UPDATE vs INSERT/Upsert from `Query.InsertIntoTable`.
- UNNEST alias parsing always runs for UPDATE (even with `@param` syntax) because `column.name` is the param name, not the table column name.
- For INSERT/Upsert, column names come from `INSERT INTO xxx (col1, col2, ...)` column list, not UNNEST aliases.
- Nullable is determined from Catalog, never from `Query.Params[].column.not_null` (always true for UNNEST).
- Custom types (enums) are detected by checking if the type name is absent from the built-in PG type map, not by checking `Type.Schema` (sqlc leaves schema empty for both built-in and custom types).
- Three generation styles (`function`/`method`/`interface`) controlled by `style` option. Default is `function` (standalone function accepting `Querier`).
- `:many` with single-column `RETURNING` is supported; return type resolved from `Query.Columns`.
- Assumes default sqlc settings (no rename/overrides support).
- See SPEC.md for full design rationale. Historical spike results in docs/.
