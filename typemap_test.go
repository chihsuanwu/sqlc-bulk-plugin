package main

import "testing"

func TestPgTypeToGoType(t *testing.T) {
	tests := []struct {
		pgType   string
		nullable bool
		want     string
	}{
		{"int4", false, "int32"},
		{"int4", true, "pgtype.Int4"},
		{"text", false, "string"},
		{"text", true, "pgtype.Text"},
		{"bool", false, "bool"},
		{"bool", true, "pgtype.Bool"},
		{"timestamptz", false, "pgtype.Timestamptz"},
		{"timestamptz", true, "pgtype.Timestamptz"},
		{"float8", false, "float64"},
		{"float8", true, "pgtype.Float8"},
		{"int2", false, "int16"},
		{"int8", false, "int64"},
		{"uuid", false, "pgtype.UUID"},
		{"uuid", true, "pgtype.UUID"},
		{"json", false, "[]byte"},
		{"json", true, "[]byte"},
		{"jsonb", false, "[]byte"},
		{"jsonb", true, "[]byte"},
		{"bytea", false, "[]byte"},
		{"bytea", true, "[]byte"},
		{"numeric", false, "pgtype.Numeric"},
		{"numeric", true, "pgtype.Numeric"},
		{"decimal", false, "pgtype.Numeric"},
		{"decimal", true, "pgtype.Numeric"},
		{"unknown_type", false, "pgtype.UnknownType"},
		{"unknown_type", true, "pgtype.UnknownType"},
		{"interval", false, "pgtype.Interval"},
		{"interval", true, "pgtype.Interval"},
		{"inet", false, "pgtype.Inet"},
		{"inet", true, "pgtype.Inet"},
	}

	for _, tt := range tests {
		name := tt.pgType
		if tt.nullable {
			name += "_nullable"
		}
		t.Run(name, func(t *testing.T) {
			got := pgTypeToGoType(tt.pgType, tt.nullable)
			if got != tt.want {
				t.Errorf("pgTypeToGoType(%q, %v) = %q, want %q", tt.pgType, tt.nullable, got, tt.want)
			}
		})
	}
}

func TestIsEnumType(t *testing.T) {
	enumSet := map[string]bool{"order_status": true, "my_enum": true}

	builtins := []string{"int4", "text", "bool", "uuid", "json", "jsonb", "bytea", "numeric", "decimal", "timestamptz", "interval", "inet"}
	for _, typ := range builtins {
		if isEnumType(typ, enumSet) {
			t.Errorf("isEnumType(%q) = true, want false", typ)
		}
	}
	enums := []string{"order_status", "my_enum"}
	for _, typ := range enums {
		if !isEnumType(typ, enumSet) {
			t.Errorf("isEnumType(%q) = false, want true", typ)
		}
	}
}

func TestCustomGoType(t *testing.T) {
	tests := []struct {
		pgType   string
		nullable bool
		want     string
	}{
		{"cancelled_source_enum", false, "CancelledSourceEnum"},
		{"cancelled_source_enum", true, "NullCancelledSourceEnum"},
		{"event_status", false, "EventStatus"},
		{"event_status", true, "NullEventStatus"},
	}

	for _, tt := range tests {
		name := tt.pgType
		if tt.nullable {
			name += "_nullable"
		}
		t.Run(name, func(t *testing.T) {
			got := customGoType(tt.pgType, tt.nullable)
			if got != tt.want {
				t.Errorf("customGoType(%q, %v) = %q, want %q", tt.pgType, tt.nullable, got, tt.want)
			}
		})
	}
}

func TestConversionExpr(t *testing.T) {
	tests := []struct {
		name     string
		access   string
		fromType string
		toType   string
		want     string
	}{
		{"same type", "item.ID", "int32", "int32", "item.ID"},
		{"pgtype.Text to string", "item.Category", "pgtype.Text", "string", "item.Category.String"},
		{"pgtype.Int4 to int32", "item.Count", "pgtype.Int4", "int32", "item.Count.Int32"},
		{"pgtype.Bool to bool", "item.Active", "pgtype.Bool", "bool", "item.Active.Bool"},
		{"pgtype.Float8 to float64", "item.Score", "pgtype.Float8", "float64", "item.Score.Float64"},
		{"nullable enum", "item.Source", "NullCancelledSourceEnum", "CancelledSourceEnum", "item.Source.CancelledSourceEnum"},
		{"nullable enum short", "item.Status", "NullEventStatus", "EventStatus", "item.Status.EventStatus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := conversionExpr(tt.access, tt.fromType, tt.toType)
			if got != tt.want {
				t.Errorf("conversionExpr(%q, %q, %q) = %q, want %q", tt.access, tt.fromType, tt.toType, got, tt.want)
			}
		})
	}
}
