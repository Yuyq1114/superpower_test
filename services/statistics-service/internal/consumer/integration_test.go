package consumer

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
	"github.com/redis/go-redis/v9"
)

func TestRealRedisPendingRecovery(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set; skipping Redis integration test")
	}
	r := redis.NewClient(&redis.Options{Addr: addr, Password: os.Getenv("TEST_REDIS_PASSWORD")})
	ctx := context.Background()
	if err := r.Ping(ctx).Err(); err != nil {
		t.Fatalf("connect test Redis: %v", err)
	}
	defer r.Close()
	_ = r.Del(ctx, Stream, DeadLetterStream).Err()
	first := New(r, handlerFunc(func(context.Context, model.WorkoutCompleted) error { return context.DeadlineExceeded }), "failed-consumer")
	first.Block = 10 * time.Millisecond
	if err := first.EnsureGroup(ctx); err != nil {
		t.Fatal(err)
	}
	addEvent(t, r, validValues())
	_ = first.ReadOnce(ctx)
	called := 0
	second := New(r, handlerFunc(func(context.Context, model.WorkoutCompleted) error { called++; return nil }), "recovery-consumer")
	if err := second.ClaimOnce(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("recovered calls=%d", called)
	}
}
