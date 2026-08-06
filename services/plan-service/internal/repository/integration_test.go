package repository

import (
	"context"
	"github.com/example/fitness-checkin/services/plan-service/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"testing"
)

func TestIntegrationMigrateAndCreate(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set; skipping PostgreSQL integration test")
	}
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "plan_test_" + uuid.NewString()[:8]
	old := DefaultSchema
	_ = old
	if e = db.Exec("CREATE SCHEMA IF NOT EXISTS \"" + schema + "\"").Error; e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Exec("DROP SCHEMA IF EXISTS \"" + schema + "\" CASCADE") })
	if e = MigrateSchema(context.Background(), db, schema); e != nil {
		t.Fatal(e)
	}
	p := model.Plan{ID: uuid.NewString(), UserID: "u", Name: "test", Status: "draft"}
	if e = (GORM{DB: db, Schema: schema}).CreatePlan(context.Background(), &p); e != nil {
		t.Fatal(e)
	}
}
func MigrateSchema(ctx context.Context, db *gorm.DB, schema string) error {
	return migrateSchema(ctx, db, schema)
}
