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

type pluginOptions struct {
	Package string `json:"package"`
}

func generate(ctx context.Context, req *plugin.GenerateRequest) (*plugin.GenerateResponse, error) {
	// Parse plugin options
	opts := pluginOptions{Package: "db"}
	if len(req.PluginOptions) > 0 {
		if err := json.Unmarshal(req.PluginOptions, &opts); err != nil {
			return nil, fmt.Errorf("invalid plugin options: %w", err)
		}
	}

	var queries []bulkQuery

	for _, q := range req.Queries {
		if isBulkUpsert(q.Comments) {
			// Phase 2 — skip with no error
			continue
		}
		if !isBulkUpdate(q.Comments) {
			continue
		}

		bq, err := buildBulkQuery(req.Catalog, q)
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", q.Name, err)
		}
		queries = append(queries, bq)
	}

	if len(queries) == 0 {
		return &plugin.GenerateResponse{}, nil
	}

	content, err := renderTemplate(opts.Package, queries)
	if err != nil {
		return nil, err
	}

	return &plugin.GenerateResponse{
		Files: []*plugin.File{
			{
				Name:     "bulk_update.go",
				Contents: content,
			},
		},
	}, nil
}

func buildBulkQuery(catalog *plugin.Catalog, q *plugin.Query) (bulkQuery, error) {
	// Parse target table
	tableName, err := parseUpdateTable(q.Text)
	if err != nil {
		return bulkQuery{}, err
	}

	// Parse UNNEST aliases (always needed for catalog lookup)
	aliases, err := parseUNNESTAliases(q.Text)
	if err != nil {
		return bulkQuery{}, err
	}

	// Look up table in catalog
	table, err := findTable(catalog, tableName)
	if err != nil {
		return bulkQuery{}, err
	}
	colMap := tableColumnMap(table)

	// Determine param naming style
	isNamedParam := len(q.Params) > 0 && q.Params[0].Column != nil && q.Params[0].Column.Name != ""

	// Build fields
	fields := make([]bulkField, 0, len(q.Params))
	paramColumnNames := make([]string, 0, len(q.Params))

	for _, p := range q.Params {
		alias, ok := aliases[int(p.Number)]
		if !ok {
			return bulkQuery{}, fmt.Errorf("no UNNEST alias found for parameter $%d", p.Number)
		}

		// Determine nullable from catalog
		nullable := false
		if catalogCol, ok := colMap[alias]; ok {
			nullable = !catalogCol.NotNull
		}

		pgType := ""
		if p.Column != nil && p.Column.Type != nil {
			pgType = p.Column.Type.Name
		}

		goType := pgTypeToGoType(pgType, nullable)
		paramsElem := pgTypeToParamsElemType(pgType)
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

	bq := bulkQuery{
		QueryName:      q.Name,
		FuncName:       q.Name + "Batch",
		TableName:      tableName,
		ModelStruct:    modelStructName(tableName),
		ItemStruct:     q.Name + "Item",
		UseModelStruct: useModel,
		ParamsStruct:   q.Name + "Params",
		Fields:         fields,
	}

	return bq, nil
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
