package storage

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
	"gorm.io/gorm"
)

func TestOpenPostgresUsesBoundedSinglePing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := OpenPostgres(ctx, config.Config{DBDSN: "postgres://invalid:invalid@192.0.2.1:5432/invalid?sslmode=disable"})
	if err == nil || !strings.Contains(err.Error(), "ping PostgreSQL") {
		t.Fatalf("expected bounded ping error, got %v", err)
	}
}

func TestInitPostgresSchemasRequiresDatabase(t *testing.T) {
	if err := InitPostgresSchemas(context.Background(), nil); err == nil {
		t.Fatal("expected nil database error")
	}
}

func TestInitPostgresSchemasRejectsEmptyRole(t *testing.T) {
	if err := InitPostgresSchemas(context.Background(), &gorm.DB{}, PostgresSchemaTarget{Schema: "auth_schema"}); err == nil || !strings.Contains(err.Error(), "role is required") {
		t.Fatalf("expected explicit role error, got %v", err)
	}
}

func TestInitPostgresSchemasRejectsUnsafeIdentifiers(t *testing.T) {
	if err := InitPostgresSchemas(context.Background(), &gorm.DB{}, PostgresSchemaTarget{Schema: "bad;drop", Role: "auth_service"}); err == nil {
		t.Fatal("expected unsafe schema identifier error")
	}
}

func TestPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set; skipping PostgreSQL integration test")
	}
	adminDB, err := OpenPostgres(context.Background(), config.Config{DBDSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, err := adminDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer adminSQL.Close()
	if err := InitPostgresSchemas(context.Background(), adminDB); err != nil {
		t.Fatalf("first schema initialization: %v", err)
	}
	if err := InitPostgresSchemas(context.Background(), adminDB); err != nil {
		t.Fatalf("second schema initialization: %v", err)
	}
	for _, target := range defaultPostgresTargets {
		var owner string
		if err := adminDB.WithContext(context.Background()).Raw("SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = ?", target.Schema).Scan(&owner).Error; err != nil {
			t.Fatalf("query owner for %s: %v", target.Schema, err)
		}
		if owner != target.Role {
			t.Errorf("schema %s owner = %q, want %q", target.Schema, owner, target.Role)
		}
	}
	checkRoleIsolation(t, dsn, "auth_service", "auth_schema", "plan_schema")
	checkRoleIsolation(t, dsn, "plan_service", "plan_schema", "auth_schema")
}

func checkRoleIsolation(t *testing.T, adminDSN, role, ownSchema, otherSchema string) {
	t.Helper()
	password := os.Getenv("TEST_" + strings.ToUpper(strings.TrimSuffix(role, "_service")) + "_SERVICE_PASSWORD")
	if password == "" {
		switch role {
		case "auth_service":
			password = "auth-local-only"
		case "plan_service":
			password = "plan-local-only"
		}
	}
	roleDSN := dsnForRole(adminDSN, role, password)
	db, err := OpenPostgres(context.Background(), config.Config{DBDSN: roleDSN})
	if err != nil {
		t.Fatalf("connect as %s: %v", role, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	table := fmt.Sprintf("storage_isolation_%d", time.Now().UnixNano())
	own := quotePostgresIdentifier(ownSchema)
	name := quotePostgresIdentifier(table)
	if err := db.Exec(fmt.Sprintf("CREATE TABLE %s.%s (id integer); CREATE SEQUENCE %s.%s_seq", own, name, own, name)).Error; err != nil {
		t.Fatalf("%s cannot create in own schema: %v", role, err)
	}
	defer db.Exec(fmt.Sprintf("DROP TABLE %s.%s; DROP SEQUENCE %s.%s_seq", own, name, own, name))
	if err := db.Exec(fmt.Sprintf("CREATE TABLE %s.%s (id integer)", quotePostgresIdentifier(otherSchema), name)).Error; err == nil {
		t.Fatalf("%s created table in other schema", role)
	}
}

func dsnForRole(raw, role, password string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.User = url.UserPassword(role, password)
	return u.String()
}
