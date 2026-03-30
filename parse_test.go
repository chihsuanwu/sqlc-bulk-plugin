package main

import (
	"testing"
)

func TestParseUNNESTAliases(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		want    map[int]string
		wantErr bool
	}{
		{
			name: "basic positional params",
			sql: `SELECT
        UNNEST($1::int[]) AS id,
        UNNEST($2::text[]) AS name`,
			want: map[int]string{1: "id", 2: "name"},
		},
		{
			name: "mixed case UNNEST",
			sql: `SELECT
        unnest($1::int[]) AS id,
        Unnest($2::text[]) AS name`,
			want: map[int]string{1: "id", 2: "name"},
		},
		{
			name: "type without brackets",
			sql:  `UNNEST($1::int) AS id`,
			want: map[int]string{1: "id"},
		},
		{
			name: "extra whitespace",
			sql:  `UNNEST( $1::int[] )   AS   id`,
			// regex expects UNNEST($N::type[]) — no space inside parens
			wantErr: true,
		},
		{
			name:    "no UNNEST patterns",
			sql:     `UPDATE products SET name = $1 WHERE id = $2`,
			wantErr: true,
		},
		{
			name: "non-sequential param numbers",
			sql: `UNNEST($3::int[]) AS price,
        UNNEST($1::int[]) AS id,
        UNNEST($2::text[]) AS name`,
			want: map[int]string{1: "id", 2: "name", 3: "price"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUNNESTAliases(tt.sql)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseUNNESTAliases() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d aliases, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("alias[$%d] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestParseUpdateTable(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		want    string
		wantErr bool
	}{
		{
			name: "simple",
			sql:  `UPDATE products SET name = $1`,
			want: "products",
		},
		{
			name: "with alias",
			sql:  `UPDATE products AS p SET name = u.name`,
			want: "products",
		},
		{
			name: "lowercase",
			sql:  `update orders set status = $1`,
			want: "orders",
		},
		{
			name:    "no UPDATE keyword",
			sql:     `INSERT INTO products (id) VALUES ($1)`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUpdateTable(tt.sql)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseUpdateTable() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsBulkUpdate(t *testing.T) {
	tests := []struct {
		comments []string
		want     bool
	}{
		{[]string{"@bulk update"}, true},
		{[]string{"some comment", "@bulk update"}, true},
		{[]string{"@bulk upsert"}, false},
		{[]string{}, false},
		{nil, false},
	}

	for _, tt := range tests {
		got := isBulkUpdate(tt.comments)
		if got != tt.want {
			t.Errorf("isBulkUpdate(%v) = %v, want %v", tt.comments, got, tt.want)
		}
	}
}

func TestIsBulkUpsert(t *testing.T) {
	tests := []struct {
		comments []string
		want     bool
	}{
		{[]string{"@bulk upsert"}, true},
		{[]string{"@bulk update"}, false},
		{nil, false},
	}

	for _, tt := range tests {
		got := isBulkUpsert(tt.comments)
		if got != tt.want {
			t.Errorf("isBulkUpsert(%v) = %v, want %v", tt.comments, got, tt.want)
		}
	}
}
