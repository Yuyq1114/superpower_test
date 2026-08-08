package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
	"github.com/redis/go-redis/v9"
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
	defer client.Close()
	id, err := AddStreamMessage(context.Background(), client, "test-workout-events", map[string]any{"event_id": "evt-1", "kind": "completed"})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := client.XRange(context.Background(), "test-workout-events", id, id).Result()
	if err != nil || len(entries) != 1 || entries[0].Values["event_id"] != "evt-1" {
		t.Fatalf("stream round trip failed: %v %#v", err, entries)
	}
}

func TestRedisIntegration(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR is not set; skipping Redis integration test")
	}
	client, err := OpenRedis(context.Background(), config.Config{RedisAddr: addr, RedisPassword: os.Getenv("TEST_REDIS_PASSWORD")})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	stream := "test-workout-events-integration"
	id, err := AddStreamMessage(context.Background(), client, stream, map[string]any{"event_id": "evt-integration", "kind": "completed"})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := client.XRange(context.Background(), stream, id, id).Result()
	if err != nil || len(entries) != 1 || entries[0].Values["kind"] != "completed" {
		t.Fatalf("XRANGE round trip failed: %v %#v", err, entries)
	}
	messages, err := client.XRead(context.Background(), &redis.XReadArgs{Streams: []string{stream, "0-0"}, Count: 1, Block: 0}).Result()
	if err != nil || len(messages) != 1 || len(messages[0].Messages) != 1 {
		t.Fatalf("XREAD round trip failed: %v %#v", err, messages)
	}
}
