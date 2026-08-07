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
	Redis            redis.UniversalClient
	Handler          Handler
	Name             string
	Logger           *slog.Logger
	MaxRetries       int64
	Block            time.Duration
	BatchSize        int64
	OnConsumed       func()
	OnRetry          func()
	OnDLQ            func()
	OnLag            func(int64)
	ProcessTimeout   time.Duration
	BackoffBase      time.Duration
	BackoffMax       time.Duration
	Sleep            func(context.Context, time.Duration) error
	RandInt63n       func(int64) int64
	RunStep          func(context.Context) error
	SourceStream     string
	DeadLetterStream string
	GroupName        string
	DedupeTTL        time.Duration
}

func New(r redis.UniversalClient, h Handler, name string) *Consumer {
	return &Consumer{Redis: r, Handler: h, Name: name, MaxRetries: 5, Block: time.Second, BatchSize: 10, ProcessTimeout: 30 * time.Second, BackoffBase: 100 * time.Millisecond, BackoffMax: 5 * time.Second, RandInt63n: rand.Int63n, SourceStream: Stream, DeadLetterStream: DeadLetterStream, GroupName: Group, DedupeTTL: 7 * 24 * time.Hour}
}

func (c *Consumer) sourceStream() string {
	if c.SourceStream != "" {
		return c.SourceStream
	}
	return Stream
}
func (c *Consumer) deadLetterStream() string {
	if c.DeadLetterStream != "" {
		return c.DeadLetterStream
	}
	return DeadLetterStream
}
func (c *Consumer) groupName() string {
	if c.GroupName != "" {
		return c.GroupName
	}
	return Group
}
func (c *Consumer) dedupeTTL() time.Duration {
	if c.DedupeTTL <= 0 {
		return 7 * 24 * time.Hour
	}
	return c.DedupeTTL
}
func (c *Consumer) dedupeKey(id string) string {
	return "statistics:dead-letter:" + c.groupName() + ":" + c.sourceStream() + ":" + id
}
func (c *Consumer) EnsureGroup(ctx context.Context) error {
	err := c.Redis.XGroupCreateMkStream(ctx, c.sourceStream(), c.groupName(), "0").Err()
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
	base, max := c.BackoffBase, c.BackoffMax
	if base <= 0 {
		base = 100 * time.Millisecond
	}
	if max <= 0 {
		max = 5 * time.Second
	}
	if base > max {
		base = max
	}
	d := base
	for i := 0; i < attempt && d < max; i++ {
		if d > max/2 {
			d = max
			break
		}
		d *= 2
	}
	if c.RandInt63n != nil && d < max {
		room := max - d
		jitterBound := d / 2
		if jitterBound > room {
			jitterBound = room
		}
		if jitterBound > 0 {
			d += time.Duration(c.RandInt63n(int64(jitterBound) + 1))
		}
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
	streams, err := c.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: c.groupName(), Consumer: c.Name, Streams: []string{c.sourceStream(), ">"}, Count: c.batch(), Block: c.Block}).Result()
	if err != nil {
		return err
	}
	return c.processStreams(ctx, streams)
}

func (c *Consumer) ClaimOnce(ctx context.Context, idle time.Duration) error {
	start := "0-0"
	var first error
	for {
		messages, next, err := c.Redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: c.sourceStream(), Group: c.groupName(), Consumer: c.Name, MinIdle: idle, Start: start, Count: c.batch()}).Result()
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
		if ackErr := c.Redis.XAck(ctx, c.sourceStream(), c.groupName(), msg.ID).Err(); ackErr != nil {
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
		values["source_stream"] = c.sourceStream()
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
local state = redis.call('GET', KEYS[1])
if state == 'written' then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
  redis.call('XACK', KEYS[3], ARGV[#ARGV - 1], ARGV[#ARGV])
  return 'acked'
end
if not state then redis.call('SET', KEYS[1], 'pending', 'EX', ARGV[1]) end
local fields = {}
for i = 2, #ARGV - 2 do fields[i - 1] = ARGV[i] end
redis.call('XADD', KEYS[2], '*', unpack(fields))
redis.call('SET', KEYS[1], 'written', 'EX', ARGV[1])
redis.call('XACK', KEYS[3], ARGV[#ARGV - 1], ARGV[#ARGV])
return 'written'
`)

func (c *Consumer) deadLetterAndAck(ctx context.Context, id string, values map[string]any) error {
	args := []any{int64(c.dedupeTTL() / time.Second)}
	for key, value := range values {
		args = append(args, key, fmt.Sprint(value))
	}
	args = append(args, c.groupName(), id)
	_, err := deadLetterAndAckScript.Run(ctx, c.Redis, []string{c.dedupeKey(id), c.deadLetterStream(), c.sourceStream()}, args...).Result()
	return err
}
func (c *Consumer) reportLag(ctx context.Context) {
	if c.OnLag == nil {
		return
	}
	pending, err := c.Redis.XPending(ctx, c.sourceStream(), c.groupName()).Result()
	if err == nil {
		c.OnLag(pending.Count)
	}
}

func (c *Consumer) deliveryCount(ctx context.Context, id string) (int64, error) {
	items, err := c.Redis.XPendingExt(ctx, &redis.XPendingExtArgs{Stream: c.sourceStream(), Group: c.groupName(), Start: id, End: id, Count: 1}).Result()
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
