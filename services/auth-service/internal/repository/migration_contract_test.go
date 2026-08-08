package repository

import (
	"strings"
	"testing"
)

func TestMigrationSQLDoesNotCreateSchema(t *testing.T) {
	for _, statement := range migrationSQL(DefaultSchema) {
		if strings.Contains(strings.ToUpper(statement), "CREATE SCHEMA") {
			t.Fatalf("migration must not create schema: %s", statement)
		}
	}
}
