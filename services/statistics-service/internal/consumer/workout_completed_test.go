package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
	"github.com/redis/go-redis/v9"
)

type handlerFunc func(context.Context, model.WorkoutCompleted) error

func (f handlerFunc) ConsumeWorkoutCompleted(ctx context.Context, e model.WorkoutCompleted) error {
	return f(ctx, e)
}

func setupConsumer(t *testing.T, h handlerFunc) (*Consumer, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	m := miniredis.RunT(t)
	r := redis.NewClient(&redis.Options{Addr: m.Addr()})
	c := New(r, h, "test-consumer")
	c.MaxRetries = 3
	if err := c.EnsureGroup(context.Background()); err != nil {
		t.Fatal(err)
	}
	return c, r, m
}

func addEvent(t *testing.T, r *redis.Client, values map[string]any) string {
	t.Helper()
	id, err := r.XAdd(context.Background(), &redis.XAddArgs{Stream: Stream, Values: values}).Result()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func validValues() map[string]any {
	return map[string]any{"event_id": "event-1", "event_type": model.WorkoutCompletedType, "user_id": "user-1", "checkin_id": "checkin-1", "completed_at": "2026-08-07T00:00:00Z", "occurred_at": "2026-08-07T00:00:01Z"}
}

func TestReadOnceHandlesAndAcknowledgesSuccess(t *testing.T) {
	called := 0
	c, r, _ := setupConsumer(t, func(context.Context, model.WorkoutCompleted) error { called++; return nil })
	var lag int64 = -1
	c.OnLag = func(value int64) { lag = value }
	addEvent(t, r, validValues())
	if err := c.ReadOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("called = %d", called)
	}
	pending, err := r.XPending(context.Background(), Stream, Group).Result()
	if err != nil || pending.Count != 0 {
		t.Fatalf("pending = %#v, err=%v", pending, err)
	}
	if lag != 0 {
		t.Fatalf("reported lag = %d", lag)
	}
}

func TestFailureRemainsPendingThenMovesToDLQ(t *testing.T) {
	c, r, m := setupConsumer(t, func(context.Context, model.WorkoutCompleted) error { return errors.New("boom") })
	var lag int64
	c.OnLag = func(value int64) { lag = value }
	id := addEvent(t, r, validValues())
	if err := c.ReadOnce(context.Background()); err == nil {
		t.Fatal("expected handler error")
	}
	pending, _ := r.XPending(context.Background(), Stream, Group).Result()
	if pending.Count != 1 {
		t.Fatalf("pending count = %d", pending.Count)
	}
	if lag != 1 {
		t.Fatalf("reported lag = %d", lag)
	}
	for i := 0; i < 2; i++ {
		m.FastForward(time.Second)
		if err := c.ClaimOnce(context.Background(), 0); err == nil && i == 0 {
			t.Fatal("expected retry error")
		}
	}
	pending, _ = r.XPending(context.Background(), Stream, Group).Result()
	if pending.Count != 0 {
		t.Fatalf("pending after DLQ = %d for %s", pending.Count, id)
	}
	if lag != 0 {
		t.Fatalf("reported lag after DLQ = %d", lag)
	}
	dlq, err := r.XRange(context.Background(), DeadLetterStream, "-", "+").Result()
	if err != nil || len(dlq) != 1 || dlq[0].Values["event_id"] != "event-1" {
		t.Fatalf("dlq = %#v, err=%v", dlq, err)
	}
}

func TestClaimOnceRecoversPendingFromAnotherConsumer(t *testing.T) {
	first, r, m := setupConsumer(t, func(context.Context, model.WorkoutCompleted) error { return errors.New("temporary") })
	addEvent(t, r, validValues())
	_ = first.ReadOnce(context.Background())
	m.FastForward(time.Second)
	called := 0
	second := New(r, handlerFunc(func(context.Context, model.WorkoutCompleted) error { called++; return nil }), "replacement")
	if err := second.ClaimOnce(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("recovered calls = %d", called)
	}
}

func TestMalformedPayloadRetriesAndDeadLetters(t *testing.T) {
	c, r, m := setupConsumer(t, func(context.Context, model.WorkoutCompleted) error { t.Fatal("handler must not run"); return nil })
	values := validValues()
	delete(values, "event_id")
	addEvent(t, r, values)
	_ = c.ReadOnce(context.Background())
	for i := 0; i < 2; i++ {
		m.FastForward(time.Second)
		_ = c.ClaimOnce(context.Background(), 0)
	}
	dlq, _ := r.XRange(context.Background(), DeadLetterStream, "-", "+").Result()
	if len(dlq) != 1 {
		t.Fatalf("dlq count = %d", len(dlq))
	}
}
