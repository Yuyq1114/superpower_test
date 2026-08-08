package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
	"github.com/redis/go-redis/v9"
)

const redisOperationTimeout = 5 * time.Second

func OpenRedis(ctx context.Context, cfg config.Config) (*redis.Client, error) {
	if cfg.RedisAddr == "" {
		return nil, errors.New("Redis address is required")
	}
	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: 0})
	pingCtx, cancel := context.WithTimeout(ctx, redisOperationTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return client, nil
}

func AddStreamMessage(ctx context.Context, client redis.UniversalClient, stream string, values map[string]any) (string, error) {
	if client == nil {
		return "", errors.New("Redis client is required")
	}
	opCtx, cancel := context.WithTimeout(ctx, redisOperationTimeout)
	defer cancel()
	return client.XAdd(opCtx, &redis.XAddArgs{Stream: stream, ID: "*", Values: values}).Result()
}
