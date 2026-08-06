package repository

import (
	"strings"
	"testing"
)

func TestMigrationSQLUpgradesExistingTable(t *testing.T) {
	src := strings.Join(migrationSQL(), "\n")
	for _, want := range []string{"pg_constraint", "DROP CONSTRAINT metrics_type_unit_range", "ADD CONSTRAINT metrics_type_unit_range", "profile metrics contain invalid type, unit, or value", "metrics_user_idempotency_unique", "metrics_user_recorded_at_idx"} {
		if !strings.Contains(src, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
}
