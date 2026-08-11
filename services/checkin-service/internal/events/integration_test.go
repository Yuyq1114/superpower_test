//go:build integration

package events

import (
	"context"
	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/storage"
	"os"
	"testing"
)

func TestRedisIntegrationSkipClearly(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR is not set; skipping Redis integration test")
	}
	// Mirrors the statistics consumer's integration harness: a real Redis is
	// normally password-protected (see deploy/docker-compose.yml), so without
	// the password this fails with NOAUTH instead of exercising anything.
	r, e := storage.OpenRedis(context.Background(), config.Config{RedisAddr: addr, RedisPassword: os.Getenv("TEST_REDIS_PASSWORD")})
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
}
