//go:build integration

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

func TestRealRedisReplaysSameEventID(t *testing.T) {
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
	stream, group := "workout-events-replay-test", "statistics-replay-test"
	if err := r.Del(ctx, stream).Err(); err != nil {
		t.Fatal(err)
	}
	defer r.Del(ctx, stream)
	seen := make(map[string]bool)
	calls := 0
	c := New(r, handlerFunc(func(_ context.Context, event model.WorkoutCompleted) error {
		calls++
		if seen[event.EventID] {
			return nil
		}
		seen[event.EventID] = true
		return nil
	}), "replay-consumer")
	c.SourceStream, c.GroupName, c.BatchSize, c.Block = stream, group, 2, 10*time.Millisecond
	if err := c.EnsureGroup(ctx); err != nil {
		t.Fatal(err)
	}
	values := validValues()
	if err := r.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: values}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := r.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: values}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := c.ReadOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(seen) != 1 {
		t.Fatalf("replay deliveries=%d unique event IDs=%d", calls, len(seen))
	}
}
