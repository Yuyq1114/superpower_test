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

func TestDeadLetterAndAckIsIdempotentAcrossAmbiguousRetry(t *testing.T) {
	c, r, _ := setupConsumer(t, func(context.Context, model.WorkoutCompleted) error { return errors.New("boom") })
	id := addEvent(t, r, validValues())
	_ = c.ReadOnce(context.Background())
	values := cloneValues(validValues())
	values["source_stream"], values["source_message_id"] = Stream, id
	if err := c.deadLetterAndAck(context.Background(), id, values); err != nil {
		t.Fatal(err)
	}
	if err := c.deadLetterAndAck(context.Background(), id, values); err != nil {
		t.Fatal(err)
	}
	dlq, err := r.XRange(context.Background(), DeadLetterStream, "-", "+").Result()
	if err != nil || len(dlq) != 1 {
		t.Fatalf("dlq=%#v err=%v", dlq, err)
	}
	pending, _ := r.XPending(context.Background(), Stream, Group).Result()
	if pending.Count != 0 {
		t.Fatalf("pending=%d", pending.Count)
	}
}

func TestHandlerTimeoutLeavesPendingAndBatchContinues(t *testing.T) {
	calls := 0
	c, r, _ := setupConsumer(t, func(ctx context.Context, event model.WorkoutCompleted) error {
		calls++
		if event.EventID == "event-1" {
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	})
	c.ProcessTimeout = 10 * time.Millisecond
	c.BatchSize = 2
	addEvent(t, r, validValues())
	second := validValues()
	second["event_id"], second["checkin_id"] = "event-2", "checkin-2"
	addEvent(t, r, second)
	started := time.Now()
	err := c.ReadOnce(context.Background())
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("err=%v elapsed=%s", err, time.Since(started))
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
	pending, _ := r.XPending(context.Background(), Stream, Group).Result()
	if pending.Count != 1 {
		t.Fatalf("pending=%d", pending.Count)
	}
}

func TestRunBacksOffErrorsResetsAfterSuccessAndCancelsPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	steps := []error{errors.New("redis down"), errors.New("redis down"), nil, errors.New("redis down")}
	var sleeps []time.Duration
	c := &Consumer{BackoffBase: 10 * time.Millisecond, BackoffMax: 40 * time.Millisecond, RandInt63n: func(int64) int64 { return 0 }}
	c.RunStep = func(context.Context) error {
		if len(steps) == 0 {
			cancel()
			return context.Canceled
		}
		e := steps[0]
		steps = steps[1:]
		return e
	}
	c.Sleep = func(ctx context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	_ = c.Run(ctx)
	want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 10 * time.Millisecond}
	if len(sleeps) != len(want) {
		t.Fatalf("sleeps=%v", sleeps)
	}
	for i := range want {
		if sleeps[i] != want[i] {
			t.Fatalf("sleeps=%v", sleeps)
		}
	}
}

