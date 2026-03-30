package main

import "fmt"

// pgTypeToGoType returns the Go type for use in Item/Model structs.
// When nullable, uses pgtype.* types.
func pgTypeToGoType(pgType string, nullable bool) string {
	if nullable {
		return pgTypeToNullableGoType(pgType)
	}
	return pgTypeToBaseGoType(pgType)
}

func pgTypeToBaseGoType(pgType string) string {
	switch pgType {
	case "int4", "serial":
		return "int32"
	case "int2", "smallserial", "smallint":
		return "int16"
	case "int8", "bigserial", "bigint":
		return "int64"
	case "text", "varchar", "character varying":
		return "string"
	case "bool", "boolean":
		return "bool"
	case "float4", "real":
		return "float32"
	case "float8", "double precision":
		return "float64"
	case "timestamptz", "timestamp with time zone":
		return "pgtype.Timestamptz"
	case "timestamp", "timestamp without time zone":
		return "pgtype.Timestamp"
	case "date":
		return "pgtype.Date"
	default:
		return "interface{}"
	}
}

func pgTypeToNullableGoType(pgType string) string {
	switch pgType {
	case "int4", "serial":
		return "pgtype.Int4"
	case "int2", "smallserial", "smallint":
		return "pgtype.Int2"
	case "int8", "bigserial", "bigint":
		return "pgtype.Int8"
	case "text", "varchar", "character varying":
		return "pgtype.Text"
	case "bool", "boolean":
		return "pgtype.Bool"
	case "float4", "real":
		return "pgtype.Float4"
	case "float8", "double precision":
		return "pgtype.Float8"
	case "timestamptz", "timestamp with time zone":
		return "pgtype.Timestamptz"
	case "timestamp", "timestamp without time zone":
		return "pgtype.Timestamp"
	case "date":
		return "pgtype.Date"
	default:
		return "interface{}"
	}
}

// pgTypeToParamsElemType returns the element type inside sqlc's Params slices.
// sqlc always uses base types for params, even for nullable columns.
func pgTypeToParamsElemType(pgType string) string {
	return pgTypeToBaseGoType(pgType)
}

// isCustomType returns true if the type schema indicates a non-built-in type (e.g. custom enum).
func isCustomType(schema string) bool {
	return schema != "" && schema != "pg_catalog"
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
