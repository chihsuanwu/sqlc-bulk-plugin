package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

var update = flag.Bool("update", false, "update golden files")

func TestGenerateStyles(t *testing.T) {
	styles := []string{styleFunction, styleMethod, styleInterface}
	for _, style := range styles {
		t.Run(style, func(t *testing.T) {
			req := buildTestRequestWithStyle(style)
			resp, err := generate(context.Background(), req)
			if err != nil {
				t.Fatalf("generate() error: %v", err)
			}
			if len(resp.Files) != 1 {
				t.Fatalf("expected 1 file, got %d", len(resp.Files))
			}

			got := resp.Files[0].Contents
			goldenPath := filepath.Join("testdata", "golden", "bulk_update_"+style+".go.golden")

			if *update {
				os.MkdirAll(filepath.Dir(goldenPath), 0755)
				if err := os.WriteFile(goldenPath, got, 0644); err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
				t.Log("golden file updated")
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("failed to read golden file (run with -update to create): %v", err)
			}

			if string(got) != string(want) {
				t.Errorf("output does not match golden file.\n\nGot:\n%s\n\nWant:\n%s", got, want)
			}
		})
	}
}

func TestGenerateNoAnnotation(t *testing.T) {
	req := &plugin.GenerateRequest{
		PluginOptions: []byte(`{"package": "db"}`),
		Catalog:       buildTestCatalog(),
		Queries: []*plugin.Query{
			{
				Name: "GetProductByID",
				Cmd:  ":one",
				Text: "SELECT id, name FROM products WHERE id = $1",
			},
		},
	}
	resp, err := generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate() error: %v", err)
	}
	if len(resp.Files) != 0 {
		t.Errorf("expected 0 files for non-bulk query, got %d", len(resp.Files))
	}
}

func TestGenerateInvalidStyle(t *testing.T) {
	req := &plugin.GenerateRequest{
		PluginOptions: []byte(`{"package": "db", "style": "invalid"}`),
		Catalog:       buildTestCatalog(),
		Queries:       []*plugin.Query{},
	}
	_, err := generate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid style, got nil")
	}
}

// TestGenerateNullableFields tests multiple nullable columns with different pgtype conversions.
func TestGenerateNullableFields(t *testing.T) {
	catalog := &plugin.Catalog{
		DefaultSchema: "public",
		Schemas: []*plugin.Schema{
			{
				Name: "public",
				Tables: []*plugin.Table{
					{
						Rel: &plugin.Identifier{Name: "orders"},
						Columns: []*plugin.Column{
							{Name: "id", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "int4"}},
							{Name: "note", NotNull: false, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "text"}},
							{Name: "discount", NotNull: false, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "int4"}},
							{Name: "is_verified", NotNull: false, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "bool"}},
							{Name: "score", NotNull: false, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "float8"}},
						},
					},
				},
			},
		},
	}

	opts, _ := json.Marshal(pluginOptions{Package: "db", Style: styleFunction})
	req := &plugin.GenerateRequest{
		PluginOptions: opts,
		Catalog:       catalog,
		Queries: []*plugin.Query{
			{
				Name:     "BulkUpdateOrders",
				Cmd:      ":exec",
				Comments: []string{"@bulk update"},
				Text: `UPDATE orders AS o SET
    note        = u.note,
    discount    = u.discount,
    is_verified = u.is_verified,
    score       = u.score
FROM (
    SELECT
        UNNEST($1::int[])             AS id,
        UNNEST($2::text[])            AS note,
        UNNEST($3::int[])             AS discount,
        UNNEST($4::boolean[])         AS is_verified,
        UNNEST($5::float8[]) AS score
) AS u
WHERE o.id = u.id`,
				Params: []*plugin.Parameter{
					makeParam(1, "", "int4", true),
					makeParam(2, "", "text", true),
					makeParam(3, "", "int4", true),
					makeParam(4, "", "bool", true),
					makeParam(5, "", "float8", true),
				},
			},
		},
	}

	resp, err := generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate() error: %v", err)
	}
	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}

	got := string(resp.Files[0].Contents)
	goldenPath := filepath.Join("testdata", "golden", "bulk_update_nullable.go.golden")

	if *update {
		os.MkdirAll(filepath.Dir(goldenPath), 0755)
		os.WriteFile(goldenPath, []byte(got), 0644)
		t.Log("golden file updated")
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file (run with -update to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("output mismatch.\n\nGot:\n%s\n\nWant:\n%s", got, want)
	}
}

