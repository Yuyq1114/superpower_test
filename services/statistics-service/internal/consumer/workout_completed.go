package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	Redis      redis.UniversalClient
	Handler    Handler
	Name       string
	Logger     *slog.Logger
	MaxRetries int64
	Block      time.Duration
	BatchSize  int64
	OnConsumed func()
	OnRetry    func()
	OnDLQ      func()
	OnLag      func(int64)
}

func New(r redis.UniversalClient, h Handler, name string) *Consumer {
	return &Consumer{Redis: r, Handler: h, Name: name, MaxRetries: 5, Block: time.Second, BatchSize: 10}
}

func (c *Consumer) EnsureGroup(ctx context.Context) error {
	err := c.Redis.XGroupCreateMkStream(ctx, Stream, Group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (c *Consumer) Run(ctx context.Context) error {
	if err := c.EnsureGroup(ctx); err != nil {
		return err
	}
	if err := c.ClaimOnce(ctx, 30*time.Second); err != nil && ctx.Err() == nil {
		c.log("pending recovery failed", err, "")
	}
	for ctx.Err() == nil {
		if err := c.ReadOnce(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, redis.Nil) {
			c.log("event processing failed", err, "")
		}
		if err := c.ClaimOnce(ctx, 30*time.Second); err != nil && ctx.Err() == nil {
			c.log("pending retry failed", err, "")
		}
	}
	return nil
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
		err = c.Handler.ConsumeWorkoutCompleted(ctx, event)
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
		if dlqErr := c.Redis.XAdd(ctx, &redis.XAddArgs{Stream: DeadLetterStream, Values: values}).Err(); dlqErr != nil {
			return fmt.Errorf("process event: %w; DLQ: %v", err, dlqErr)
		}
		if ackErr := c.Redis.XAck(ctx, Stream, Group, msg.ID).Err(); ackErr != nil {
			return fmt.Errorf("DLQ written but ACK failed: %w", ackErr)
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
