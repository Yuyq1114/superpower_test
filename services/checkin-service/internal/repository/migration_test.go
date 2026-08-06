package repository

import (
	"os"
	"strings"
	"testing"
)

func TestMigrationContainsFingerprintForCreateAndUpgrade(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "request_fingerprint text NOT NULL DEFAULT") {
		t.Fatal("fresh create must include request_fingerprint")
	}
	if !strings.Contains(text, "ADD COLUMN IF NOT EXISTS request_fingerprint") {
		t.Fatal("upgrade must add request_fingerprint")
	}
}
