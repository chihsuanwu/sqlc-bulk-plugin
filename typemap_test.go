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
		{"unknown_type", false, "interface{}"},
		{"unknown_type", true, "interface{}"},
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
