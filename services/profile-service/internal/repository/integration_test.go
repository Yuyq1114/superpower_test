package repository

import (
	"context"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/profile-service/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"strings"
	"testing"
	"time"
)

func integrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set; skipping real PostgreSQL profile migration tests")
	}
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	return db
}
func TestPostgresMigrationConstraintsUpgradeAndIdempotency(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	_ = db.Exec(`DROP SCHEMA IF EXISTS "profile_schema" CASCADE`).Error
	if e := db.Exec(`CREATE SCHEMA "profile_schema"; CREATE TABLE "profile_schema".metrics (id text PRIMARY KEY,user_id text NOT NULL,metric_type text NOT NULL,value double precision NOT NULL,unit text NOT NULL,recorded_at timestamptz NOT NULL,created_at timestamptz NOT NULL,idempotency_key text NOT NULL,request_fingerprint text NOT NULL DEFAULT '')`).Error; e != nil {
		t.Fatal(e)
	}
	if e := Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	bad := model.Metric{ID: "bad", UserID: "u", MetricType: "weight", Value: 0, Unit: "kg", RecordedAt: time.Now(), IdempotencyKey: "bad", RequestFingerprint: "bad", CreatedAt: time.Now()}
	if e := db.Table(table(DefaultSchema, "metrics")).Create(&bad).Error; e == nil {
		t.Fatal("CHECK constraint accepted invalid metric")
	}
	r := GORM{DB: db, Schema: DefaultSchema}
	a := model.Metric{ID: "a", UserID: "u", MetricType: "weight", Value: 70, Unit: "kg", RecordedAt: time.Now(), IdempotencyKey: "k", RequestFingerprint: "f", CreatedAt: time.Now()}
	if e := r.Create(ctx, &a); e != nil {
		t.Fatal(e)
	}
	same := a
	same.ID = "b"
	if e := r.Create(ctx, &same); e != nil || same.ID != "a" {
		t.Fatal("same fingerprint did not return original", e)
	}
	different := a
	different.ID = "c"
	different.RequestFingerprint = "other"
	if e := r.Create(ctx, &different); apperror.CodeOf(e) != apperror.CodeConflict {
		t.Fatal("expected idempotency conflict", e)
	}
}
func TestPostgresMigrationRejectsExistingDirtyData(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	_ = db.Exec(`DROP SCHEMA IF EXISTS "profile_schema" CASCADE`).Error
	if e := db.Exec(`CREATE SCHEMA "profile_schema"; CREATE TABLE "profile_schema".metrics (id text PRIMARY KEY,user_id text NOT NULL,metric_type text NOT NULL,value double precision NOT NULL,unit text NOT NULL,recorded_at timestamptz NOT NULL,created_at timestamptz NOT NULL,idempotency_key text,request_fingerprint text NOT NULL DEFAULT ''); INSERT INTO "profile_schema".metrics (id,user_id,metric_type,value,unit,recorded_at,created_at,idempotency_key) VALUES ('dirty','u','weight',70,'kg',now(),now(),'')`).Error; e != nil {
		t.Fatal(e)
	}
	e := Migrate(ctx, db)
	if e == nil || !strings.Contains(e.Error(), "profile metrics contain missing or invalid idempotency keys") {
		t.Fatalf("expected explicit dirty data failure: %v", e)
	}
}
