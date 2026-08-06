package storage

import (
	"context"
	"testing"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
)

func TestOpenRedisReturnsConnectionError(t *testing.T) {
	cfg := config.Config{RedisAddr: "127.0.0.1:1"}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := OpenRedis(ctx, cfg); err == nil {
		t.Fatal("expected Redis connection error")
	}
}

func TestRedisStreamRoundTrip(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()
	stream := "test-workout-events"

	id, err := AddStreamMessage(ctx, client, stream, map[string]any{"event_id": "evt-1"})
	if err != nil {
		t.Fatalf("add stream message: %v", err)
	}
	if id == "" {
		t.Fatal("expected stream message ID")
	}
}
