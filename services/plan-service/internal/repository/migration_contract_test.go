package repository

import (
	"os"
	"strings"
	"testing"
)

func TestMigrationSourceDoesNotCreateSchema(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToUpper(string(data)), "CREATE SCHEMA") {
		t.Fatal("plan migration must not create schema")
	}
}
