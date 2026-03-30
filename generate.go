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
		var bq bulkQuery
		var err error

		switch {
		case isBulkUpdate(q.Comments):
			bq, err = buildBulkUpdateQuery(req.Catalog, q)
		case isBulkUpsert(q.Comments):
			bq, err = buildBulkUpsertQuery(req.Catalog, q)
		default:
			continue
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

func buildBulkUpsertQuery(catalog *plugin.Catalog, q *plugin.Query) (bulkQuery, error) {
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

	return bulkQuery{
		QueryName:      q.Name,
		FuncName:       q.Name + "Batch",
		TableName:      tableName,
		ModelStruct:    modelStructName(tableName),
		ItemStruct:     q.Name + "Item",
		UseModelStruct: useModel,
		ParamsStruct:   q.Name + "Params",
		Fields:         fields,
	}, nil
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
