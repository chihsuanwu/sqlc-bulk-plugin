package main

import "fmt"

type pgTypeMapping struct {
	baseGoType     string
	nullableGoType string
}

// pgTypeMap maps PostgreSQL type names to their Go type representations.
// When baseGoType == nullableGoType, the type handles nullability internally (e.g. pgtype.*).
var pgTypeMap = map[string]pgTypeMapping{
	"int4":                        {"int32", "pgtype.Int4"},
	"serial":                      {"int32", "pgtype.Int4"},
	"int2":                        {"int16", "pgtype.Int2"},
	"smallserial":                 {"int16", "pgtype.Int2"},
	"smallint":                    {"int16", "pgtype.Int2"},
	"int8":                        {"int64", "pgtype.Int8"},
	"bigserial":                   {"int64", "pgtype.Int8"},
	"bigint":                      {"int64", "pgtype.Int8"},
	"text":                        {"string", "pgtype.Text"},
	"varchar":                     {"string", "pgtype.Text"},
	"character varying":           {"string", "pgtype.Text"},
	"bool":                        {"bool", "pgtype.Bool"},
	"boolean":                     {"bool", "pgtype.Bool"},
	"float4":                      {"float32", "pgtype.Float4"},
	"real":                        {"float32", "pgtype.Float4"},
	"float8":                      {"float64", "pgtype.Float8"},
	"double precision":            {"float64", "pgtype.Float8"},
	"timestamptz":                 {"pgtype.Timestamptz", "pgtype.Timestamptz"},
	"timestamp with time zone":    {"pgtype.Timestamptz", "pgtype.Timestamptz"},
	"timestamp":                   {"pgtype.Timestamp", "pgtype.Timestamp"},
	"timestamp without time zone": {"pgtype.Timestamp", "pgtype.Timestamp"},
	"date":                        {"pgtype.Date", "pgtype.Date"},
	"uuid":                        {"pgtype.UUID", "pgtype.UUID"},
	"json":                        {"[]byte", "[]byte"},
	"jsonb":                       {"[]byte", "[]byte"},
	"bytea":                       {"[]byte", "[]byte"},
	"numeric":                     {"pgtype.Numeric", "pgtype.Numeric"},
	"decimal":                     {"pgtype.Numeric", "pgtype.Numeric"},
}

// pgTypeToGoType returns the Go type for use in Item/Model structs.
// When nullable, uses pgtype.* types.
func pgTypeToGoType(pgType string, nullable bool) string {
	if nullable {
		return pgTypeToNullableGoType(pgType)
	}
	return pgTypeToBaseGoType(pgType)
}

func pgTypeToBaseGoType(pgType string) string {
	if m, ok := pgTypeMap[pgType]; ok {
		return m.baseGoType
	}
	return "interface{}"
}

func pgTypeToNullableGoType(pgType string) string {
	if m, ok := pgTypeMap[pgType]; ok {
		return m.nullableGoType
	}
	return "interface{}"
}

// pgTypeToParamsElemType returns the element type inside sqlc's Params slices.
// sqlc always uses base types for params, even for nullable columns.
func pgTypeToParamsElemType(pgType string) string {
	return pgTypeToBaseGoType(pgType)
}

// isCustomType returns true if the type is not a known built-in PostgreSQL type.
// sqlc may leave the schema empty for both built-in and custom types,
// so we check against the known type names instead.
func isCustomType(pgType string) bool {
	_, ok := pgTypeMap[pgType]
	return !ok
}

// customGoType returns the Go type for a custom type (e.g. enum).
// sqlc generates PascalCase type alias for enums, and Null+PascalCase for nullable.
func customGoType(pgType string, nullable bool) string {
	name := pascalCase(pgType)
	if nullable {
		return "Null" + name
	}
	return name
}

// resolveGoType returns the Go type string for a PG type, handling both
// built-in and custom types (enums).
func resolveGoType(pgType string, nullable bool) string {
	if isCustomType(pgType) {
		return customGoType(pgType, nullable)
	}
	return pgTypeToGoType(pgType, nullable)
}

// conversionExpr returns the expression to convert from Item struct field to Params element.
// fieldAccess is like "item.Category".
func conversionExpr(fieldAccess, fromType, toType string) string {
	if fromType == toType {
		return fieldAccess
	}
	accessor, ok := pgtypeAccessor[fromType]
	if ok {
		return fieldAccess + accessor
	}
	// Nullable custom type (e.g. NullCancelledSourceEnum → CancelledSourceEnum)
	if fromType == "Null"+toType {
		return fieldAccess + "." + toType
	}
	return fmt.Sprintf("/* TODO: convert %s -> %s */ %s", fromType, toType, fieldAccess)
}

var pgtypeAccessor = map[string]string{
	"pgtype.Text":   ".String",
	"pgtype.Int4":   ".Int32",
	"pgtype.Int2":   ".Int16",
	"pgtype.Int8":   ".Int64",
	"pgtype.Bool":   ".Bool",
	"pgtype.Float4": ".Float32",
	"pgtype.Float8": ".Float64",
}