// TestGenerateMultiTable tests that the correct table is found when catalog has multiple tables.
func TestGenerateMultiTable(t *testing.T) {
	catalog := &plugin.Catalog{
		DefaultSchema: "public",
		Schemas: []*plugin.Schema{
			{
				Name: "public",
				Tables: []*plugin.Table{
					{
						Rel: &plugin.Identifier{Name: "users"},
						Columns: []*plugin.Column{
							{Name: "id", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "int4"}},
							{Name: "email", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "text"}},
						},
					},
					{
						Rel: &plugin.Identifier{Name: "products"},
						Columns: []*plugin.Column{
							{Name: "id", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "int4"}},
							{Name: "name", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "text"}},
						},
					},
				},
			},
		},
	}

	opts, _ := json.Marshal(pluginOptions{Package: "db", Style: styleMethod})
	req := &plugin.GenerateRequest{
		PluginOptions: opts,
		Catalog:       catalog,
		Queries: []*plugin.Query{
			{
				Name:     "BulkUpdateUsers",
				Cmd:      ":exec",
				Comments: []string{"@bulk update"},
				Text: `UPDATE users AS u SET
    email = v.email
FROM (
    SELECT
        UNNEST($1::int[])  AS id,
        UNNEST($2::text[]) AS email
) AS v
WHERE u.id = v.id`,
				Params: []*plugin.Parameter{
					makeParam(1, "", "int4", true),
					makeParam(2, "", "text", true),
				},
			},
		},
	}

	resp, err := generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate() error: %v", err)
	}
	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}

	got := string(resp.Files[0].Contents)

	// Full column match (users has id + email) → should reuse model struct "User"
	if !strings.Contains(got, "items []User") {
		t.Errorf("expected model struct reuse ([]User), got:\n%s", got)
	}
	// Should NOT generate an Item struct
	if strings.Contains(got, "BulkUpdateUsersItem") {
		t.Errorf("should not generate Item struct when full column match, got:\n%s", got)
	}
}

// TestGenerateParamFullColumn tests @param syntax with full column match → model struct reuse.
func TestGenerateParamFullColumn(t *testing.T) {
	catalog := &plugin.Catalog{
		DefaultSchema: "public",
		Schemas: []*plugin.Schema{
			{
				Name: "public",
				Tables: []*plugin.Table{
					{
						Rel: &plugin.Identifier{Name: "tags"},
						Columns: []*plugin.Column{
							{Name: "id", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "int4"}},
							{Name: "label", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "text"}},
						},
					},
				},
			},
		},
	}

	opts, _ := json.Marshal(pluginOptions{Package: "db", Style: styleFunction})
	req := &plugin.GenerateRequest{
		PluginOptions: opts,
		Catalog:       catalog,
		Queries: []*plugin.Query{
			{
				Name:     "BulkUpdateTags",
				Cmd:      ":exec",
				Comments: []string{"@bulk update"},
				Text: `UPDATE tags AS t SET
    label = u.label
FROM (
    SELECT
        UNNEST($1::int[])  AS id,
        UNNEST($2::text[]) AS label
) AS u
WHERE t.id = u.id`,
				Params: []*plugin.Parameter{
					makeParam(1, "tag_ids", "int4", true),
					makeParam(2, "labels", "text", true),
				},
			},
		},
	}

	resp, err := generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate() error: %v", err)
	}
	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}

	got := string(resp.Files[0].Contents)

	// Full column match → model struct reuse
	if !strings.Contains(got, "items []Tag") {
		t.Errorf("expected model struct reuse ([]Tag), got:\n%s", got)
	}
	// @param syntax → named params fields
	if !strings.Contains(got, "params.TagIds[i]") {
		t.Errorf("expected @param field name TagIds, got:\n%s", got)
	}
	if !strings.Contains(got, "params.Labels[i]") {
		t.Errorf("expected @param field name Labels, got:\n%s", got)
	}
}