func TestDLQWriteFailureLeavesPendingAndRetryCompensates(t *testing.T) {
	c, r, _ := setupConsumer(t, func(context.Context, model.WorkoutCompleted) error { return errors.New("boom") })
	id := addEvent(t, r, validValues())
	_ = c.ReadOnce(context.Background())
	if err := r.Set(context.Background(), c.deadLetterStream(), "wrong-type", 0).Err(); err != nil {
		t.Fatal(err)
	}
	values := cloneValues(validValues())
	if err := c.deadLetterAndAck(context.Background(), id, values); err == nil {
		t.Fatal("expected XADD wrong-type error")
	}
	pending, _ := r.XPending(context.Background(), c.sourceStream(), c.groupName()).Result()
	if pending.Count != 1 {
		t.Fatalf("pending=%d", pending.Count)
	}
	state, _ := r.Get(context.Background(), c.dedupeKey(id)).Result()
	if state != "pending" {
		t.Fatalf("state=%q", state)
	}
	if err := r.Del(context.Background(), c.deadLetterStream()).Err(); err != nil {
		t.Fatal(err)
	}
	if err := c.deadLetterAndAck(context.Background(), id, values); err != nil {
		t.Fatal(err)
	}
	pending, _ = r.XPending(context.Background(), c.sourceStream(), c.groupName()).Result()
	if pending.Count != 0 {
		t.Fatalf("pending after retry=%d", pending.Count)
	}
	if got, _ := r.XLen(context.Background(), c.deadLetterStream()).Result(); got != 1 {
		t.Fatalf("dlq=%d", got)
	}
}
func TestConfiguredStreamsGroupAndDedupeTTL(t *testing.T) {
	m := miniredis.RunT(t)
	r := redis.NewClient(&redis.Options{Addr: m.Addr()})
	c := New(r, handlerFunc(func(context.Context, model.WorkoutCompleted) error { return errors.New("boom") }), "configured")
	c.SourceStream, c.DeadLetterStream, c.GroupName = "custom-events", "custom-dlq", "custom-group"
	c.DedupeTTL = 48 * time.Hour
	if err := c.EnsureGroup(context.Background()); err != nil {
		t.Fatal(err)
	}
	id, _ := r.XAdd(context.Background(), &redis.XAddArgs{Stream: c.SourceStream, Values: validValues()}).Result()
	_ = c.ReadOnce(context.Background())
	values := cloneValues(validValues())
	values["source_message_id"] = id
	if err := c.deadLetterAndAck(context.Background(), id, values); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.XLen(context.Background(), c.DeadLetterStream).Result(); got != 1 {
		t.Fatalf("dlq len=%d", got)
	}
	ttl := m.TTL(c.dedupeKey(id))
	if ttl != 48*time.Hour {
		t.Fatalf("ttl=%s", ttl)
	}
}

func TestPendingDLQStateRetriesWriteBeforeAck(t *testing.T) {
	c, r, _ := setupConsumer(t, func(context.Context, model.WorkoutCompleted) error { return errors.New("boom") })
	id := addEvent(t, r, validValues())
	_ = c.ReadOnce(context.Background())
	if err := r.Set(context.Background(), c.dedupeKey(id), "pending", c.dedupeTTL()).Err(); err != nil {
		t.Fatal(err)
	}
	values := cloneValues(validValues())
	values["source_message_id"] = id
	if err := c.deadLetterAndAck(context.Background(), id, values); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.XLen(context.Background(), c.deadLetterStream()).Result(); got != 1 {
		t.Fatalf("dlq=%d", got)
	}
	pending, _ := r.XPending(context.Background(), c.sourceStream(), c.groupName()).Result()
	if pending.Count != 0 {
		t.Fatalf("pending=%d", pending.Count)
	}
}

func TestBackoffClampsExtremeDurationsWithoutOverflow(t *testing.T) {
	cases := []Consumer{{BackoffBase: time.Duration(1 << 62), BackoffMax: time.Duration(1 << 62), RandInt63n: func(n int64) int64 { return n - 1 }}, {BackoffBase: -1, BackoffMax: -1}}
	for i := range cases {
		d := cases[i].backoff(1000)
		if d <= 0 || d > 5*time.Second && i == 1 || d < 0 {
			t.Fatalf("case %d duration=%s", i, d)
		}
	}
}

func TestValidateSettingsRejectsSubSecondTTL(t *testing.T) {
	c := New(nil, nil, "test")
	c.DedupeTTL = 500 * time.Millisecond
	if err := c.ValidateSettings(); err == nil {
		t.Fatal("expected sub-second TTL error")
	}
}

func TestValidateSettingsRejectsClusterMode(t *testing.T) {
	c := New(nil, nil, "test")
	c.RedisCluster = true
	if err := c.ValidateSettings(); err == nil {
		t.Fatal("expected cluster unsupported error")
	}
}

func TestValidateSettingsAcceptsCustomStreamsAndTTL(t *testing.T) {
	c := New(nil, nil, "test")
	c.SourceStream, c.DeadLetterStream, c.GroupName = "events", "dlq", "group"
	c.DedupeTTL = 2 * time.Second
	if err := c.ValidateSettings(); err != nil {
		t.Fatal(err)
	}
	if c.DedupeTTL != 2*time.Second {
		t.Fatalf("ttl=%s", c.DedupeTTL)
	}
}
