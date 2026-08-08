package repository

import (
	"strings"
	"testing"
)

func TestMigrationSQLUsesExecutableRegclassAndEnforcesKeys(t *testing.T) {
	src := strings.Join(migrationSQL(DefaultSchema), "\n")
	for _, want := range []string{`conrelid='"profile_schema"."metrics"'::regclass`, "missing or invalid idempotency keys", "ALTER COLUMN idempotency_key SET NOT NULL", "metrics_idempotency_key_length", "char_length(idempotency_key) BETWEEN 1 AND 128", "CREATE UNIQUE INDEX"} {
		if !strings.Contains(src, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
	if strings.Contains(src, "conrelid=('\"") {
		t.Fatal("invalid quoted regclass SQL retained")
	}
}

func TestMigrationSQLSafelyQuotesRequestedSchemaEverywhere(t *testing.T) {
	schema := `profile-test"quoted`
	src := strings.Join(migrationSQL(schema), "\n")
	quotedSchema := `"profile-test""quoted"`
	qualifiedTable := quotedSchema + `."metrics"`
	regclass := `'"profile-test""quoted"."metrics"'::regclass`

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS " + qualifiedTable,
		"ALTER TABLE " + qualifiedTable,
		"conrelid=" + regclass,
		"DROP INDEX IF EXISTS " + quotedSchema + ".\"metrics_user_idempotency_unique\"",
		"ON " + qualifiedTable + "(user_id,idempotency_key)",
		"ON " + qualifiedTable + "(user_id,recorded_at DESC)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("migration for special schema missing %q\n%s", want, src)
		}
	}
	if strings.Contains(src, schema+".metrics") {
		t.Fatalf("migration contains unquoted schema name %q", schema)
	}
}
