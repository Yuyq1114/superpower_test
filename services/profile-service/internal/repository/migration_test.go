package repository

import (
	"strings"
	"testing"
)

func TestMigrationSQLUsesExecutableRegclassAndEnforcesKeys(t *testing.T) {
	src := strings.Join(migrationSQL(), "\n")
	for _, want := range []string{"conrelid='profile_schema.metrics'::regclass", "missing or invalid idempotency keys", "ALTER COLUMN idempotency_key SET NOT NULL", "metrics_idempotency_key_length", "char_length(idempotency_key) BETWEEN 1 AND 128", "CREATE UNIQUE INDEX metrics_user_idempotency_unique"} {
		if !strings.Contains(src, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
	if strings.Contains(src, "conrelid=('\"") {
		t.Fatal("invalid quoted regclass SQL retained")
	}
}
