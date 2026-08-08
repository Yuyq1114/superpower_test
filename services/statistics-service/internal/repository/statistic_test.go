package repository

import (
	"strings"
	"testing"
)

func TestTableQuotesIdentifiers(t *testing.T) {
	tests := []struct{ name, schema, table, want string }{
		{name: "default schema", table: "summaries", want: `"statistics_schema"."summaries"`},
		{name: "custom schema", schema: "tenant_one", table: "summaries", want: `"tenant_one"."summaries"`},
		{name: "escaped identifiers", schema: `tenant"one`, table: `summary"data`, want: `"tenant""one"."summary""data"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := table(tt.schema, tt.table); got != tt.want {
				t.Fatalf("table(%q, %q) = %q, want %q", tt.schema, tt.table, got, tt.want)
			}
		})
	}
}

func TestMigrationSQLUsesOnlyRequestedSchema(t *testing.T) {
	const schema = "statistics_test_0123456789abcdef0123456789abcdef"
	for _, query := range migrationSQL(schema, false) {
		if !strings.Contains(query, `"`+schema+`"`) {
			t.Fatalf("migration query does not reference requested schema: %s", query)
		}
		if strings.Contains(query, DefaultSchema) {
			t.Fatalf("migration query references fixed schema: %s", query)
		}
	}
}
