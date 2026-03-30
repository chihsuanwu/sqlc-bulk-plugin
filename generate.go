package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

type bulkQuery struct {
	QueryName      string
	FuncName       string
	TableName      string
	ModelStruct    string
	ItemStruct     string
	UseModelStruct bool
	ParamsStruct   string
	Fields         []bulkField
	ReturnType     string // empty for :exec, e.g. "[]int32" for :many with single-column RETURNING
}

type bulkField struct {
	ParamNumber    int32
	ColumnName     string
	ParamsField    string
	ItemFieldName  string
	GoType         string
	ParamsElemType string
	NeedsConvert   bool
	ConvertExpr    string
}

const (
	styleFunction  = "function"
	styleMethod    = "method"
	styleInterface = "interface"
)

type pluginOptions struct {
	Package string `json:"package"`
	Style   string `json:"style"`
}

func generate(ctx context.Context, req *plugin.GenerateRequest) (*plugin.GenerateResponse, error) {
	// Parse plugin options
	opts := pluginOptions{Package: "db", Style: styleFunction}
	if len(req.PluginOptions) > 0 {
		if err := json.Unmarshal(req.PluginOptions, &opts); err != nil {
			return nil, fmt.Errorf("invalid plugin options: %w", err)
		}
	}
	switch opts.Style {
	case styleFunction, styleMethod, styleInterface:
		// valid
	case "":
		opts.Style = styleFunction
	default:
		return nil, fmt.Errorf("invalid style %q: must be %q, %q, or %q", opts.Style, styleFunction, styleMethod, styleInterface)
	}

	var queries []bulkQuery

	for _, q := range req.Queries {
		if !isBulk(q.Comments) {
			continue
		}

		var bq bulkQuery
		var err error

		if isInsertQuery(q) {
			bq, err = buildBulkInsertQuery(req.Catalog, q)
		} else {
			bq, err = buildBulkUpdateQuery(req.Catalog, q)
		}
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", q.Name, err)
		}
		queries = append(queries, bq)
	}

	if len(queries) == 0 {
		return &plugin.GenerateResponse{}, nil
	}

	content, err := renderTemplate(opts.Package, opts.Style, queries)
	if err != nil {
		return nil, err
	}

	return &plugin.GenerateResponse{
		Files: []*plugin.File{
			{
				Name:     "bulk.go",
				Contents: content,
			},
		},
	}, nil
}

func buildBulkUpdateQuery(catalog *plugin.Catalog, q *plugin.Query) (bulkQuery, error) {
	tableName, err := parseUpdateTable(q.Text)
	if err != nil {
		return bulkQuery{}, err
	}
	aliases, err := parseUNNESTAliases(q.Text)
	if err != nil {
		return bulkQuery{}, err
	}
	return buildBulkQueryFromAliases(catalog, q, tableName, aliases)
}

// isInsertQuery returns true if the query is an INSERT-based query (upsert or insert).
// Determined by whether sqlc provides InsertIntoTable.
func isInsertQuery(q *plugin.Query) bool {
	return q.InsertIntoTable != nil && q.InsertIntoTable.Name != ""
}

func buildBulkInsertQuery(catalog *plugin.Catalog, q *plugin.Query) (bulkQuery, error) {
	if q.InsertIntoTable == nil || q.InsertIntoTable.Name == "" {
		return bulkQuery{}, fmt.Errorf("InsertIntoTable not provided by sqlc for upsert query")
	}
	tableName := q.InsertIntoTable.Name
	aliases, err := parseUpsertAliases(q.Text)
	if err != nil {
		return bulkQuery{}, err
	}
	return buildBulkQueryFromAliases(catalog, q, tableName, aliases)
}

func buildBulkQueryFromAliases(catalog *plugin.Catalog, q *plugin.Query, tableName string, aliases map[int]string) (bulkQuery, error) {
	table, err := findTable(catalog, tableName)
	if err != nil {
		return bulkQuery{}, err
	}
	colMap := tableColumnMap(table)

	isNamedParam := len(q.Params) > 0 && q.Params[0].Column != nil && q.Params[0].Column.Name != ""

	fields := make([]bulkField, 0, len(q.Params))
	paramColumnNames := make([]string, 0, len(q.Params))

	for _, p := range q.Params {
		alias, ok := aliases[int(p.Number)]
		if !ok {
			return bulkQuery{}, fmt.Errorf("no column name found for parameter $%d", p.Number)
		}

		nullable := false
		if catalogCol, ok := colMap[alias]; ok {
			nullable = !catalogCol.NotNull
		}

		pgType := ""
		pgTypeSchema := ""
		if p.Column != nil && p.Column.Type != nil {
			pgType = p.Column.Type.Name
			pgTypeSchema = p.Column.Type.Schema
		}

		var goType, paramsElem string
		if isCustomType(pgTypeSchema) {
			goType = customGoType(pgType, nullable)
			paramsElem = pascalCase(pgType)
		} else {
			goType = pgTypeToGoType(pgType, nullable)
			paramsElem = pgTypeToParamsElemType(pgType)
		}
		itemField := pascalCase(alias)

		pf := paramsFieldName(p)
		if !isNamedParam {
			pf = fmt.Sprintf("Column%d", p.Number)
		}

		fieldAccess := "item." + itemField
		convertExpr := conversionExpr(fieldAccess, goType, paramsElem)

		fields = append(fields, bulkField{
			ParamNumber:    p.Number,
			ColumnName:     alias,
			ParamsField:    pf,
			ItemFieldName:  itemField,
			GoType:         goType,
			ParamsElemType: paramsElem,
			NeedsConvert:   goType != paramsElem,
			ConvertExpr:    convertExpr,
		})

		paramColumnNames = append(paramColumnNames, alias)
	}

	useModel := isFullColumnMatch(colMap, paramColumnNames)

	returnType, err := resolveReturnType(q)
	if err != nil {
		return bulkQuery{}, err
	}

	return bulkQuery{
		QueryName:      q.Name,
		FuncName:       q.Name + "Batch",
		TableName:      tableName,
		ModelStruct:    modelStructName(tableName),
		ItemStruct:     q.Name + "Item",
		UseModelStruct: useModel,
		ParamsStruct:   q.Name + "Params",
		Fields:         fields,
		ReturnType:     returnType,
	}, nil
}

// resolveReturnType determines the return type for the adapter function.
// Returns empty string for :exec, "[]GoType" for :many with single-column RETURNING.
func resolveReturnType(q *plugin.Query) (string, error) {
	if q.Cmd != ":many" {
		return "", nil
	}
	if len(q.Columns) == 0 {
		return "", fmt.Errorf(":many query has no RETURNING columns")
	}
	if len(q.Columns) > 1 {
		return "", fmt.Errorf(":many with multiple RETURNING columns not yet supported (got %d columns)", len(q.Columns))
	}
	col := q.Columns[0]
	if col.Type == nil {
		return "", fmt.Errorf(":many RETURNING column has no type information")
	}
	var goType string
	if isCustomType(col.Type.Schema) {
		goType = customGoType(col.Type.Name, !col.NotNull)
	} else {
		goType = pgTypeToGoType(col.Type.Name, !col.NotNull)
	}
	return "[]" + goType, nil
}

// needsPgtype returns true if any query uses pgtype.* types.
func needsPgtype(queries []bulkQuery) bool {
	for _, q := range queries {
		for _, f := range q.Fields {
			if strings.HasPrefix(f.GoType, "pgtype.") || strings.HasPrefix(f.ParamsElemType, "pgtype.") {
				return true
			}
		}
	}
	return false
}
