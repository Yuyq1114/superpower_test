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
	q := `"` + schema + `"`
	for _, x := range []string{`CREATE TABLE IF NOT EXISTS ` + q + `.plans (id text PRIMARY KEY,user_id text NOT NULL,name text NOT NULL,status text NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`, `CREATE TABLE IF NOT EXISTS ` + q + `.workout_days (id text PRIMARY KEY,user_id text NOT NULL,plan_id text NOT NULL,workout_date date NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`, `CREATE TABLE IF NOT EXISTS ` + q + `.workout_items (id text PRIMARY KEY,user_id text NOT NULL,workout_day_id text NOT NULL,name text NOT NULL,sets integer NOT NULL,repetitions integer NOT NULL,weight double precision NOT NULL,duration_seconds integer NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`} {
		if e := db.WithContext(ctx).Exec(x).Error; e != nil {
			return e
		}
	}
	return nil
}
