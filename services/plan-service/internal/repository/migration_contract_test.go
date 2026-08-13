package repository

import (
	"os"
	"strings"
	"testing"
)

func TestMigrationSourceDoesNotCreateSchema(t *testing.T) {
	for _, file := range []string{"repository.go", "weekly_migration.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToUpper(string(data)), "CREATE SCHEMA") {
			t.Fatalf("plan migration must not create schema: %s", file)
		}
	}
}
