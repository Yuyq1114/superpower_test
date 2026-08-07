package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
	"github.com/redis/go-redis/v9"
)

const (
	Stream           = "workout-events"
	Group            = "statistics-service"
	DeadLetterStream = "workout-events-dead-letter"
)

type Handler interface {
	ConsumeWorkoutCompleted(context.Context, model.WorkoutCompleted) error
}

type Consumer struct {
	Redis          redis.UniversalClient
	Handler        Handler
	Name           string
	Logger         *slog.Logger
	MaxRetries     int64
	Block          time.Duration
	BatchSize      int64
	OnConsumed     func()
	OnRetry        func()
	OnDLQ          func()
	OnLag          func(int64)
	ProcessTimeout time.Duration
	BackoffBase    time.Duration
	BackoffMax     time.Duration
	Sleep          func(context.Context, time.Duration) error
	RandInt63n     func(int64) int64
	RunStep        func(context.Context) error
}

func New(r redis.UniversalClient, h Handler, name string) *Consumer {
	return &Consumer{Redis: r, Handler: h, Name: name, MaxRetries: 5, Block: time.Second, BatchSize: 10, ProcessTimeout: 30 * time.Second, BackoffBase: 100 * time.Millisecond, BackoffMax: 5 * time.Second, RandInt63n: rand.Int63n}
}

func (c *Consumer) EnsureGroup(ctx context.Context) error {
	err := c.Redis.XGroupCreateMkStream(ctx, Stream, Group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (c *Consumer) Run(ctx context.Context) error {
	attempt := 0
	for ctx.Err() == nil {
		var err error
		if c.RunStep != nil {
			err = c.RunStep(ctx)
		} else {
			err = c.runStep(ctx)
		}
		if err == nil || errors.Is(err, redis.Nil) {
			attempt = 0
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.log("consumer iteration failed", err, "")
		if err = c.sleep(ctx, c.backoff(attempt)); err != nil {
			return err
		}
		attempt++
	}
	return ctx.Err()
}

func (c *Consumer) runStep(ctx context.Context) error {
	if err := c.EnsureGroup(ctx); err != nil {
		return err
	}
	if err := c.ClaimOnce(ctx, 30*time.Second); err != nil {
		return err
	}
	return c.ReadOnce(ctx)
}

func (c *Consumer) backoff(attempt int) time.Duration {
	d := c.BackoffBase
	if d <= 0 {
		d = 100 * time.Millisecond
	}
	max := c.BackoffMax
	if max <= 0 {
		max = 5 * time.Second
	}
	for i := 0; i < attempt && d < max; i++ {
		d *= 2
	}
	if d > max {
		d = max
	}
	if c.RandInt63n != nil {
		d += time.Duration(c.RandInt63n(int64(d/2) + 1))
	}
	return d
}
func (c *Consumer) sleep(ctx context.Context, d time.Duration) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func (c *Consumer) ReadOnce(ctx context.Context) error {
	streams, err := c.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: Group, Consumer: c.Name, Streams: []string{Stream, ">"}, Count: c.batch(), Block: c.Block}).Result()
	if err != nil {
		return err
	}
	return c.processStreams(ctx, streams)
}

func (c *Consumer) ClaimOnce(ctx context.Context, idle time.Duration) error {
	start := "0-0"
	var first error
	for {
		messages, next, err := c.Redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: Stream, Group: Group, Consumer: c.Name, MinIdle: idle, Start: start, Count: c.batch()}).Result()
		if err != nil {
			return err
		}
		for _, m := range messages {
			if err := c.process(ctx, m); err != nil && first == nil {
				first = err
			}
		}
		if next == "0-0" || next == start || len(messages) == 0 {
			return first
		}
		start = next
	}
}

