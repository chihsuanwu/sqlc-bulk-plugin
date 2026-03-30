package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
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