// TestGenerateErrorTableNotFound tests that an error is returned when the UPDATE table is not in catalog.
func TestGenerateErrorTableNotFound(t *testing.T) {
	opts, _ := json.Marshal(pluginOptions{Package: "db", Style: styleFunction})
	req := &plugin.GenerateRequest{
		PluginOptions: opts,
		Catalog:       buildTestCatalog(), // only has "products"
		Queries: []*plugin.Query{
			{
				Name:     "BulkUpdateMissing",
				Cmd:      ":exec",
				Comments: []string{"@bulk update"},
				Text: `UPDATE nonexistent_table AS t SET
    name = u.name
FROM (
    SELECT
        UNNEST($1::int[])  AS id,
        UNNEST($2::text[]) AS name
) AS u
WHERE t.id = u.id`,
				Params: []*plugin.Parameter{
					makeParam(1, "", "int4", true),
					makeParam(2, "", "text", true),
				},
			},
		},
	}

	_, err := generate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing table, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent_table") {
		t.Errorf("error should mention table name, got: %v", err)
	}
}

// TestGenerateErrorMissingUNNEST tests error when SQL lacks UNNEST pattern.
func TestGenerateErrorMissingUNNEST(t *testing.T) {
	opts, _ := json.Marshal(pluginOptions{Package: "db", Style: styleFunction})
	req := &plugin.GenerateRequest{
		PluginOptions: opts,
		Catalog:       buildTestCatalog(),
		Queries: []*plugin.Query{
			{
				Name:     "BulkUpdateBadSQL",
				Cmd:      ":exec",
				Comments: []string{"@bulk update"},
				Text:     `UPDATE products SET name = $1 WHERE id = $2`,
				Params: []*plugin.Parameter{
					makeParam(1, "", "text", true),
					makeParam(2, "", "int4", true),
				},
			},
		},
	}

	_, err := generate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing UNNEST pattern, got nil")
	}
	if !strings.Contains(err.Error(), "UNNEST") {
		t.Errorf("error should mention UNNEST, got: %v", err)
	}
}

// TestGenerateBulkUpsertSkipped tests that @bulk upsert queries are silently skipped (Phase 2).
func TestGenerateBulkUpsertSkipped(t *testing.T) {
	opts, _ := json.Marshal(pluginOptions{Package: "db", Style: styleFunction})
	req := &plugin.GenerateRequest{
		PluginOptions: opts,
		Catalog:       buildTestCatalog(),
		Queries: []*plugin.Query{
			{
				Name:     "UpsertProducts",
				Cmd:      ":exec",
				Comments: []string{"@bulk upsert"},
				Text:     `INSERT INTO products (id, name) SELECT * FROM UNNEST($1::int[], $2::text[]) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`,
				Params: []*plugin.Parameter{
					makeParam(1, "", "int4", true),
					makeParam(2, "", "text", true),
				},
			},
		},
	}

	resp, err := generate(context.Background(), req)
	if err != nil {
		t.Fatalf("bulk upsert should not error, got: %v", err)
	}
	if len(resp.Files) != 0 {
		t.Errorf("expected 0 files for upsert (Phase 2 skip), got %d", len(resp.Files))
	}
}

// TestGenerateNoPgtype tests that pgtype import is omitted when no fields need it.
func TestGenerateNoPgtype(t *testing.T) {
	catalog := &plugin.Catalog{
		DefaultSchema: "public",
		Schemas: []*plugin.Schema{
			{
				Name: "public",
				Tables: []*plugin.Table{
					{
						Rel: &plugin.Identifier{Name: "tags"},
						Columns: []*plugin.Column{
							{Name: "id", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "int4"}},
							{Name: "label", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "text"}},
						},
					},
				},
			},
		},
	}

	opts, _ := json.Marshal(pluginOptions{Package: "db", Style: styleFunction})
	req := &plugin.GenerateRequest{
		PluginOptions: opts,
		Catalog:       catalog,
		Queries: []*plugin.Query{
			{
				Name:     "BulkUpdateTags",
				Cmd:      ":exec",
				Comments: []string{"@bulk update"},
				Text: `UPDATE tags AS t SET
    label = u.label
FROM (
    SELECT
        UNNEST($1::int[])  AS id,
        UNNEST($2::text[]) AS label
) AS u
WHERE t.id = u.id`,
				Params: []*plugin.Parameter{
					makeParam(1, "", "int4", true),
					makeParam(2, "", "text", true),
				},
			},
		},
	}

	resp, err := generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate() error: %v", err)
	}
	got := string(resp.Files[0].Contents)
	if strings.Contains(got, "pgtype") {
		t.Errorf("expected no pgtype import for all-not-null basic types, got:\n%s", got)
	}
}