func (c *Consumer) processStreams(ctx context.Context, streams []redis.XStream) error {
	var first error
	for _, stream := range streams {
		for _, m := range stream.Messages {
			if err := c.process(ctx, m); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}

func (c *Consumer) process(ctx context.Context, msg redis.XMessage) error {
	defer c.reportLag(ctx)
	event, parseErr := parse(msg.Values)
	var err error
	if parseErr != nil {
		err = parseErr
	} else {
		processCtx, cancel := context.WithTimeout(ctx, c.processTimeout())
		err = c.Handler.ConsumeWorkoutCompleted(processCtx, event)
		cancel()
	}
	if err == nil {
		if ackErr := c.Redis.XAck(ctx, Stream, Group, msg.ID).Err(); ackErr != nil {
			return ackErr
		}
		if c.OnConsumed != nil {
			c.OnConsumed()
		}
		return nil
	}
	deliveries, pendingErr := c.deliveryCount(ctx, msg.ID)
	if pendingErr != nil {
		return fmt.Errorf("process event: %w; pending metadata: %v", err, pendingErr)
	}
	if deliveries >= c.maxRetries() {
		values := cloneValues(msg.Values)
		values["source_stream"] = Stream
		values["source_message_id"] = msg.ID
		values["delivery_count"] = deliveries
		values["dead_letter_reason"] = err.Error()
		values["dead_lettered_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		if dlqErr := c.deadLetterAndAck(ctx, msg.ID, values); dlqErr != nil {
			return fmt.Errorf("process event: %w; DLQ/ACK: %v", err, dlqErr)
		}
		if c.OnDLQ != nil {
			c.OnDLQ()
		}
		c.log("event moved to dead-letter stream", err, value(msg.Values, "event_id"))
		return nil
	}
	if c.OnRetry != nil {
		c.OnRetry()
	}
	return err
}

func (c *Consumer) processTimeout() time.Duration {
	if c.ProcessTimeout <= 0 {
		return 30 * time.Second
	}
	return c.ProcessTimeout
}

var deadLetterAndAckScript = redis.NewScript(`
local added = redis.call('SETNX', KEYS[1], '1')
if added == 1 then
  redis.call('XADD', KEYS[2], '*', unpack(ARGV, 1, #ARGV - 2))
end
redis.call('XACK', KEYS[3], ARGV[#ARGV - 1], ARGV[#ARGV])
return added
`)

func (c *Consumer) deadLetterAndAck(ctx context.Context, id string, values map[string]any) error {
	args := make([]any, 0, len(values)*2+3)
	for key, value := range values {
		args = append(args, key, value)
	}
	args = append(args, Group, id)
	dedupeKey := "{statistics}:dead-letter:" + Stream + ":" + id
	_, err := deadLetterAndAckScript.Run(ctx, c.Redis, []string{dedupeKey, DeadLetterStream, Stream}, args...).Result()
	return err
}
func (c *Consumer) reportLag(ctx context.Context) {
	if c.OnLag == nil {
		return
	}
	pending, err := c.Redis.XPending(ctx, Stream, Group).Result()
	if err == nil {
		c.OnLag(pending.Count)
	}
}

func (c *Consumer) deliveryCount(ctx context.Context, id string) (int64, error) {
	items, err := c.Redis.XPendingExt(ctx, &redis.XPendingExtArgs{Stream: Stream, Group: Group, Start: id, End: id, Count: 1}).Result()
	if err != nil {
		return 0, err
	}
	if len(items) != 1 {
		return 0, errors.New("pending message not found")
	}
	return items[0].RetryCount, nil
}

func parse(values map[string]any) (model.WorkoutCompleted, error) {
	e := model.WorkoutCompleted{EventID: value(values, "event_id"), EventType: value(values, "event_type"), UserID: value(values, "user_id"), CheckinID: value(values, "checkin_id")}
	var err error
	if e.CompletedAt, err = time.Parse(time.RFC3339Nano, value(values, "completed_at")); err != nil {
		return model.WorkoutCompleted{}, errors.New("invalid completed_at")
	}
	if e.OccurredAt, err = time.Parse(time.RFC3339Nano, value(values, "occurred_at")); err != nil {
		return model.WorkoutCompleted{}, errors.New("invalid occurred_at")
	}
	if e.EventID == "" || e.EventType == "" || e.UserID == "" || e.CheckinID == "" {
		return model.WorkoutCompleted{}, errors.New("missing stable event fields")
	}
	return e, nil
}

func value(values map[string]any, key string) string {
	if v, ok := values[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}
func cloneValues(values map[string]any) map[string]any {
	out := make(map[string]any, len(values)+6)
	for k, v := range values {
		out[k] = v
	}
	return out
}
func (c *Consumer) batch() int64 {
	if c.BatchSize <= 0 {
		return 10
	}
	return c.BatchSize
}
func (c *Consumer) maxRetries() int64 {
	if c.MaxRetries <= 0 {
		return 5
	}
	return c.MaxRetries
}
func (c *Consumer) log(message string, err error, eventID string) {
	if c.Logger != nil {
		c.Logger.Warn(message, "event_id", eventID, "error", err)
	}
}
