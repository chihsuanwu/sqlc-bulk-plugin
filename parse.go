package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Matches: UNNEST($1::int[]) AS id
	unnestAliasRe = regexp.MustCompile(`(?i)UNNEST\(\$(\d+)::\w+(?:\[\])?\)\s+AS\s+(\w+)`)

	// Matches: UPDATE products or UPDATE products AS p
	updateTableRe = regexp.MustCompile(`(?i)UPDATE\s+(\w+)`)

	// Matches: INSERT INTO tablename (col1, col2, ...)
	insertColumnsRe = regexp.MustCompile(`(?i)INSERT\s+INTO\s+\w+\s*\(([^)]+)\)`)

	// Matches: UNNEST($1::int[]) without requiring AS alias
	unnestParamRe = regexp.MustCompile(`(?i)UNNEST\(\$(\d+)::\w+(?:\[\])?\)`)
)

// parseUNNESTAliases extracts $N → column alias mappings from SQL text.
// Returns a map from parameter number (1-based) to the UNNEST alias name.
func parseUNNESTAliases(sql string) (map[int]string, error) {
	matches := unnestAliasRe.FindAllStringSubmatch(sql, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no UNNEST($N::type[]) AS alias patterns found in SQL")
	}
	result := make(map[int]string, len(matches))
	for _, m := range matches {
		num, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("invalid parameter number %q: %w", m[1], err)
		}
		result[num] = m[2]
	}
	return result, nil
}

// parseUpdateTable extracts the target table name from an UPDATE statement.
func parseUpdateTable(sql string) (string, error) {
	m := updateTableRe.FindStringSubmatch(sql)
	if m == nil {
		return "", fmt.Errorf("could not find UPDATE table name in SQL")
	}
	return m[1], nil
}

// parseInsertColumns extracts the column list from an INSERT INTO statement.
// Returns column names in declaration order.
func parseInsertColumns(sql string) ([]string, error) {
	m := insertColumnsRe.FindStringSubmatch(sql)
	if m == nil {
		return nil, fmt.Errorf("could not find INSERT INTO table (col1, col2, ...) pattern in SQL")
	}
	parts := strings.Split(m[1], ",")
	cols := make([]string, 0, len(parts))
	for _, p := range parts {
		col := strings.TrimSpace(p)
		if col != "" {
			cols = append(cols, col)
		}
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("empty column list in INSERT INTO")
	}
	return cols, nil
}

// parseUpsertAliases extracts $N → column name mappings for upsert queries.
// It pairs INSERT column list positions with UNNEST $N parameters in VALUES clause order.
func parseUpsertAliases(sql string) (map[int]string, error) {
	cols, err := parseInsertColumns(sql)
	if err != nil {
		return nil, err
	}

	// Extract UNNEST $N in order of appearance
	matches := unnestParamRe.FindAllStringSubmatch(sql, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no UNNEST($N::type[]) patterns found in SQL")
	}
	if len(matches) != len(cols) {
		return nil, fmt.Errorf("column count (%d) does not match UNNEST count (%d)", len(cols), len(matches))
	}

	result := make(map[int]string, len(cols))
	for i, m := range matches {
		num, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("invalid parameter number %q: %w", m[1], err)
		}
		result[num] = cols[i]
	}
	return result, nil
}

// isBulkUpdate checks if a query's comments contain the @bulk update annotation.
func isBulkUpdate(comments []string) bool {
	for _, c := range comments {
		if strings.Contains(c, "@bulk update") {
			return true
		}
	}
	return false
}

// isBulkUpsert checks if a query's comments contain the @bulk upsert annotation.
func isBulkUpsert(comments []string) bool {
	for _, c := range comments {
		if strings.Contains(c, "@bulk upsert") {
			return true
		}
	}
	return false
}