func buildTestRequestWithStyle(style string) *plugin.GenerateRequest {
	opts, _ := json.Marshal(pluginOptions{Package: "db", Style: style})
	return &plugin.GenerateRequest{
		PluginOptions: opts,
		Catalog:       buildTestCatalog(),
		Queries:       buildTestQueries(),
	}
}

func buildTestQueries() []*plugin.Query {
	return []*plugin.Query{
		// Case 1: Full-column bulk update with $N syntax
		{
			Name:     "BulkUpdateProducts",
			Cmd:      ":exec",
			Comments: []string{"@bulk update"},
			Text: `UPDATE products AS p SET
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
WHERE p.id = u.id`,
			Params: []*plugin.Parameter{
				makeParam(1, "", "int4", true),
				makeParam(2, "", "text", true),
				makeParam(3, "", "int4", true),
				makeParam(4, "", "text", true),
				makeParam(5, "", "bool", true),
				makeParam(6, "", "timestamptz", true),
			},
		},
		// Case 2: Partial-column bulk update with $N syntax
		{
			Name:     "BulkUpdateProductPrices",
			Cmd:      ":exec",
			Comments: []string{"@bulk update"},
			Text: `UPDATE products AS p SET
    price      = u.price,
    updated_at = u.updated_at
FROM (
    SELECT
        UNNEST($1::int[])          AS id,
        UNNEST($2::int[])          AS price,
        UNNEST($3::timestamptz[])  AS updated_at
) AS u
WHERE p.id = u.id`,
			Params: []*plugin.Parameter{
				makeParam(1, "", "int4", true),
				makeParam(2, "", "int4", true),
				makeParam(3, "", "timestamptz", true),
			},
		},
		// Case 3: @param syntax (sqlc converts @param to $N in Query.Text)
		{
			Name:     "BulkUpdateProductNames",
			Cmd:      ":exec",
			Comments: []string{"@bulk update"},
			Text: `UPDATE products AS p SET
    name = u.name
FROM (
    SELECT
        UNNEST($1::int[])    AS id,
        UNNEST($2::text[]) AS name
) AS u
WHERE p.id = u.id`,
			Params: []*plugin.Parameter{
				makeParam(1, "ids", "int4", true),
				makeParam(2, "names", "text", true),
			},
		},
		// Case 4: Non-bulk query — should be skipped
		{
			Name: "GetProductByID",
			Cmd:  ":one",
			Text: "SELECT id, name FROM products WHERE id = $1",
			Params: []*plugin.Parameter{
				makeParam(1, "", "int4", true),
			},
		},
	}
}

func buildTestCatalog() *plugin.Catalog {
	return &plugin.Catalog{
		DefaultSchema: "public",
		Schemas: []*plugin.Schema{
			{
				Name: "public",
				Tables: []*plugin.Table{
					{
						Rel: &plugin.Identifier{Name: "products"},
						Columns: []*plugin.Column{
							{Name: "id", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "int4"}},
							{Name: "name", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "text"}},
							{Name: "price", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "int4"}},
							{Name: "category", NotNull: false, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "text"}},
							{Name: "is_active", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "bool"}},
							{Name: "updated_at", NotNull: true, Type: &plugin.Identifier{Schema: "pg_catalog", Name: "timestamptz"}},
						},
					},
				},
			},
		},
	}
}

func makeParam(number int32, name, pgType string, notNull bool) *plugin.Parameter {
	return &plugin.Parameter{
		Number: number,
		Column: &plugin.Column{
			Name:    name,
			NotNull: notNull,
			IsArray: true,
			Type:    &plugin.Identifier{Schema: "pg_catalog", Name: pgType},
		},
	}
}
