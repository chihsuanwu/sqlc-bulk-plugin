package main

import (
	"fmt"
	"strings"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

var commonInitialisms = map[string]string{
	"id":   "ID",
	"url":  "URL",
	"api":  "API",
	"uri":  "URI",
	"uid":  "UID",
	"uuid": "UUID",
	"ip":   "IP",
	"http": "HTTP",
	"sql":  "SQL",
}

func pascalCase(snake string) string {
	parts := strings.Split(snake, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if upper, ok := commonInitialisms[strings.ToLower(p)]; ok {
			b.WriteString(upper)
		} else {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

func singularize(name string) string {
	if strings.HasSuffix(name, "ies") {
		return name[:len(name)-3] + "y"
	}
	if strings.HasSuffix(name, "ses") || strings.HasSuffix(name, "xes") {
		return name[:len(name)-2]
	}
	if strings.HasSuffix(name, "s") && !strings.HasSuffix(name, "ss") {
		return name[:len(name)-1]
	}
	return name
}

func modelStructName(tableName string) string {
	return pascalCase(singularize(tableName))
}

func paramsFieldName(param *plugin.Parameter) string {
	if param.Column != nil && param.Column.Name != "" {
		return pascalCase(param.Column.Name)
	}
	return fmt.Sprintf("Column%d", param.Number)
}
