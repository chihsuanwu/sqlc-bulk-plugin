package main

import (
	"testing"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

func TestFindTable(t *testing.T) {
	catalog := &plugin.Catalog{
		DefaultSchema: "public",
		Schemas: []*plugin.Schema{
			{
				Name: "pg_catalog",
				Tables: []*plugin.Table{
					{Rel: &plugin.Identifier{Name: "pg_class"}},
				},
			},
			{
				Name: "public",
				Tables: []*plugin.Table{
					{Rel: &plugin.Identifier{Name: "users"}},
					{Rel: &plugin.Identifier{Name: "products"}},
				},
			},
		},
	}

	t.Run("found", func(t *testing.T) {
		tbl, err := findTable(catalog, "products")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tbl.Rel.Name != "products" {
			t.Errorf("got table %q, want %q", tbl.Rel.Name, "products")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := findTable(catalog, "orders")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("ignores non-default schema", func(t *testing.T) {
		_, err := findTable(catalog, "pg_class")
		if err == nil {
			t.Fatal("expected error for system table, got nil")
		}
	})
}

func TestIsFullColumnMatch(t *testing.T) {
	colMap := map[string]*plugin.Column{
		"id":   {Name: "id"},
		"name": {Name: "name"},
	}

	tests := []struct {
		name   string
		params []string
		want   bool
	}{
		{"exact match", []string{"id", "name"}, true},
		{"reversed order", []string{"name", "id"}, true},
		{"subset", []string{"id"}, false},
		{"superset", []string{"id", "name", "extra"}, false},
		{"wrong name", []string{"id", "label"}, false},
		{"empty", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFullColumnMatch(colMap, tt.params)
			if got != tt.want {
				t.Errorf("isFullColumnMatch(%v) = %v, want %v", tt.params, got, tt.want)
			}
		})
	}
}
