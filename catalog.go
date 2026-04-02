package main

import (
	"fmt"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

// findTable locates a table by name in the catalog's default schema (public).
func findTable(catalog *plugin.Catalog, tableName string) (*plugin.Table, error) {
	schemaName := catalog.DefaultSchema
	if schemaName == "" {
		schemaName = "public"
	}
	for _, s := range catalog.Schemas {
		if s.Name != schemaName {
			continue
		}
		for _, t := range s.Tables {
			if t.Rel != nil && t.Rel.Name == tableName {
				return t, nil
			}
		}
	}
	return nil, fmt.Errorf("table %q not found in schema %q", tableName, schemaName)
}

// tableColumnMap returns a map from column name to column definition.
func tableColumnMap(table *plugin.Table) map[string]*plugin.Column {
	m := make(map[string]*plugin.Column, len(table.Columns))
	for _, c := range table.Columns {
		m[c.Name] = c
	}
	return m
}

// catalogEnumSet builds a set of enum type names from the catalog.
func catalogEnumSet(catalog *plugin.Catalog) map[string]bool {
	set := make(map[string]bool)
	for _, s := range catalog.Schemas {
		for _, e := range s.Enums {
			set[e.Name] = true
		}
	}
	return set
}

// isFullColumnMatch returns true if the param column names exactly match
// all columns in the table (ignoring order).
func isFullColumnMatch(tableColumns map[string]*plugin.Column, paramColumnNames []string) bool {
	if len(paramColumnNames) != len(tableColumns) {
		return false
	}
	for _, name := range paramColumnNames {
		if _, ok := tableColumns[name]; !ok {
			return false
		}
	}
	return true
}
