package repository

import (
	"context"
	"github.com/example/fitness-checkin/services/profile-service/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"testing"
	"time"
)

func TestPostgresCRUDIsolationIdempotencyAndSort(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e = Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	r := GORM{DB: db, Schema: DefaultSchema}
	u := "profile-test-" + time.Now().Format("150405.000000")
	defer db.Exec(`DELETE FROM "profile_schema"."metrics" WHERE user_id LIKE ?`, u+"%")
	t1 := time.Now().UTC().Add(-time.Hour)
	a := model.Metric{ID: u + "-a", UserID: u, MetricType: "weight", Value: 70, Unit: "kg", RecordedAt: t1, IdempotencyKey: "k", RequestFingerprint: "f", CreatedAt: time.Now().UTC()}
	if e = r.Create(ctx, &a); e != nil {
		t.Fatal(e)
	}
	again := model.Metric{ID: u + "-b", UserID: u, MetricType: "weight", Value: 70, Unit: "kg", RecordedAt: t1, IdempotencyKey: "k", RequestFingerprint: "f", CreatedAt: time.Now().UTC()}
	if e = r.Create(ctx, &again); e != nil || again.ID != a.ID {
		t.Fatal("idempotency", e)
	}
	b := model.Metric{ID: u + "-c", UserID: u, MetricType: "weight", Value: 71, Unit: "kg", RecordedAt: t1.Add(time.Minute), CreatedAt: time.Now().UTC()}
	if e = r.Create(ctx, &b); e != nil {
		t.Fatal(e)
	}
	other := b
	other.ID = u + "-other"
	other.UserID = u + "-2"
	if e = r.Create(ctx, &other); e != nil {
		t.Fatal(e)
	}
	got, e := r.List(ctx, u, "weight", t1.Add(-time.Minute), time.Now().UTC())
	if e != nil || len(got) != 2 || got[0].ID != b.ID {
		t.Fatalf("sort/isolation: %#v %v", got, e)
	}
}
