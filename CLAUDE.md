# sqlc-bulk-plugin

## Project Overview

A sqlc process plugin that generates adapter functions for bulk UPDATE operations using UNNEST. Converts row-oriented `[]Struct` into the column-oriented `XxxParams` that sqlc generates.

## Architecture

All source files are in `package main` (the plugin binary). Generated output goes into the user's sqlc package (e.g. `gen/`).

| File | Purpose |
|---|---|
| `main.go` | Entry point — `codegen.Run(generate)` |
| `generate.go` | Core orchestration + `bulkQuery`/`bulkField` types |
| `parse.go` | Regex-based SQL parsing (UNNEST aliases, UPDATE table) |
| `catalog.go` | Catalog lookup (table columns, nullable, full-column match) |
| `typemap.go` | PG type → Go type mapping + conversion expressions |
| `naming.go` | PascalCase, singularize, params field naming |
| `template.go` | text/template rendering + go/format |

## Development

```bash
# Build
go build -o sqlc-bulk-plugin .

# Run end-to-end (requires sqlc installed)
sqlc generate

# Test
go test -v .

# Update golden files after intentional output changes
go test -update .
```

## Key Design Decisions

- UNNEST alias parsing always runs (even with `@param` syntax) because `column.name` is the param name, not the table column name. See SPEC.md FR-7.
- Nullable is determined from Catalog, never from `Query.Params[].column.not_null` (always true for UNNEST).
- Three generation styles (`function`/`method`/`interface`) controlled by `style` option. Default is `function` (standalone function accepting `Querier`). See SPEC.md FR-6.
- Phase 1 assumes default sqlc settings (no rename/overrides support).
- See SPEC.md for full design rationale and spike results.
