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
