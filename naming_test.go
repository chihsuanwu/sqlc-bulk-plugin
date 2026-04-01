package main

import "testing"

func TestPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"id", "ID"},
		{"user_id", "UserID"},
		{"first_name", "FirstName"},
		{"url", "URL"},
		{"api_key", "APIKey"},
		{"is_active", "IsActive"},
		{"updated_at", "UpdatedAt"},
		{"a", "A"},
		{"", ""},
	}

	for _, tt := range tests {
		got := pascalCase(tt.input)
		if got != tt.want {
			t.Errorf("pascalCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSingularize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"products", "product"},
		{"categories", "category"},
		{"boxes", "box"},
		{"addresses", "address"},
		{"buses", "bus"},
		{"status", "status"},
		{"class", "class"},
		{"user", "user"},
		{"campus", "campus"},
		{"bonus", "bonus"},
		{"virus", "virus"},
		{"series", "series"},
		{"analyses", "analysis"},
		{"indices", "index"},
		{"statuses", "status"},
		{"gas", "gas"},
	}

	for _, tt := range tests {
		got := singularize(tt.input)
		if got != tt.want {
			t.Errorf("singularize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestModelStructName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"products", "Product"},
		{"tra_daily_timetables", "TraDailyTimetable"},
		{"users", "User"},
		{"categories", "Category"},
		{"order_items", "OrderItem"},
	}

	for _, tt := range tests {
		got := modelStructName(tt.input)
		if got != tt.want {
			t.Errorf("modelStructName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
